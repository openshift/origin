package options

import (
	"k8s.io/apiserver/pkg/admission"
	admissionmetrics "k8s.io/apiserver/pkg/admission/metrics"
)

var AdmissionDecorator admission.Decorator = admission.DecoratorFunc(admissionmetrics.WithControllerMetrics)
