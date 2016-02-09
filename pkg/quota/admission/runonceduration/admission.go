package runonceduration

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strconv"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/admission/runonceduration/api"
	"github.com/openshift/origin/pkg/quota/admission/runonceduration/api/validation"
)

func init() {
	admission.RegisterPlugin("RunOnceDuration", func(client kclient.Interface, config io.Reader) (admission.Interface, error) {
		pluginConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return NewRunOnceDuration(pluginConfig), nil
	})
}

func readConfig(reader io.Reader) (*api.RunOnceDurationConfig, error) {
	config := &api.RunOnceDurationConfig{}
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return config, nil
	}
	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	err = configlatest.ReadYAML(configBytes, config)
	if err != nil {
		return nil, err
	}
	errs := validation.ValidateRunOnceDurationConfig(config)
	if len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	return config, nil
}

func NewRunOnceDuration(config *api.RunOnceDurationConfig) admission.Interface {
	return &runOnceDuration{
		Handler: admission.NewHandler(admission.Create, admission.Update),
		config:  config,
	}
}

type runOnceDuration struct {
	*admission.Handler
	config *api.RunOnceDurationConfig
	cache  *projectcache.ProjectCache
}

var _ = oadmission.WantsProjectCache(&runOnceDuration{})
var _ = oadmission.Validator(&runOnceDuration{})

func (a *runOnceDuration) Admit(attributes admission.Attributes) error {
	switch {
	case a.config == nil,
		!a.config.Enabled,
		attributes.GetResource() != kapi.Resource("pods"),
		len(attributes.GetSubresource()) > 0:
		return nil
	}
	pod, ok := attributes.GetObject().(*kapi.Pod)
	if !ok {
		return admission.NewForbidden(attributes, fmt.Errorf("unexpected object: %#v", attributes.GetObject()))
	}

	// Only update pods with a restart policy of Never or OnFailure
	switch pod.Spec.RestartPolicy {
	case kapi.RestartPolicyNever,
		kapi.RestartPolicyOnFailure:
		// continue
	default:
		return nil
	}

	appliedProjectOverride, err := a.applyProjectAnnotationOverride(attributes.GetNamespace(), pod)
	if err != nil {
		return admission.NewForbidden(attributes, err)
	}

	if !appliedProjectOverride && a.config.ActiveDeadlineSecondsOverride != nil {
		pod.Spec.ActiveDeadlineSeconds = a.config.ActiveDeadlineSecondsOverride
	}
	return nil
}

func (a *runOnceDuration) SetProjectCache(cache *projectcache.ProjectCache) {
	a.cache = cache
}

func (a *runOnceDuration) Validate() error {
	if a.cache == nil {
		return errors.New("RunOnceDuration plugin requires a project cache")
	}
	return nil
}

func (a *runOnceDuration) applyProjectAnnotationOverride(namespace string, pod *kapi.Pod) (bool, error) {
	ns, err := a.cache.GetNamespace(namespace)
	if err != nil {
		return false, fmt.Errorf("error looking up pod namespace: %v", err)
	}
	if ns.Annotations == nil {
		return false, nil
	}
	override, hasOverride := ns.Annotations[api.ActiveDeadlineSecondsOverrideAnnotation]
	if !hasOverride {
		return false, nil
	}
	overrideInt64, err := strconv.ParseInt(override, 10, 64)
	if err != nil {
		return false, fmt.Errorf("cannot parse the ActiveDeadlineSeconds override (%s) for project %s: %v", override, ns.Name, err)
	}
	pod.Spec.ActiveDeadlineSeconds = &overrideInt64
	return true, nil
}
