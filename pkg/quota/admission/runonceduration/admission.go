package runonceduration

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/golang/glog"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/util/integer"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/admission/apis/runonceduration"
	"github.com/openshift/origin/pkg/quota/admission/apis/runonceduration/validation"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("RunOnceDuration",
		func(config io.Reader) (admission.Interface, error) {
			pluginConfig, err := readConfig(config)
			if err != nil {
				return nil, err
			}
			if pluginConfig == nil {
				glog.Infof("Admission plugin %q is not configured so it will be disabled.", "RunOnceDuration")
				return nil, nil
			}
			return NewRunOnceDuration(pluginConfig), nil
		})
}

func readConfig(reader io.Reader) (*runonceduration.RunOnceDurationConfig, error) {
	obj, err := configlatest.ReadYAML(reader)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*runonceduration.RunOnceDurationConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object %#v", obj)
	}
	errs := validation.ValidateRunOnceDurationConfig(config)
	if len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	return config, nil
}

// NewRunOnceDuration creates a new RunOnceDuration admission plugin
func NewRunOnceDuration(config *runonceduration.RunOnceDurationConfig) admission.Interface {
	return &runOnceDuration{
		Handler: admission.NewHandler(admission.Create),
		config:  config,
	}
}

type runOnceDuration struct {
	*admission.Handler
	config *runonceduration.RunOnceDurationConfig
	cache  *projectcache.ProjectCache
}

var _ = oadmission.WantsProjectCache(&runOnceDuration{})

func (a *runOnceDuration) Admit(attributes admission.Attributes) error {
	switch {
	case a.config == nil,
		attributes.GetResource().GroupResource() != kapi.Resource("pods"),
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

	appliedProjectLimit, err := a.applyProjectAnnotationLimit(attributes.GetNamespace(), pod)
	if err != nil {
		return admission.NewForbidden(attributes, err)
	}

	if !appliedProjectLimit && a.config.ActiveDeadlineSecondsLimit != nil {
		pod.Spec.ActiveDeadlineSeconds = int64MinP(a.config.ActiveDeadlineSecondsLimit, pod.Spec.ActiveDeadlineSeconds)
	}
	return nil
}

func (a *runOnceDuration) SetProjectCache(cache *projectcache.ProjectCache) {
	a.cache = cache
}

func (a *runOnceDuration) ValidateInitialization() error {
	if a.cache == nil {
		return errors.New("RunOnceDuration plugin requires a project cache")
	}
	return nil
}

func (a *runOnceDuration) applyProjectAnnotationLimit(namespace string, pod *kapi.Pod) (bool, error) {
	ns, err := a.cache.GetNamespace(namespace)
	if err != nil {
		return false, fmt.Errorf("error looking up pod namespace: %v", err)
	}
	if ns.Annotations == nil {
		return false, nil
	}
	limit, hasLimit := ns.Annotations[runonceduration.ActiveDeadlineSecondsLimitAnnotation]
	if !hasLimit {
		return false, nil
	}
	limitInt64, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		return false, fmt.Errorf("cannot parse the ActiveDeadlineSeconds limit (%s) for project %s: %v", limit, ns.Name, err)
	}
	pod.Spec.ActiveDeadlineSeconds = int64MinP(&limitInt64, pod.Spec.ActiveDeadlineSeconds)
	return true, nil
}

func int64MinP(a, b *int64) *int64 {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	default:
		c := integer.Int64Min(*a, *b)
		return &c
	}
}
