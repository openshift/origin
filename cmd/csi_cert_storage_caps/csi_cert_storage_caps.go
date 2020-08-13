package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
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

// Test results xml structs

type TestSuite struct {
	XMLName   xml.Name   `xml:"testsuite"`
	Name      string     `xml:"name,attr"`
	Tests     uint       `xml:"tests,attr"`
	Failures  uint       `xml:"failures,attr"`
	Errors    uint       `xml:"errors,attr"`
	Time      float32    `xml:"time,attr"`
	TestCases []TestCase `xml:"testcase"`
}

type TestCase struct {
	XMLName   xml.Name `xml:"testcase"`
	Name      string   `xml:"name,attr"`
	ClassName string   `xml:"classname,attr"`
	Time      float32  `xml:"time,attr"`
	Passed    string   `xml:"passed"`
	Failure   string   `xml:"failure"`
	Skipped   Skip     `xml:"skipped"`
}

type Skip struct {
	Message string `xml:"message,attr"`
}

func parseManifest(manifestFilename string) {
	yamlFile, err := ioutil.ReadFile(manifestFilename)
	if err != nil {
		fmt.Println("Failed to", err)
		return
	}

	var yamlManifest YamlManifest
	err = yaml.Unmarshal(yamlFile, &yamlManifest)
	if err != nil {
		fmt.Println("Error parsing", err)
		return
	}
	fmt.Println("Driver short name:                 ", yamlManifest.ShortName)
	fmt.Println("Driver name:                       ", yamlManifest.DriverInfo.Name)
	fmt.Println("Storage class:                     ", yamlManifest.StorageClass.FromFile)
	fmt.Println("Raw block VM disks supported:      ", yamlManifest.DriverInfo.Capabilities.Block)
	fmt.Println("Live migration supported:          ", yamlManifest.DriverInfo.Capabilities.RWX)
	fmt.Println("Snapshotting tested:               ", yamlManifest.SnapshotClass.FromName)
	fmt.Println("VM snapshots supported:            ", yamlManifest.DriverInfo.Capabilities.Snapshot)
	fmt.Println("Storage-assisted cloning supported:", yamlManifest.DriverInfo.Capabilities.Snapshot)
}

func parseResults(resultsFilename string, listPassedTests bool, listFailedTests bool) bool {
	xmlFile, err := os.Open(resultsFilename)
	if err != nil {
		fmt.Println("Failed to", err)
		return false
	}

	byteValue, _ := ioutil.ReadAll(xmlFile)
	var testSuite TestSuite
	err = xml.Unmarshal(byteValue, &testSuite)
	if err != nil {
		fmt.Println("Failed to", err)
		return false
	}

	if testSuite.Failures == 0 {
		fmt.Println("CSI test results are all successful")
	} else {
		fmt.Println("NOTE: CSI test results indicate some failures, so reported driver capabilities might be inaccurate")
	}

	fmt.Println("Test Suite Name:", testSuite.Name, "| Tests:", testSuite.Tests, "| Failures:", testSuite.Failures,
		"| Errors:", testSuite.Errors, "| Time:", testSuite.Time)
	fmt.Println("---")

	if listPassedTests {
		countPassed := 0
		for i := 0; i < len(testSuite.TestCases); i++ {
			if len(testSuite.TestCases[i].Failure) == 0 && len(testSuite.TestCases[i].Skipped.Message) == 0 {
				fmt.Println("[Passed]", testSuite.TestCases[i].Name)
				countPassed++
			}
		}
		fmt.Println("Tests Passed:", countPassed)
		fmt.Println("---")
	}

	if testSuite.Failures > 0 && listFailedTests {
		countFailed := 0
		for i := 0; i < len(testSuite.TestCases); i++ {
			if len(testSuite.TestCases[i].Failure) > 0 {
				fmt.Println("[Failed]", testSuite.TestCases[i].Name)
				countFailed++
			}
		}
		fmt.Println("Tests Failed:", countFailed)
		fmt.Println("---")
	}

	xmlFile.Close()
	return true
}

func main() {
	manifestFilename := flag.String("m", "manifest.yaml", "Driver manifest.yaml")
	resultsFilename := flag.String("r", "", "CSI test results junit_e2e.xml")
	listPassedTests := flag.Bool("p", false, "Lists the passed CSI tests")
	listFailedTests := flag.Bool("f", false, "Lists the failed CSI tests")

	flag.Parse()

	if len(*resultsFilename) == 0 || len(*manifestFilename) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if parseResults(*resultsFilename, *listPassedTests, *listFailedTests) {
		parseManifest(*manifestFilename)
	}
}
