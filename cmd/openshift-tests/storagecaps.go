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
	Persistence         bool `yaml:"persistence"`
	Block               bool `yaml:"block"`
	FsGroup             bool `yaml:"fsGroup"`
	Exec                bool `yaml:"exec"`
	SnapshotDataSource  bool `yaml:"snapshotDataSource"`
	PVCDataSource       bool `yaml:"pvcDataSource"`
	MultiPODs           bool `yaml:"multipods"`
	RWX                 bool `yaml:"RWX"`
	ControllerExpansion bool `yaml:"controllerExpansion"`
	NodeExpansion       bool `yaml:"nodeExpansion"`
	VolumeLimits        bool `yaml:"volumeLimits"`
	SingleNodeVolume    bool `yaml:"singleNodeVolume"`
	Topology            bool `yaml:"topology"`
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

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Storage Capabilities (guaranteed only on full CSI test suite with 0 fails)")
	fmt.Fprintln(out, "==========================================================================")
	fmt.Fprintln(out, "Driver short name:                        ", yamlManifest.ShortName)
	fmt.Fprintln(out, "Driver name:                              ", yamlManifest.DriverInfo.Name)
	fmt.Fprintln(out, "Storage class:                            ", yamlManifest.StorageClass.FromFile)
	fmt.Fprintln(out, "Supported OpenShift / CSI features:")
	fmt.Fprintln(out, "  Persistent volumes:                     ", yamlManifest.DriverInfo.Capabilities.Persistence)
	fmt.Fprintln(out, "  Raw block mode:                         ", yamlManifest.DriverInfo.Capabilities.Block)
	fmt.Fprintln(out, "  FSGroup:                                ", yamlManifest.DriverInfo.Capabilities.FsGroup)
	fmt.Fprintln(out, "  Executable files on a volume:           ", yamlManifest.DriverInfo.Capabilities.Exec)
	fmt.Fprintln(out, "  Volume snapshots:                       ", yamlManifest.DriverInfo.Capabilities.SnapshotDataSource)
	fmt.Fprintln(out, "  Volume cloning:                         ", yamlManifest.DriverInfo.Capabilities.PVCDataSource)
	fmt.Fprintln(out, "  Use volume from multiple pods on a node:", yamlManifest.DriverInfo.Capabilities.MultiPODs)
	fmt.Fprintln(out, "  ReadWriteMany access mode:              ", yamlManifest.DriverInfo.Capabilities.RWX)
	fmt.Fprintln(out, "  Volume expansion for controller:        ", yamlManifest.DriverInfo.Capabilities.ControllerExpansion)
	fmt.Fprintln(out, "  Volume expansion for node:              ", yamlManifest.DriverInfo.Capabilities.NodeExpansion)
	fmt.Fprintln(out, "  Volume limits:                          ", yamlManifest.DriverInfo.Capabilities.VolumeLimits)
	fmt.Fprintln(out, "  Volume can run on single node:          ", yamlManifest.DriverInfo.Capabilities.SingleNodeVolume)
	fmt.Fprintln(out, "  Topology:                               ", yamlManifest.DriverInfo.Capabilities.Topology)
	fmt.Fprintln(out, "Supported OpenShift Virtualization features:")
	fmt.Fprintln(out, "  Raw block VM disks:                     ", yamlManifest.DriverInfo.Capabilities.Block)
	fmt.Fprintln(out, "  Live migration:                         ", yamlManifest.DriverInfo.Capabilities.RWX)
	fmt.Fprintln(out, "  VM snapshots:                           ", yamlManifest.DriverInfo.Capabilities.SnapshotDataSource)
	fmt.Fprintln(out, "  Storage-assisted cloning:               ", yamlManifest.DriverInfo.Capabilities.SnapshotDataSource)
	fmt.Fprintln(out)
}
