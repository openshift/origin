package fake

import (
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

type ImageStreamLimitVerifier struct {
	ImageStreamEvaluator func(ns string, is *imageapi.ImageStream) error
	Err                  error
}

func (f *ImageStreamLimitVerifier) VerifyLimits(ns string, is *imageapi.ImageStream) error {
	if f.ImageStreamEvaluator != nil {
		return f.ImageStreamEvaluator(ns, is)
	}
	return f.Err
}
