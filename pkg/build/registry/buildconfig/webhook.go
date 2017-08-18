package buildconfig

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/client"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/util/rest"
)

// NewWebHookREST returns the webhook handler wrapped in a rest.WebHook object.
func NewWebHookREST(buildConfigClient buildclient.BuildInterface, groupVersion schema.GroupVersion, plugins map[string]webhook.Plugin) *rest.WebHook {
	return newWebHookREST(buildConfigClient, client.BuildConfigInstantiatorClient{BuildClient: buildConfigClient}, groupVersion, plugins)
}

// this supports simple unit testing
func newWebHookREST(buildConfigClient buildclient.BuildInterface, instantiator client.BuildConfigInstantiator, groupVersion schema.GroupVersion, plugins map[string]webhook.Plugin) *rest.WebHook {
	hook := &WebHook{
		groupVersion:      groupVersion,
		buildConfigClient: buildConfigClient,
		instantiator:      instantiator,
		plugins:           plugins,
	}
	return rest.NewWebHook(hook, false)
}

type WebHook struct {
	groupVersion      schema.GroupVersion
	buildConfigClient buildclient.BuildInterface
	instantiator      client.BuildConfigInstantiator
	plugins           map[string]webhook.Plugin
}

// ServeHTTP implements rest.HookHandler
func (w *WebHook) ServeHTTP(writer http.ResponseWriter, req *http.Request, ctx apirequest.Context, name, subpath string) error {
	parts := strings.Split(subpath, "/")
	if len(parts) != 2 {
		return errors.NewBadRequest(fmt.Sprintf("unexpected hook subpath %s", subpath))
	}
	secret, hookType := parts[0], parts[1]

	plugin, ok := w.plugins[hookType]
	if !ok {
		return errors.NewNotFound(buildapi.LegacyResource("buildconfighook"), hookType)
	}

	config, err := w.buildConfigClient.BuildConfigs(apirequest.NamespaceValue(ctx)).Get(name, metav1.GetOptions{})
	if err != nil {
		// clients should not be able to find information about build configs in
		// the system unless the config exists and the secret matches
		return errors.NewUnauthorized(fmt.Sprintf("the webhook %q for %q did not accept your secret", hookType, name))
	}

	revision, envvars, dockerStrategyOptions, proceed, err := plugin.Extract(config, secret, "", req)
	if !proceed {
		switch err {
		case webhook.ErrSecretMismatch, webhook.ErrHookNotEnabled:
			return errors.NewUnauthorized(fmt.Sprintf("the webhook %q for %q did not accept your secret", hookType, name))
		case webhook.MethodNotSupported:
			return errors.NewMethodNotSupported(buildapi.Resource("buildconfighook"), req.Method)
		}
		if _, ok := err.(*errors.StatusError); !ok && err != nil {
			return errors.NewInternalError(fmt.Errorf("hook failed: %v", err))
		}
		return err
	}
	warning := err

	buildTriggerCauses := generateBuildTriggerInfo(revision, hookType, secret)
	request := &buildapi.BuildRequest{
		TriggeredBy: buildTriggerCauses,
		ObjectMeta:  metav1.ObjectMeta{Name: name},
		Revision:    revision,
		Env:         envvars,
		DockerStrategyOptions: dockerStrategyOptions,
	}

	newBuild, err := w.instantiator.Instantiate(config.Namespace, request)
	if err != nil {
		return errors.NewInternalError(fmt.Errorf("could not generate a build: %v", err))
	}

	// Send back the build name so that the client can alert the user.
	if newBuildEncoded, err := runtime.Encode(kapi.Codecs.LegacyCodec(w.groupVersion), newBuild); err != nil {
		utilruntime.HandleError(err)
	} else {
		writer.Write(newBuildEncoded)
	}

	return warning
}

func generateBuildTriggerInfo(revision *buildapi.SourceRevision, hookType, secret string) (buildTriggerCauses []buildapi.BuildTriggerCause) {
	hiddenSecret := fmt.Sprintf("%s***", secret[:(len(secret)/2)])
	switch {
	case hookType == "generic":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseGenericMsg,
				GenericWebHook: &buildapi.GenericWebHookCause{
					Revision: revision,
					Secret:   hiddenSecret,
				},
			})
	case hookType == "github":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseGithubMsg,
				GitHubWebHook: &buildapi.GitHubWebHookCause{
					Revision: revision,
					Secret:   hiddenSecret,
				},
			})
	case hookType == "gitlab":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseGitLabMsg,
				GitLabWebHook: &buildapi.GitLabWebHookCause{
					CommonWebHookCause: buildapi.CommonWebHookCause{
						Revision: revision,
						Secret:   hiddenSecret,
					},
				},
			})
	case hookType == "bitbucket":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseBitbucketMsg,
				BitbucketWebHook: &buildapi.BitbucketWebHookCause{
					CommonWebHookCause: buildapi.CommonWebHookCause{
						Revision: revision,
						Secret:   hiddenSecret,
					},
				},
			})
	}
	return buildTriggerCauses
}
