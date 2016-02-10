package requestlimit

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	requestlimitapi "github.com/openshift/origin/pkg/project/admission/requestlimit/api"
	requestlimitapivalidation "github.com/openshift/origin/pkg/project/admission/requestlimit/api/validation"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectcache "github.com/openshift/origin/pkg/project/cache"
)

func init() {
	admission.RegisterPlugin("ProjectRequestLimit", func(client kclient.Interface, config io.Reader) (admission.Interface, error) {
		pluginConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return NewProjectRequestLimit(pluginConfig)
	})
}

func readConfig(reader io.Reader) (*requestlimitapi.ProjectRequestLimitConfig, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return &requestlimitapi.ProjectRequestLimitConfig{}, nil
	}

	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	config := &requestlimitapi.ProjectRequestLimitConfig{}
	err = configlatest.ReadYAML(configBytes, config)
	if err != nil {
		return nil, err
	}
	errs := requestlimitapivalidation.ValidateProjectRequestLimitConfig(config)
	if len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	return config, nil
}

type projectRequestLimit struct {
	*admission.Handler
	client client.Interface
	config *requestlimitapi.ProjectRequestLimitConfig
	cache  *projectcache.ProjectCache
}

// ensure that the required Openshift admission interfaces are implemented
var _ = oadmission.WantsProjectCache(&projectRequestLimit{})
var _ = oadmission.WantsOpenshiftClient(&projectRequestLimit{})
var _ = oadmission.Validator(&projectRequestLimit{})

// Admit ensures that only a configured number of projects can be requested by a particular user.
func (o *projectRequestLimit) Admit(a admission.Attributes) (err error) {
	if a.GetResource() != projectapi.Resource("projectrequests") {
		return nil
	}
	if _, isProjectRequest := a.GetObject().(*projectapi.ProjectRequest); !isProjectRequest {
		return nil
	}
	userName := a.GetUserInfo().GetName()
	projectCount, err := o.projectCountByRequester(userName)
	if err != nil {
		return err
	}
	maxProjects, hasLimit, err := o.maxProjectsByRequester(userName)
	if err != nil {
		return err
	}
	if hasLimit && projectCount >= maxProjects {
		return admission.NewForbidden(a, fmt.Errorf("user %s cannot create more than %d project(s).", userName, maxProjects))
	}
	return nil
}

// maxProjectsByRequester returns the maximum number of projects allowed for a given user, whether a limit exists, and an error
// if an error occurred. If a limit doesn't exist, the maximum number should be ignored.
func (o *projectRequestLimit) maxProjectsByRequester(userName string) (int, bool, error) {
	// prevent a user lookup if no limits are configured
	if len(o.config.Limits) == 0 {
		return 0, false, nil
	}

	user, err := o.client.Users().Get(userName)
	if err != nil {
		return 0, false, err
	}
	userLabels := labels.Set(user.Labels)

	for _, limit := range o.config.Limits {
		selector := labels.Set(limit.Selector).AsSelector()
		if selector.Matches(userLabels) {
			if limit.MaxProjects == nil {
				return 0, false, nil
			}
			return *limit.MaxProjects, true, nil
		}
	}
	return 0, false, nil
}

func (o *projectRequestLimit) projectCountByRequester(userName string) (int, error) {
	namespaces, err := o.cache.Store.ByIndex("requester", userName)
	if err != nil {
		return 0, err
	}
	return len(namespaces), nil
}

func (o *projectRequestLimit) SetOpenshiftClient(client client.Interface) {
	o.client = client
}

func (o *projectRequestLimit) SetProjectCache(cache *projectcache.ProjectCache) {
	o.cache = cache
}

func (o *projectRequestLimit) Validate() error {
	if o.client == nil {
		return fmt.Errorf("ProjectRequestLimit plugin requires an Openshift client")
	}
	if o.cache == nil {
		return fmt.Errorf("ProjectRequestLimit plugin requires a project cache")
	}
	return nil
}

func NewProjectRequestLimit(config *requestlimitapi.ProjectRequestLimitConfig) (admission.Interface, error) {
	return &projectRequestLimit{
		config:  config,
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

func projectRequester(ns *kapi.Namespace) string {
	return ns.Annotations[projectapi.ProjectRequester]
}
