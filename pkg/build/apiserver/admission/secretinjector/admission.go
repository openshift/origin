package secretinjector

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	restclient "k8s.io/client-go/rest"
	api "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	authclient "github.com/openshift/origin/pkg/client/impersonatingclient"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/util/urlpattern"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("build.openshift.io/BuildConfigSecretInjector",
		func(config io.Reader) (admission.Interface, error) {
			return &secretInjector{
				Handler: admission.NewHandler(admission.Create),
			}, nil
		})
}

type secretInjector struct {
	*admission.Handler
	restClientConfig restclient.Config
}

var _ = oadmission.WantsRESTClientConfig(&secretInjector{})
var _ = admission.MutationInterface(&secretInjector{})
var _ = admission.ValidationInterface(&secretInjector{})

func (si *secretInjector) Admit(attr admission.Attributes) (err error) {
	return si.admit(attr, true)
}

func (si *secretInjector) Validate(attr admission.Attributes) (err error) {
	return si.admit(attr, false)
}

func (si *secretInjector) admit(attr admission.Attributes, mutationAllowed bool) (err error) {
	bc, ok := attr.GetObject().(*buildapi.BuildConfig)
	if !ok {
		return nil
	}

	if bc.Spec.Source.SourceSecret != nil || bc.Spec.Source.Git == nil {
		return nil
	}

	client, err := authclient.NewImpersonatingKubernetesClientset(attr.GetUserInfo(), si.restClientConfig)
	if err != nil {
		glog.V(2).Infof("secretinjector: could not create client: %v", err)
		return nil
	}

	namespace := attr.GetNamespace()

	url, err := url.Parse(bc.Spec.Source.Git.URI)
	if err != nil {
		glog.V(2).Infof(`secretinjector: buildconfig "%s/%s": URI %q parse failed: %v`, namespace, bc.GetName(), bc.Spec.Source.Git.URI, err)
		return nil
	}

	secrets, err := client.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	if err != nil {
		glog.V(2).Infof("secretinjector: failed to list Secrets: %v", err)
		return nil
	}

	patterns := []*urlpattern.URLPattern{}
	for _, secret := range secrets.Items {
		if secret.Type == corev1.SecretTypeBasicAuth && url.Scheme == "ssh" ||
			secret.Type == corev1.SecretTypeSSHAuth && url.Scheme != "ssh" {
			continue
		}

		for k, v := range secret.GetAnnotations() {
			if strings.HasPrefix(k, buildapi.BuildSourceSecretMatchURIAnnotationPrefix) {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}

				pattern, err := urlpattern.NewURLPattern(v)
				if err != nil {
					glog.V(2).Infof(`secretinjector: buildconfig "%s/%s": unparseable annotation %q: %v`, namespace, bc.GetName(), k, err)
					continue
				}

				pattern.Cookie = secret.GetName()
				patterns = append(patterns, pattern)
			}
		}
	}

	if match := urlpattern.Match(patterns, url); match != nil {
		secretName := match.Cookie.(string)
		glog.V(4).Infof(`secretinjector: matched secret "%s/%s" to buildconfig "%s"`, namespace, secretName, bc.GetName())
		if mutationAllowed {
			bc.Spec.Source.SourceSecret = &api.LocalObjectReference{Name: secretName}
		} else {
			return admission.NewForbidden(attr, fmt.Errorf("mutated spec.source.sourceSecret, expected: %v, got %v", api.LocalObjectReference{Name: secretName}, bc.Spec.Source.SourceSecret))
		}
	}

	return nil
}

func (si *secretInjector) SetRESTClientConfig(restClientConfig restclient.Config) {
	si.restClientConfig = restClientConfig
}

func (si *secretInjector) ValidateInitialization() error {
	return nil
}
