package monitorserialization

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kube-openapi/pkg/util/sets"
)

func InstanceMapToFile(filename string, resourceType string, instances monitorapi.InstanceMap) error {
	namespaceToKeys := map[string][]monitorapi.InstanceKey{}
	for key, obj := range instances {
		ns := "---missing|metadata---"
		metadata, err := meta.Accessor(obj)
		if err == nil {
			ns = metadata.GetNamespace()
		}

		if _, ok := namespaceToKeys[ns]; !ok {
			namespaceToKeys[ns] = []monitorapi.InstanceKey{}
		}
		namespaceToKeys[ns] = append(namespaceToKeys[ns], key)
	}

	nsToItems := map[string]*unstructured.UnstructuredList{}
	for _, ns := range sets.StringKeySet(namespaceToKeys).List() {
		nsList := &unstructured.UnstructuredList{}
		for _, instanceKey := range namespaceToKeys[ns] {
			instanceMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(instances[instanceKey])
			if err != nil {
				return err
			}
			nsList.Items = append(
				nsList.Items,
				unstructured.Unstructured{
					Object: instanceMap,
				},
			)
		}
		nsToItems[ns] = nsList
	}

	byteBuffer := &bytes.Buffer{}
	zipWriter := zip.NewWriter(byteBuffer)

	for namespace, nsItems := range nsToItems {
		json, err := json.MarshalIndent(nsItems, "", "  ")
		if err != nil {
			return err
		}
		nsWriter, err := zipWriter.Create(filepath.Join(namespace, resourceType+".json"))
		if err != nil {
			return err
		}
		if _, err := nsWriter.Write(json); err != nil {
			return err
		}
	}

	if err := zipWriter.Flush(); err != nil {
		return err
	}
	if err := zipWriter.Close(); err != nil {
		return err
	}

	return ioutil.WriteFile(filename, byteBuffer.Bytes(), 0644)
}
