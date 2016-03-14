package start

import (

	// Admission control plug-ins used by OpenShift
	_ "github.com/openshift/origin/pkg/build/admission/defaults"
	_ "github.com/openshift/origin/pkg/build/admission/overrides"
	_ "github.com/openshift/origin/pkg/build/admission/strategyrestrictions"
	_ "github.com/openshift/origin/pkg/project/admission/lifecycle"
	_ "github.com/openshift/origin/pkg/project/admission/nodeenv"
	_ "github.com/openshift/origin/pkg/project/admission/requestlimit"
	_ "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	_ "github.com/openshift/origin/pkg/quota/admission/resourcequota"
	_ "github.com/openshift/origin/pkg/quota/admission/runonceduration"
	_ "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints"
	_ "github.com/openshift/origin/pkg/security/admission"
	_ "k8s.io/kubernetes/plugin/pkg/admission/admit"
	_ "k8s.io/kubernetes/plugin/pkg/admission/alwayspullimages"
	_ "k8s.io/kubernetes/plugin/pkg/admission/exec"
	_ "k8s.io/kubernetes/plugin/pkg/admission/limitranger"
	_ "k8s.io/kubernetes/plugin/pkg/admission/namespace/exists"
	_ "k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle"
	_ "k8s.io/kubernetes/plugin/pkg/admission/resourcequota"
	_ "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
)
