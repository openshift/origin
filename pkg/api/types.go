package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
)

type ServiceList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []Service `json:"items" yaml:"items,omitempty"`
}

type Service struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Labels       map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// codec defines methods for serializing and deserializing API
// objects
type codec interface {
	Encode(obj interface{}) (data []byte, err error)
	Decode(data []byte) (interface{}, error)
	DecodeInto(data []byte, obj interface{}) error
}

// resourceVersioner provides methods for setting and retrieving
// the resource version from an API object
type resourceVersioner interface {
	SetResourceVersion(obj interface{}, version uint64) error
	ResourceVersion(obj interface{}) (uint64, error)
}

var Codec codec
var ResourceVersioner resourceVersioner

var scheme *conversion.Scheme

func init() {
	scheme = conversion.NewScheme()
	scheme.InternalVersion = ""
	scheme.ExternalVersion = "v1beta1"
	scheme.MetaInsertionFactory = metaInsertion{}
	scheme.AddKnownTypes("",
		ServiceList{},
		Service{},
		api.Status{},
		api.ServerOp{},
		api.ServerOpList{},
	)
	scheme.AddKnownTypes("v1beta1",
		ServiceList{},
		Service{},
		v1beta1.Status{},
		v1beta1.ServerOp{},
		v1beta1.ServerOpList{},
	)

	Codec = scheme
	ResourceVersioner = api.NewJSONBaseResourceVersioner()
}

// metaInsertion implements conversion.MetaInsertionFactory, which lets the conversion
// package figure out how to encode our object's types and versions. These fields are
// located in our JSONBase.
type metaInsertion struct {
	JSONBase struct {
		APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
		Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`
	} `json:",inline" yaml:",inline"`
}

// Create returns a new metaInsertion with the version and kind fields set.
func (metaInsertion) Create(version, kind string) interface{} {
	m := metaInsertion{}
	m.JSONBase.APIVersion = version
	m.JSONBase.Kind = kind
	return &m
}

// Interpret returns the version and kind information from in, which must be
// a metaInsertion pointer object.
func (metaInsertion) Interpret(in interface{}) (version, kind string) {
	m := in.(*metaInsertion)
	return m.JSONBase.APIVersion, m.JSONBase.Kind
}

// AddKnownTypes registers the types of the arguments to the marshaller of the package api.
// Encode() refuses the object unless its type is registered with AddKnownTypes.
func AddKnownTypes(version string, types ...interface{}) {
	scheme.AddKnownTypes(version, types...)
}
