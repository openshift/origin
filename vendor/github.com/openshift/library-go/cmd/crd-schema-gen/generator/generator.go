package generator

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	crdgenerator "sigs.k8s.io/controller-tools/pkg/crd/generator"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	v1beta1.AddToScheme(scheme)
}

func Run() error {
	apisDir := flag.String("apis-dir", "pkg/apis", "the (relative) path to the package with API definitions")
	apis := flag.String("apis", "*", "the apis to generate from the apis-dir, in bash glob syntax")
	manifestDir := flag.String("manifests-dir", "manifests", "the directory with existing CRD manifests")
	outputDir := flag.String("output-dir", "", "optional directory to output the kubebuilder CRDs. By default a temporary directory is used.")
	verifyOnly := flag.Bool("verify-only", false, "do not write files, only compare and return with return code 1 if dirty")

	flag.Parse()

	// load existing manifests from manifests/ dir
	existing, err := crdsFromDirectory(*manifestDir)
	if err != nil {
		return err
	}

	// create temp dir
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	tmpDir, err := ioutil.TempDir(pwd, "")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %v\n", err)
	}
	defer os.RemoveAll(tmpDir)

	// copy APIs to temp dir
	fmt.Printf("Copying vendor/github.com/openshift/api/config to temporary pkg/apis...\n")
	if err := os.MkdirAll(filepath.Join(tmpDir, "pkg/apis"), 0755); err != nil {
		return err
	}
	cmd := fmt.Sprintf("cp -av \"%s/\"%s \"%s\"", *apisDir, *apis, filepath.Join(tmpDir, "pkg/apis"))
	out, err := exec.Command("/bin/bash", "-c", cmd).CombinedOutput()
	if err != nil {
		fmt.Print(string(out))
		return err
	}

	// generate kubebuilder NamedYaml manifests into temp dir
	g := crdgenerator.Generator{
		RootPath:          tmpDir,
		Domain:            "openshift.io",
		OutputDir:         filepath.Join(tmpDir, "manifests"),
		SkipMapValidation: true,
	}

	if len(*outputDir) != 0 {
		g.OutputDir = *outputDir
		fmt.Printf("Creating kubebuilder manifests %s...", *outputDir)
	} else {
		fmt.Printf("Creating kubebuilder manifests in ...\n")
	}

	if err := g.ValidateAndInitFields(); err != nil {
		return err
	}
	if err := g.Do(); err != nil {
		return err
	}

	// the generator changes the directory for some reason
	os.Chdir(pwd)

	// load kubebuilder manifests from temp dir
	fromKubebuilder, err := crdsFromDirectory(g.OutputDir)
	if err != nil {
		return err
	}

	existingFileNames := map[string]string{}
	for fn, crd := range existing {
		existingFileNames[crd.Name] = fn
	}

	// update existing manifests with validations of kubebuilder output
	dirty := false
	for fn, withValidation := range fromKubebuilder {
		existingFileName, ok := existingFileNames[withValidation.Name]
		if !ok {
			continue
		}

		crd := existing[existingFileName]
		// TODO: support multiple versions
		validation, _, err := nested(withValidation.Yaml, "spec", "validation")
		if err != nil {
			return fmt.Errorf("failed to access spec.validation in %s: %v", fn, err)
		}
		if validation == nil {
			continue
		}
		updated, err := set(crd.Yaml, validation, "spec", "validation")
		if err != nil {
			return fmt.Errorf("failed to set spec.validation in %s: %v", existingFileName, err)
		}
		if reflect.DeepEqual(updated, crd.Yaml) {
			fmt.Printf("Validation of %s in %s did not change\n", crd.Name, existingFileName)
			continue
		}

		bs, err := yaml.Marshal(updated)
		if err != nil {
			return err
		}

		// write updated file, either to old location, or to temp dir in verify mode
		newFn := existingFileName
		if *verifyOnly {
			newFn = filepath.Join(tmpDir, filepath.Base(existingFileName))
		} else {
			fmt.Printf("Updating validation of %s in %s\n", crd.Name, existingFileName)
		}
		if err := ioutil.WriteFile(newFn, bs, 0644); err != nil {
			return err
		}

		// compare old and new file
		if *verifyOnly {
			out, err := exec.Command("diff", "-u", existingFileName, newFn).CombinedOutput()
			if err != nil {
				fmt.Println(string(out))
				dirty = true
			}
		}
	}

	if *verifyOnly && dirty {
		return fmt.Errorf("verification failed")
	}

	return nil
}

func nested(x interface{}, pth ...string) (interface{}, bool, error) {
	if len(pth) == 0 {
		return x, true, nil
	}
	m, ok := x.(yaml.MapSlice)
	if !ok {
		return nil, false, fmt.Errorf("%s is not an object, but %T", strings.Join(pth, "."), x)
	}
	for _, item := range m {
		s, ok := item.Key.(string)
		if !ok {
			continue
		}
		if s == pth[0] {
			ret, found, err := nested(item.Value, pth[1:]...)
			if err != nil {
				return ret, found, fmt.Errorf("%s.%s", pth[0], err)
			}
			return ret, found, nil
		}
	}
	return nil, false, nil
}

func set(x interface{}, v interface{}, pth ...string) (interface{}, error) {
	if len(pth) == 0 {
		return v, nil
	}

	if x == nil {
		result, err := set(nil, v, pth[1:]...)
		if err != nil {
			return nil, fmt.Errorf("%s.%s", pth[0], err)
		}
		return yaml.MapSlice{yaml.MapItem{Key: pth[0], Value: result}}, nil
	}

	m, ok := x.(yaml.MapSlice)
	if !ok {
		return nil, fmt.Errorf("%s is not an object", strings.Join(pth, "."))
	}

	foundAt := -1
	for i, item := range m {
		s, ok := item.Key.(string)
		if !ok {
			continue
		}
		if s == pth[0] {
			foundAt = i
			break
		}
	}

	if foundAt < 0 {
		ret := make(yaml.MapSlice, len(m), len(m)+1)
		copy(ret, m)
		result, err := set(nil, v, pth[1:]...)
		if err != nil {
			return nil, fmt.Errorf("%s.%s", pth[0], err)
		}
		return append(ret, yaml.MapItem{Key: pth[0], Value: result}), nil
	}

	result, err := set(m[foundAt].Value, v, pth[1:]...)
	ret := make(yaml.MapSlice, len(m))
	copy(ret, m)
	if err != nil {
		return nil, fmt.Errorf("%s.%s", pth[0], err)
	}
	ret[foundAt].Value = result
	return ret, nil
}

type NamedYaml struct {
	Name string
	Yaml interface{}
}

// crdsFromDirectory returns CRDs by file path
func crdsFromDirectory(dir string) (map[string]NamedYaml, error) {
	ret := map[string]NamedYaml{}
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		if !strings.HasSuffix(info.Name(), ".yaml") {
			continue
		}
		bs, err := ioutil.ReadFile(filepath.Join(dir, info.Name()))
		if err != nil {
			return nil, err
		}

		obj, _, err := codecs.UniversalDeserializer().Decode(bs, nil, nil)
		if err != nil {
			continue
		}
		crd, ok := obj.(*v1beta1.CustomResourceDefinition)
		if !ok {
			continue
		}

		var y yaml.MapSlice
		if err := yaml.Unmarshal(bs, &y); err != nil {
			fmt.Printf("Warning: failed to unmarshal %q, skipping\n", info.Name())
			continue
		}
		ret[filepath.Join(dir, info.Name())] = NamedYaml{crd.Name, y}
	}
	if err != nil {
		return nil, err
	}
	return ret, err
}
