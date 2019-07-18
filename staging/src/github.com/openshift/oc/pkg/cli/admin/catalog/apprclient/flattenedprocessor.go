package apprclient

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func NewFlattenedProcessor() *flattenedProcessor {
	return &flattenedProcessor{
		parser: &manifestYAMLParser{},
	}
}

type flattenedProcessor struct {
	parser ManifestYAMLParser
	count  int
}

func (w *flattenedProcessor) GetProcessedCount() int {
	return w.count
}

// Process handles a flattened single file operator manifest.
//
// It expects a single file, as soon as the function encounters a file it parses
// the raw yaml, separates it, converts it into a nested directory format,
// and writes those nested manifests to files.
// Process handles a flattened single file operator manifest.
//
// It expects a single file, as soon as the function encounters a file it parses
// the raw yaml, separates it, converts it into a nested directory format,
// and writes those nested manifests to files.
func (w *flattenedProcessor) Process(header *tar.Header, manifestName, workingDirectory string, reader io.Reader) (done bool, err error) {
	if header.Typeflag != tar.TypeReg {
		return
	}

	// We ran into the first file, We don't need to walk the tar ball any
	// further. Instruct the tar walker to quit.
	defer func() {
		done = true
	}()

	writer := &bytes.Buffer{}
	if _, err = io.Copy(writer, reader); err != nil {
		return
	}

	rawYAML := writer.Bytes()
	manifest, err := w.parser.Unmarshal(rawYAML)
	if err != nil {
		return
	}

	// now let's write each file to a directory
	packageName := manifest.Packages[0].PackageName

	manifestFolder := filepath.Join(workingDirectory, packageName)

	err = os.MkdirAll(manifestFolder, directoryPerm)
	if err != nil {
		return
	}

	// write csvs and crds for each csv version
	for _, csv := range manifest.ClusterServiceVersions {
		csvFileName := filepath.Join(manifestFolder, fmt.Sprintf("%s.clusterserviceversion.yaml", csv.GetName()))
		csvFile, err := w.parser.MarshalCSV(&csv)
		if err != nil {
			return done, err
		}

		err = writeYamlToFile(csvFileName, csvFile)
		if err != nil {
			return done, err
		}
	}

	// write crds
	for _, crd := range manifest.CustomResourceDefinitions {
		crdFileName := filepath.Join(manifestFolder, fmt.Sprintf("%s-%s.crd.yaml", crd.Spec.Names.Kind, crd.Spec.Version))
		crdFile, err := w.parser.MarshalCRD(&crd)
		if err != nil {
			return done, err
		}

		err = writeYamlToFile(crdFileName, crdFile)
		if err != nil {
			return done, err
		}
	}

	// write package file
	packageFileName := filepath.Join(manifestFolder, fmt.Sprintf("%s.package.yaml", packageName))
	packageFile, err := w.parser.MarshalPackage(&manifest.Packages[0])
	if err != nil {
		return
	}

	err = writeYamlToFile(packageFileName, packageFile)
	if err != nil {
		return
	}

	w.count++
	return
}

func writeYamlToFile(filepath, content string) error {
	fo, err := os.Create(filepath)
	if err != nil {
		panic(err)
	}

	defer fo.Close()

	_, err = fo.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}
