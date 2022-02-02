package aws

import (
	"strings"

	v1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/objx"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

func SkipUnlessPlatformAWS(infra objx.Map) {
	platform := infra.Get("spec.platformSpec.type")
	if !strings.EqualFold(platform.Str(""), string(v1.AWSPlatformType)) {
		e2eskipper.Skipf("unsupported platform")
	}
}
