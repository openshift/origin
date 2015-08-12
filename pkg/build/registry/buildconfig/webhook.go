package buildconfig

import (
	"fmt"
	"net/http"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/registry/generic/rest"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/webhook"
)

func NewWebHookREST(registry Registry, instantiator client.BuildConfigInstantiator, plugins map[string]webhook.Plugin) *rest.WebHook {
	controller := &controller{
		registry:     registry,
		instantiator: instantiator,
		plugins:      plugins,
	}
	return rest.NewWebHook(controller, false)
}

type controller struct {
	registry     Registry
	instantiator client.BuildConfigInstantiator
	plugins      map[string]webhook.Plugin
}

// ServeHTTP implements rest.HookHandler
func (c *controller) ServeHTTP(w http.ResponseWriter, req *http.Request, ctx kapi.Context, name, subpath string) error {
	parts := strings.Split(subpath, "/")
	if len(parts) < 2 {
		return errors.NewBadRequest(fmt.Sprintf("unexpected hook subpath %s", subpath))
	}
	secret, hookType := parts[0], parts[1]

	plugin, ok := c.plugins[hookType]
	if !ok {
		return errors.NewNotFound("BuildConfigHook", hookType)
	}

	config, err := c.registry.GetBuildConfig(ctx, name)
	if err != nil {
		// clients should not be able to find information about build configs in the system unless the config exists
		// and the secret matches
		return errors.NewUnauthorized(fmt.Sprintf("the webhook %q for %q did not accept your secret", hookType, name))
	}

	revision, proceed, err := plugin.Extract(config, secret, "", req)
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

	request := &buildapi.BuildRequest{
		ObjectMeta: kapi.ObjectMeta{Name: name},
		Revision:   revision,
	}
	if _, err := c.instantiator.Instantiate(config.Namespace, request); err != nil {
		return errors.NewInternalError(fmt.Errorf("could not generate a build: %v", err))
	}
	return nil
}
