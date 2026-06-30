package csi

import (
	"sync"

	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

// InitCSITests may run more than once; register volume-expand patterns only once.
var registerVolumeExpandFsTypeTestsOnce sync.Once

// RegisterVolumeExpandFsTypeTests appends ext4 and xfs allowExpansion patterns to
// CSISuites for the upstream volume-expand test suite. Upstream DefineTests()
// handles offline and online resize; this only registers additional fstype combinations.
func RegisterVolumeExpandFsTypeTests() {
	registerVolumeExpandFsTypeTestsOnce.Do(func() {
		testsuites.CSISuites = append(testsuites.CSISuites, initVolumeExpandFsTypeSuite())
	})
}

func initVolumeExpandFsTypeSuite() func() storageframework.TestSuite {
	return func() storageframework.TestSuite {
		return testsuites.InitCustomVolumeExpandTestSuite([]storageframework.TestPattern{
			ext4AllowExpansionPattern(),
			xfsAllowExpansionPattern(),
		})
	}
}

func ext4AllowExpansionPattern() storageframework.TestPattern {
	p := storageframework.Ext4DynamicPV
	p.Name = "Dynamic PV (ext4)(allowExpansion)"
	p.AllowExpansion = true
	return p
}

func xfsAllowExpansionPattern() storageframework.TestPattern {
	p := storageframework.XfsDynamicPV
	p.Name = "Dynamic PV (xfs)(allowExpansion)"
	p.AllowExpansion = true
	return p
}
