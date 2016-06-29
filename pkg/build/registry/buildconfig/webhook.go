package buildconfig

import (
	"fmt"
	"net/http"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/util/rest"
)

// NewWebHookREST returns the webhook handler wrapped in a rest.WebHook object.
func NewWebHookREST(registry Registry, instantiator client.BuildConfigInstantiator, plugins map[string]webhook.Plugin) *rest.WebHook {
	hook := &WebHook{
		registry:     registry,
		instantiator: instantiator,
		plugins:      plugins,
	}
	return rest.NewWebHook(hook, false)
}

type WebHook struct {
	registry     Registry
	instantiator client.BuildConfigInstantiator
	plugins      map[string]webhook.Plugin
}

// ServeHTTP implements rest.HookHandler
func (w *WebHook) ServeHTTP(writer http.ResponseWriter, req *http.Request, ctx kapi.Context, name, subpath string) error {
	parts := strings.Split(subpath, "/")
	if len(parts) != 2 {
		return errors.NewBadRequest(fmt.Sprintf("unexpected hook subpath %s", subpath))
	}
	secret, hookType := parts[0], parts[1]

	plugin, ok := w.plugins[hookType]
	if !ok {
		return errors.NewNotFound(buildapi.Resource("buildconfighook"), hookType)
	}

	config, err := w.registry.GetBuildConfig(ctx, name)
	if err != nil {
		// clients should not be able to find information about build configs in
		// the system unless the config exists and the secret matches
		return errors.NewUnauthorized(fmt.Sprintf("the webhook %q for %q did not accept your secret", hookType, name))
	}

	revision, envvars, proceed, err := plugin.Extract(config, secret, "", req)
	switch err {
	case webhook.ErrSecretMismatch, webhook.ErrHookNotEnabled:
		return errors.NewUnauthorized(fmt.Sprintf("the webhook %q for %q did not accept your secret", hookType, name))
	case nil:
	default:
		return errors.NewInternalError(fmt.Errorf("hook failed: %v", err))
	}

	if !proceed {
		return nil
	}

	buildTriggerCauses := generateBuildTriggerInfo(revision, hookType, secret)
	request := &buildapi.BuildRequest{
		TriggeredBy: buildTriggerCauses,
		ObjectMeta:  kapi.ObjectMeta{Name: name},
		Revision:    revision,
		Env:         envvars,
	}
	if _, err := w.instantiator.Instantiate(config.Namespace, request); err != nil {
		return errors.NewInternalError(fmt.Errorf("could not generate a build: %v", err))
	}
	return nil
}

func generateBuildTriggerInfo(revision *buildapi.SourceRevision, hookType, secret string) (buildTriggerCauses []buildapi.BuildTriggerCause) {
	hiddenSecret := fmt.Sprintf("%s***", secret[:(len(secret)/2)])
	switch {
	case hookType == "generic":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: "Generic WebHook",
				GenericWebHook: &buildapi.GenericWebHookCause{
					Revision: revision,
					Secret:   hiddenSecret,
				},
			})
	case hookType == "github":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: "GitHub WebHook",
				GitHubWebHook: &buildapi.GitHubWebHookCause{
					Revision: revision,
					Secret:   hiddenSecret,
				},
			})
	}
	return buildTriggerCauses
}
