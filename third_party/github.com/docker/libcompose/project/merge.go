package project

import (
	"bufio"
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

var (
	// ValidRemotes list the of valid prefixes that can be sent to Docker as a build remote location
	// This is public for consumers of libcompose to use
	ValidRemotes = []string{
		"git://",
		"git@github.com:",
		"github.com",
		"http:",
		"https:",
	}
	noMerge = []string{
		"links",
		"volumes_from",
	}
)

type rawSchema struct {
	Version  string        `yaml:"version"`
	Services rawServiceMap `yaml:"services"`
}

type rawService map[string]interface{}
type rawServiceMap map[string]rawService

func mergeProject(p *Project, file string, bytes []byte) (map[string]*ServiceConfig, error) {
	configs := make(map[string]*ServiceConfig)

	var schema rawSchema
	if err := yaml.Unmarshal(bytes, &schema); err != nil {
		return nil, err
	}

	var datas = make(rawServiceMap)
	switch {
	case schema.Version == "2":
		datas = schema.Services
	case len(schema.Version) == 0:
		datas = make(rawServiceMap)
		if err := yaml.Unmarshal(bytes, &datas); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("docker-compose file with schema version %q is not supported", schema.Version)
	}

	if err := interpolate(p.context.EnvironmentLookup, &datas); err != nil {
		return nil, err
	}

	for name, data := range datas {
		data, err := parse(p.context.ResourceLookup, p.context.EnvironmentLookup, file, data, datas)
		if err != nil {
			logrus.Errorf("Failed to parse service %s: %v", name, err)
			return nil, err
		}

		if _, ok := p.Configs[name]; ok {
			var rawExistingService rawService
			if err := Convert(p.Configs[name], &rawExistingService); err != nil {
				return nil, err
			}

			data = mergeConfig(rawExistingService, data)
		}

		datas[name] = data
	}

	if err := Convert(datas, &configs); err != nil {
		return nil, err
	}

	adjustValues(configs)

	return configs, nil
}

func adjustValues(configs map[string]*ServiceConfig) {
	// yaml parser turns "no" into "false" but that is not valid for a restart policy
	for _, v := range configs {
		if v.Restart == "false" {
			v.Restart = "no"
		}
	}
}

func readEnvFile(resourceLookup ResourceLookup, inFile string, serviceData rawService) (rawService, error) {
	var config ServiceConfig

	if err := Convert(serviceData, &config); err != nil {
		return nil, err
	}

	if len(config.EnvFile.Slice()) == 0 {
		return serviceData, nil
	}

	if resourceLookup == nil {
		return nil, fmt.Errorf("Can not use env_file in file %s no mechanism provided to load files", inFile)
	}

	vars := config.Environment.Slice()

	for i := len(config.EnvFile.Slice()) - 1; i >= 0; i-- {
		envFile := config.EnvFile.Slice()[i]
		content, _, err := resourceLookup.Lookup(envFile, inFile)
		if err != nil {
			return nil, err
		}

		if err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(bytes.NewBuffer(content))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			key := strings.SplitAfter(line, "=")[0]

			found := false
			for _, v := range vars {
				if strings.HasPrefix(v, key) {
					found = true
					break
				}
			}

			if !found {
				vars = append(vars, line)
			}
		}

		if scanner.Err() != nil {
			return nil, scanner.Err()
		}
	}

	serviceData["environment"] = vars

	delete(serviceData, "env_file")

	return serviceData, nil
}

func resolveBuild(inFile string, serviceData rawService) (rawService, error) {

	build := asString(serviceData["build"])
	if build == "" {
		return serviceData, nil
	}

	for _, remote := range ValidRemotes {
		if strings.HasPrefix(build, remote) {
			return serviceData, nil
		}
	}
	if filepath.IsAbs(build) {
		return serviceData, nil
	}

	current := path.Dir(inFile)

	if build == "." {
		build = current
	} else {
		current = path.Join(current, build)
	}

	serviceData["build"] = current

	return serviceData, nil
}

func parse(resourceLookup ResourceLookup, environmentLookup EnvironmentLookup, inFile string, serviceData rawService, datas rawServiceMap) (rawService, error) {
	serviceData, err := readEnvFile(resourceLookup, inFile, serviceData)
	if err != nil {
		return nil, err
	}

	serviceData, err = resolveBuild(inFile, serviceData)
	if err != nil {
		return nil, err
	}

	value, ok := serviceData["extends"]
	if !ok {
		return serviceData, nil
	}

	mapValue, ok := value.(map[interface{}]interface{})
	if !ok {
		return serviceData, nil
	}

	if resourceLookup == nil {
		return nil, fmt.Errorf("Can not use extends in file %s no mechanism provided to files", inFile)
	}

	file := asString(mapValue["file"])
	service := asString(mapValue["service"])

	if service == "" {
		return serviceData, nil
	}

	var baseService rawService

	if file == "" {
		if serviceData, ok := datas[service]; ok {
			baseService, err = parse(resourceLookup, environmentLookup, inFile, serviceData, datas)
		} else {
			return nil, fmt.Errorf("Failed to find service %s to extend", service)
		}
	} else {
		bytes, resolved, err := resourceLookup.Lookup(file, inFile)
		if err != nil {
			logrus.Errorf("Failed to lookup file %s: %v", file, err)
			return nil, err
		}

		var baseRawServices rawServiceMap
		if err := yaml.Unmarshal(bytes, &baseRawServices); err != nil {
			return nil, err
		}

		err = interpolate(environmentLookup, &baseRawServices)
		if err != nil {
			return nil, err
		}

		baseService, ok = baseRawServices[service]
		if !ok {
			return nil, fmt.Errorf("Failed to find service %s in file %s", service, file)
		}

		baseService, err = parse(resourceLookup, environmentLookup, resolved, baseService, baseRawServices)
	}

	if err != nil {
		return nil, err
	}

	baseService = clone(baseService)

	logrus.Debugf("Merging %#v, %#v", baseService, serviceData)

	for _, k := range noMerge {
		if _, ok := baseService[k]; ok {
			source := file
			if source == "" {
				source = inFile
			}
			return nil, fmt.Errorf("Cannot extend service '%s' in %s: services with '%s' cannot be extended", service, source, k)
		}
	}

	baseService = mergeConfig(baseService, serviceData)

	logrus.Debugf("Merged result %#v", baseService)

	return baseService, nil
}

func mergeConfig(baseService, serviceData rawService) rawService {
	for k, v := range serviceData {
		// Image and build are mutually exclusive in merge
		if k == "image" {
			delete(baseService, "build")
		} else if k == "build" {
			delete(baseService, "image")
		}
		existing, ok := baseService[k]
		if ok {
			baseService[k] = merge(existing, v)
		} else {
			baseService[k] = v
		}
	}

	return baseService
}

func merge(existing, value interface{}) interface{} {
	// append strings
	if left, lok := existing.([]interface{}); lok {
		if right, rok := value.([]interface{}); rok {
			return append(left, right...)
		}
	}

	//merge maps
	if left, lok := existing.(map[interface{}]interface{}); lok {
		if right, rok := value.(map[interface{}]interface{}); rok {
			newLeft := make(map[interface{}]interface{})
			for k, v := range left {
				newLeft[k] = v
			}
			for k, v := range right {
				newLeft[k] = v
			}
			return newLeft
		}
	}

	return value
}

func clone(in rawService) rawService {
	result := rawService{}
	for k, v := range in {
		result[k] = v
	}

	return result
}

func asString(obj interface{}) string {
	if v, ok := obj.(string); ok {
		return v
	}
	return ""
}
