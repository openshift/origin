package architecture

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type Architecture int

const (
	AMD64 Architecture = iota
	ARM64
	PPC64LE
	S390X
	MULTI
	UNKNOWN
)

const (
	NodeArchitectureLabel = "kubernetes.io/arch"
)

// SkipIfNoNodeWithArchitectures skip the test if the cluster is one of the given architectures
func SkipIfNoNodeWithArchitectures(oc *exutil.CLI, architectures ...Architecture) {
	if sets.New(
		GetAvailableArchitecturesSet(oc)...).IsSuperset(
		sets.New(architectures...)) {
		return
	}
	g.Skip(fmt.Sprintf("Skip for no nodes with requested architectures"))
}

// SkipArchitectures skip the test if the cluster is one of the given architectures
func SkipArchitectures(oc *exutil.CLI, architectures ...Architecture) (architecture Architecture) {
	architecture = ClusterArchitecture(oc)
	for _, arch := range architectures {
		if arch == architecture {
			g.Skip(fmt.Sprintf("Skip for cluster architecture: %s", arch.String()))
		}
	}
	return
}

// SkipNonAmd64SingleArch skip the test if the cluster is not an AMD64, single-arch, cluster
func SkipNonAmd64SingleArch(oc *exutil.CLI) (Architecture Architecture) {
	architecture := ClusterArchitecture(oc)
	if architecture != AMD64 {
		g.Skip(fmt.Sprintf("Skip for cluster architecture: %s", architecture.String()))
	}
	return
}

// GetAvailableArchitecturesSet returns multi-arch node cluster's Architectures
func GetAvailableArchitecturesSet(oc *exutil.CLI) []Architecture {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("nodes", "-o=jsonpath={.items[*].status.nodeInfo.architecture}").Output()
	if err != nil {
		e2e.Failf("unable to get the cluster architecture: ", err)
	}
	if output == "" {
		e2e.Failf("the retrieved architecture is empty")
	}
	architectureList := strings.Split(output, " ")
	archMap := make(map[Architecture]bool, 0)
	var architectures []Architecture
	for _, nodeArchitecture := range architectureList {
		if _, ok := archMap[FromString(nodeArchitecture)]; !ok {
			archMap[FromString(nodeArchitecture)] = true
			architectures = append(architectures, FromString(nodeArchitecture))
		}
	}
	return architectures
}

// SkipNonMultiArchCluster skip the test if the cluster is not an multi-arch cluster
func SkipNonMultiArchCluster(oc *exutil.CLI) {
	if !IsMultiArchCluster(oc) {
		g.Skip("This cluster is not multi-arch cluster, skip this case!")
	}
}

// IsMultiArchCluster check if the cluster is multi-arch cluster
func IsMultiArchCluster(oc *exutil.CLI) bool {
	architectures := GetAvailableArchitecturesSet(oc)
	return len(architectures) > 1
}

// FromString returns the Architecture value for the given string
func FromString(arch string) Architecture {
	switch arch {
	case "amd64":
		return AMD64
	case "arm64":
		return ARM64
	case "ppc64le":
		return PPC64LE
	case "s390x":
		return S390X
	case "multi":
		return MULTI
	default:
		e2e.Failf("Unknown architecture %s", arch)
	}
	return AMD64
}

// String returns the string value for the given Architecture
func (a Architecture) String() string {
	switch a {
	case AMD64:
		return "amd64"
	case ARM64:
		return "arm64"
	case PPC64LE:
		return "ppc64le"
	case S390X:
		return "s390x"
	case MULTI:
		return "multi"
	default:
		e2e.Failf("Unknown architecture %d", a)
	}
	return ""
}

// ClusterArchitecture returns the cluster's Architecture
// If the cluster uses the multi-arch payload, this function returns Architecture.multi
func ClusterArchitecture(oc *exutil.CLI) (architecture Architecture) {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("nodes", "-o=jsonpath={.items[*].status.nodeInfo.architecture}").Output()
	if err != nil {
		e2e.Failf("unable to get the cluster architecture: ", err)
	}
	if output == "" {
		e2e.Failf("the retrieved architecture is empty")
	}
	architectureList := strings.Split(output, " ")
	architecture = FromString(architectureList[0])
	for _, nodeArchitecture := range architectureList[1:] {
		if FromString(nodeArchitecture) != architecture {
			e2e.Logf("Found multi-arch node cluster")
			return MULTI
		}
	}
	return
}

func (a Architecture) GNUString() string {
	switch a {
	case AMD64:
		return "x86_64"
	case ARM64:
		return "aarch64"
	case PPC64LE:
		return "ppc64le"
	case S390X:
		return "s390x"
	case MULTI:
		return "multi"
	default:
		e2e.Failf("Unknown architecture %d", a)
	}
	return ""
}

// GetControlPlaneArch get the architecture of the contol plane node
func GetControlPlaneArch(oc *exutil.CLI) Architecture {
	masterNode, err := compat_otp.GetFirstMasterNode(oc)
	architecture, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", masterNode, "-o=jsonpath={.status.nodeInfo.architecture}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return FromString(architecture)
}
