package v1

import "k8s.io/kubernetes/pkg/runtime"

func SetDefaults_RouteSpec(obj *RouteSpec) {
	if len(obj.To.Kind) == 0 {
		obj.To.Kind = "Service"
	}
}

func SetDefaults_TLSConfig(obj *TLSConfig) {
	if len(obj.Termination) == 0 && len(obj.DestinationCACertificate) == 0 {
		obj.Termination = TLSTerminationEdge
	}
	switch obj.Termination {
	case TLSTerminationType("Reencrypt"):
		obj.Termination = TLSTerminationReencrypt
	case TLSTerminationType("Edge"):
		obj.Termination = TLSTerminationEdge
	case TLSTerminationType("Passthrough"):
		obj.Termination = TLSTerminationPassthrough
	}
}

func addDefaultingFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		SetDefaults_RouteSpec,
		SetDefaults_TLSConfig,
	)
	if err != nil {
		panic(err)
	}
}
