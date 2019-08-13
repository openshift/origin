package cli

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericclioptions/openshiftpatch"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kcmdset "k8s.io/kubernetes/pkg/kubectl/cmd/set"
	describeversioned "k8s.io/kubernetes/pkg/kubectl/describe/versioned"
	"k8s.io/kubernetes/pkg/kubectl/generate/versioned"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	"github.com/openshift/library-go/pkg/legacyapi/legacygroupification"
	"github.com/openshift/oc/pkg/helpers/clientcmd"
	"github.com/openshift/oc/pkg/helpers/originpolymorphichelpers"
)

func shimKubectlForOc() {
	// we only need this change for `oc`.  `kubectl` should behave as close to `kubectl` as we can
	// if we call this factory construction method, we want the openshift style config loading
	kclientcmd.ErrEmptyConfig = genericclioptions.NewErrConfigurationMissing()
	kcmdset.ParseDockerImageReferenceToStringFunc = clientcmd.ParseDockerImageReferenceToStringFunc
	openshiftpatch.OAPIToGroupified = legacygroupification.OAPIToGroupified
	openshiftpatch.OAPIToGroupifiedGVK = legacygroupification.OAPIToGroupifiedGVK
	openshiftpatch.IsOAPIFn = legacygroupification.IsOAPI
	openshiftpatch.IsOC = true

	// update polymorphic helpers
	polymorphichelpers.AttachablePodForObjectFn = originpolymorphichelpers.NewAttachablePodForObjectFn(polymorphichelpers.AttachablePodForObjectFn)
	polymorphichelpers.CanBeExposedFn = originpolymorphichelpers.NewCanBeExposedFn(polymorphichelpers.CanBeExposedFn)
	polymorphichelpers.HistoryViewerFn = originpolymorphichelpers.NewHistoryViewerFn(polymorphichelpers.HistoryViewerFn)
	polymorphichelpers.LogsForObjectFn = originpolymorphichelpers.NewLogsForObjectFn(polymorphichelpers.LogsForObjectFn)
	polymorphichelpers.MapBasedSelectorForObjectFn = originpolymorphichelpers.NewMapBasedSelectorForObjectFn(polymorphichelpers.MapBasedSelectorForObjectFn)
	polymorphichelpers.ObjectPauserFn = originpolymorphichelpers.NewObjectPauserFn(polymorphichelpers.ObjectPauserFn)
	polymorphichelpers.ObjectResumerFn = originpolymorphichelpers.NewObjectResumerFn(polymorphichelpers.ObjectResumerFn)
	polymorphichelpers.PortsForObjectFn = originpolymorphichelpers.NewPortsForObjectFn(polymorphichelpers.PortsForObjectFn)
	polymorphichelpers.ProtocolsForObjectFn = originpolymorphichelpers.NewProtocolsForObjectFn(polymorphichelpers.ProtocolsForObjectFn)
	polymorphichelpers.RollbackerFn = originpolymorphichelpers.NewRollbackerFn(polymorphichelpers.RollbackerFn)
	polymorphichelpers.StatusViewerFn = originpolymorphichelpers.NewStatusViewerFn(polymorphichelpers.StatusViewerFn)
	polymorphichelpers.UpdatePodSpecForObjectFn = originpolymorphichelpers.NewUpdatePodSpecForObjectFn(polymorphichelpers.UpdatePodSpecForObjectFn)

	// update some functions we inject
	versioned.GeneratorFn = originpolymorphichelpers.NewGeneratorsFn(versioned.GeneratorFn)
	describeversioned.DescriberFn = originpolymorphichelpers.NewDescriberFn(describeversioned.DescriberFn)
}
