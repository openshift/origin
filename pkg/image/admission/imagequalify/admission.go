package imagequalify

import (
	"fmt"
	"io"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
)

var _ admission.MutationInterface = &Plugin{}
var _ admission.ValidationInterface = &Plugin{}

// Plugin is an implementation of admission.Interface.
type Plugin struct {
	*admission.Handler

	rules []api.ImageQualifyRule
}

// Register creates and registers the new plugin but only if there is
// non-empty and a valid configuration.
func Register(plugins *admission.Plugins) {
	plugins.Register(api.PluginName, func(config io.Reader) (admission.Interface, error) {
		pluginConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		if pluginConfig == nil {
			glog.Infof("Admission plugin %q is not configured so it will be disabled.", api.PluginName)
			return nil, nil
		}
		return NewPlugin(pluginConfig.Rules), nil
	})
}

func isSubresourceRequest(attributes admission.Attributes) bool {
	return len(attributes.GetSubresource()) > 0
}

func isPodsRequest(attributes admission.Attributes) bool {
	return attributes.GetResource().GroupResource() == kapi.Resource("pods")
}

func shouldIgnore(attributes admission.Attributes) bool {
	switch {
	case isSubresourceRequest(attributes):
		return true
	case !isPodsRequest(attributes):
		return true
	default:
		return false
	}
}

func qualifyImages(images []string, rules []api.ImageQualifyRule) ([]string, error) {
	qnames := make([]string, len(images))

	for i := range images {
		qname, err := qualifyImage(images[i], rules)
		if err != nil {
			return nil, apierrs.NewBadRequest(fmt.Sprintf("invalid image %q: %s", images[i], err))
		}
		qnames[i] = qname
	}

	return qnames, nil
}

func containerImages(containers []kapi.Container) []string {
	names := make([]string, len(containers))

	for i := range containers {
		names[i] = containers[i].Image
	}

	return names
}

func qualifyContainers(containers []kapi.Container, rules []api.ImageQualifyRule, action func(index int, qname string) error) error {
	qnames, err := qualifyImages(containerImages(containers), rules)

	if err != nil {
		return err
	}

	for i := range containers {
		if err := action(i, qnames[i]); err != nil {
			return err
		}
	}

	return nil
}

// Admit makes an admission decision based on the request attributes.
// If the attributes are valid then any container image names that are
// unqualified (i.e., have no domain component) will be qualified with
// domain according to the set of rules. If no rule matches then the
// name can still remain unqualified.
func (p *Plugin) Admit(attributes admission.Attributes) error {
	// Ignore all calls to subresources or resources other than pods.
	if shouldIgnore(attributes) {
		return nil
	}

	pod, ok := attributes.GetObject().(*kapi.Pod)
	if !ok {
		return apierrs.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
	}

	if err := qualifyContainers(pod.Spec.InitContainers, p.rules, func(i int, qname string) error {
		if pod.Spec.InitContainers[i].Image != qname {
			glog.V(4).Infof("qualifying image %q as %q", pod.Spec.InitContainers[i].Image, qname)
			pod.Spec.InitContainers[i].Image = qname
		}
		return nil
	}); err != nil {
		return err
	}

	if err := qualifyContainers(pod.Spec.Containers, p.rules, func(i int, qname string) error {
		if pod.Spec.Containers[i].Image != qname {
			glog.V(4).Infof("qualifying image %q as %q", pod.Spec.Containers[i].Image, qname)
			pod.Spec.Containers[i].Image = qname
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// Validate makes an admission decision based on the request
// attributes. It checks that image names that got qualified in
// Admit() remain qualified, returning an error if this condition no
// longer holds true.
func (p *Plugin) Validate(attributes admission.Attributes) error {
	// Ignore all calls to subresources or resources other than pods.
	if shouldIgnore(attributes) {
		return nil
	}

	pod, ok := attributes.GetObject().(*kapi.Pod)
	if !ok {
		return apierrs.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
	}

	// Re-qualify - anything that has become unqualified has been
	// changed post Admit() and is now in error.

	if err := qualifyContainers(pod.Spec.InitContainers, p.rules, func(i int, qname string) error {
		if pod.Spec.InitContainers[i].Image != qname {
			msg := fmt.Sprintf("image %q should be qualified as %q", pod.Spec.InitContainers[i].Image, qname)
			return apierrs.NewBadRequest(msg)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := qualifyContainers(pod.Spec.Containers, p.rules, func(i int, qname string) error {
		if pod.Spec.Containers[i].Image != qname {
			msg := fmt.Sprintf("image %q should be qualified as %q", pod.Spec.Containers[i].Image, qname)
			return apierrs.NewBadRequest(msg)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// NewPlugin creates a new admission handler.
func NewPlugin(rules []api.ImageQualifyRule) *Plugin {
	return &Plugin{
		Handler: admission.NewHandler(admission.Create, admission.Update),
		rules:   rules,
	}
}
