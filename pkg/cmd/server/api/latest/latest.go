package latest

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// HACK TO ELIMINATE CYCLE UNTIL WE KILL THIS PACKAGE

// Version is the string that represents the current external default version.
var Version = unversioned.GroupVersion{Group: "", Version: "v1"}

// OldestVersion is the string that represents the oldest server version supported,
// for client code that wants to hardcode the lowest common denominator.
var OldestVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

// Versions is the list of versions that are recognized in code. The order provided
// may be assumed to be least feature rich to most feature rich, and clients may
// choose to prefer the latter items in the list over the former items when presented
// with a set of versions to choose.
var Versions = []unversioned.GroupVersion{unversioned.GroupVersion{Group: "", Version: "v1"}}
