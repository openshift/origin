package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

// Manifest yaml structs

type StorageClass struct {
	FromFile string `yaml:"FromFile"`
}

type SnapshotClass struct {
	FromName bool `yaml:"FromName"`
}

type Capabilities struct {
	Block    bool `yaml:"block"`
	RWX      bool `yaml:"RWX"`
	Snapshot bool `yaml:"snapshotDataSource"`
}

type DriverInfo struct {
	Name         string `yaml:"Name"`
	Capabilities `yaml:"Capabilities"`
}

type YamlManifest struct {
	ShortName     string `yaml:"ShortName"`
	StorageClass  `yaml:"StorageClass"`
	SnapshotClass `yaml:"SnapshotClass"`
	DriverInfo    `yaml:"DriverInfo"`
}

func printStorageCapabilities(out io.Writer) {
	manifestFilename := strings.Split(os.Getenv(manifestEnvVar), ",")[0]
	if manifestFilename == "" {
		fmt.Fprintln(out, "No manifest filename passed")
		return
	}

	yamlFile, err := ioutil.ReadFile(manifestFilename)
	if err != nil {
		fmt.Fprintln(out, "Failed to", err)
		return
	}

	var yamlManifest YamlManifest
	err = yaml.Unmarshal(yamlFile, &yamlManifest)
	if err != nil {
		fmt.Fprintln(out, "Error parsing", err)
		return
	}

	supportsSnapshot := yamlManifest.DriverInfo.Capabilities.Snapshot && yamlManifest.SnapshotClass.FromName

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Storage Capabilities (guaranteed only on full CSI test suite with 0 fails)")
	fmt.Fprintln(out, "==========================================================================")
	fmt.Fprintln(out, "Driver short name:                 ", yamlManifest.ShortName)
	fmt.Fprintln(out, "Driver name:                       ", yamlManifest.DriverInfo.Name)
	fmt.Fprintln(out, "Storage class:                     ", yamlManifest.StorageClass.FromFile)
	fmt.Fprintln(out, "Raw block VM disks supported:      ", yamlManifest.DriverInfo.Capabilities.Block)
	fmt.Fprintln(out, "Live migration supported:          ", yamlManifest.DriverInfo.Capabilities.RWX)
	fmt.Fprintln(out, "VM snapshots supported:            ", supportsSnapshot)
	fmt.Fprintln(out, "Storage-assisted cloning supported:", supportsSnapshot)
	fmt.Fprintln(out)
}
