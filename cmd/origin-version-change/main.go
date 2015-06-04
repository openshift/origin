// origin-version-change is a simple utility for converting a
// storage object into a different api version.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	kruntime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	configapi "github.com/openshift/origin/pkg/config/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

var (
	inputSource   = flag.StringP("input", "i", "-", "Input source; '-' means stdin")
	outputDest    = flag.StringP("output", "o", "-", "Output destination; '-' means stdout")
	rewrite       = flag.StringP("rewrite", "r", "", "If nonempty, use this as both input and output.")
	outputVersion = flag.StringP("out-version", "v", latest.Version, "Version to convert input to")
)

// isYAML determines whether data is JSON or YAML formatted by seeing
// if it will parse as json.
func isYAML(data []byte) bool {
	var unused interface{}
	if err := json.Unmarshal(data, &unused); err != nil {
		return true
	}
	return false
}

func changeObjectsVersion(items []kruntime.Object) {
	if errs := kruntime.DecodeList(items, api.Scheme); len(errs) > 0 {
		log.Fatalf("Unable to decode Template objects: %v", errs)
	}
	for i, obj := range items {
		_, kind, err := api.Scheme.ObjectVersionAndKind(obj)
		if err != nil {
			glog.Infof("Template.Objects[%d]: Unable to determine version and kind: %v", i, err)
			continue
		}
		mapping, err := latest.RESTMapper.RESTMapping(kind, *outputVersion)
		if err != nil {
			glog.Infof("Template.Objects[%d]: Unable to get REST mappings: %v", err)
			continue
		}
		info := resource.Info{Object: obj, Mapping: mapping}
		outputObj, err := resource.AsVersionedObject([]*resource.Info{&info}, false, *outputVersion)
		if err != nil {
			glog.Infof("Template.Objects[%d]: Unable to convert: %v", err)
			continue
		}
		items[i] = outputObj
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.CommandLine.SetNormalizeFunc(util.WordSepNormalizeFunc)
	flag.Parse()

	if *rewrite != "" {
		*inputSource = *rewrite
		*outputDest = *rewrite
	}

	var in io.Reader
	if *inputSource == "-" {
		in = os.Stdin
	} else {
		f, err := os.Open(*inputSource)
		if err != nil {
			log.Fatalf("Couldn't open %q: %q", *inputSource, err)
		}
		defer f.Close()
		in = f
	}

	data, err := ioutil.ReadAll(in)
	if err != nil {
		log.Fatalf("Couldn't read from input: %q", err)
	}
	isYAML := isYAML(data)

	if isYAML {
		data, err = yaml.YAMLToJSON(data)
		if err != nil {
			log.Fatalf("Failed to convert YAML to JSON: %q", err)
		}
	}
	obj, err := api.Scheme.Decode(data)
	if err != nil {
		log.Fatalf("Couldn't decode input: %q", err)
	}

	if template, ok := obj.(*templateapi.Template); ok {
		changeObjectsVersion(template.Objects)
	}
	if list, ok := obj.(*api.List); ok {
		changeObjectsVersion(list.Items)
	}
	if list, ok := obj.(*configapi.Config); ok {
		changeObjectsVersion(list.Items)
	}

	outData, err := api.Scheme.EncodeToVersion(obj, *outputVersion)
	if err != nil {
		log.Fatalf("Failed to encode to version %q: %q", *outputVersion, err)
	}

	if isYAML {
		outData, err = yaml.JSONToYAML(outData)
		if err != nil {
			log.Fatalf("Failed to convert to YAML: %q", err)
		}
	} else if true {
		// TODO: figure out if input JSON was pretty.
		var buf bytes.Buffer
		err = json.Indent(&buf, outData, "", "  ")
		if err != nil {
			log.Fatalf("Failed to indent JSON: %q", err)
		}
		outData = buf.Bytes()
	}

	var out io.Writer
	if *outputDest == "-" {
		out = os.Stdout
	} else {
		f, err := os.Create(*outputDest)
		if err != nil {
			log.Fatalf("Couldn't open %q: %q", *outputDest, err)
		}
		defer f.Close()
		out = f
	}

	if _, err = out.Write(outData); err != nil {
		log.Fatalf("Failed to write: %q", err)
	}
}
