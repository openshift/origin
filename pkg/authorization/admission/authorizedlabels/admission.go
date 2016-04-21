package authorizedlabels

import (
	"fmt"
	"io"
	"strings"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
)

func init() {
	admission.RegisterPlugin("LabelAuthorization", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		return &labelEnforcer{
			Handler: admission.NewHandler(admission.Create, admission.Update, admission.Delete),
		}, nil
	})
}

type labelEnforcer struct {
	*admission.Handler
	ruleResolver rulevalidation.AuthorizationRuleResolver

	// TODO replace this with shared caches after we get SharedIndexInformers from upstream
	clientConfig restclient.Config
}

// ensure that the required Openshift admission interfaces are implemented
var _ = oadmission.WantsAuthorizationRuleResolver(&labelEnforcer{})
var _ = oadmission.WantsClientConfig(&labelEnforcer{})
var _ = oadmission.Validator(&labelEnforcer{})

// Admit ensures that only a configured number of projects can be requested by a particular user.
func (o *labelEnforcer) Admit(a admission.Attributes) (err error) {
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), a.GetNamespace()), a.GetUserInfo())

	rules, err := o.ruleResolver.GetEffectivePolicyRules(ctx)
	if err != nil {
		return err
	}
	filteredRules := []authorizationapi.PolicyRule{}

	authAttributes := authorizer.DefaultAuthorizationAttributes{
		Verb:         strings.ToLower(string(a.GetOperation())),
		APIGroup:     a.GetResource().Group,
		Resource:     a.GetResource().Resource,
		ResourceName: a.GetName(),
	}

	// means delete collection
	if a.GetOperation() == admission.Delete && len(a.GetName()) == 0 {
		return nil
	}

	// filter the rules to only get rules with a selector, with a matching verb, with a matching group resource tuple
	for _, rule := range rules {
		matches, err := authAttributes.RuleMatches(rule)
		if err != nil {
			return err
		}
		if !matches {
			continue
		}
		if len(rule.Selector) == 0 {
			// if we match the rule, but have no selector, then this request should succeed
			return nil
		}

		filteredRules = append(filteredRules, rule)
	}

	// without any rules to apply, this succeeds
	if len(filteredRules) == 0 {
		return nil
	}

	var objectLabels map[string]string
	if a.GetOperation() == admission.Create {
		// when we do a create we have the object we're interested in right here
		var err error
		objectLabels, err = meta.NewAccessor().Labels(a.GetObject())
		if err != nil {
			return err
		}

	} else {
		// otherwise, we need to get the current object.  Without a rewrite of upstream admission
		// this is the best we can get.
		config := o.clientConfig
		gv := a.GetResource().GroupVersion()
		config.GroupVersion = &gv
		client, err := restclient.RESTClientFor(&config)
		if err != nil {
			return err
		}

		// TODO use generic client here
		originalObject := &kapi.ResourceQuota{}
		if err := client.Get().Namespace(a.GetNamespace()).Resource(a.GetResource().Resource).Name(a.GetName()).Do().Into(originalObject); err != nil {
			return err
		}
		objectLabels, err = meta.NewAccessor().Labels(originalObject)
		if err != nil {
			return err
		}
	}

	for _, rule := range filteredRules {
		selector := labels.Set(rule.Selector).AsSelector()
		if selector.Matches(labels.Set(objectLabels)) {
			return nil
		}
	}

	return admission.NewForbidden(a, fmt.Errorf("%v does not match %v", objectLabels, filteredRules))
}

func (o *labelEnforcer) SetAuthorizationRuleResolver(ruleResolver rulevalidation.AuthorizationRuleResolver) {
	o.ruleResolver = ruleResolver
}
func (o *labelEnforcer) SetClientConfig(clientConfig restclient.Config) {
	o.clientConfig = clientConfig
	o.clientConfig.APIPath = "api"
	o.clientConfig.Codec = kapi.Codecs.LegacyCodec(kapiv1.SchemeGroupVersion)
}

func (o *labelEnforcer) Validate() error {
	if o.ruleResolver == nil {
		return fmt.Errorf("ruleResolver is required")
	}
	if o.clientConfig.Codec == nil {
		return fmt.Errorf("clientConfig.Codec is required")
	}
	return nil
}
