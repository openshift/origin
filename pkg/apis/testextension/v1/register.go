package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName = "testextension.redhat.io"
	Version   = "v1"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
)
