package rest

import (
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"
)

type PassThroughStreamer struct {
	In          io.ReadCloser
	Flush       bool
	ContentType string
}

// a PipeStreamer must implement a rest.ResourceStreamer
var _ rest.ResourceStreamer = &PassThroughStreamer{}

func (obj *PassThroughStreamer) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (obj *PassThroughStreamer) DeepCopyObject() runtime.Object {
	panic("passThroughStreamer does not implement DeepCopyObject")
}

// InputStream returns a stream with the contents of the embedded pipe.
func (s *PassThroughStreamer) InputStream(apiVersion, acceptHeader string) (stream io.ReadCloser, flush bool, contentType string, err error) {
	return s.In, s.Flush, s.ContentType, nil
}
