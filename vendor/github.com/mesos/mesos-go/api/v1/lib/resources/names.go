package resources

import (
	"sort"

	"github.com/mesos/mesos-go/api/v1/lib"
)

type (
	Name  string
	Names []Name
)

const (
	NameCPUs  = Name("cpus")
	NameDisk  = Name("disk")
	NameGPUs  = Name("gpus")
	NameMem   = Name("mem")
	NamePorts = Name("ports")
)

// String implements fmt.Stringer
func (n Name) String() string                { return string(n) }
func (n Name) Filter(r *mesos.Resource) bool { return r != nil && r.Name == string(n) }

func (ns Names) Len() int           { return len(ns) }
func (ns Names) Less(i, j int) bool { return ns[i] < ns[j] }
func (ns Names) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }

func (ns Names) Sort() { sort.Stable(ns) }
