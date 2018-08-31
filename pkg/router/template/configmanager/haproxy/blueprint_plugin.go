package haproxy

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routev1 "github.com/openshift/api/route/v1"
	templaterouter "github.com/openshift/origin/pkg/router/template"
)

// BlueprintPlugin implements the router.Plugin interface to process routes
// from the blueprint namespace for the associated config manager.
type BlueprintPlugin struct {
	manager templaterouter.ConfigManager
}

// NewBlueprintPlugin returns a new blueprint routes plugin.
func NewBlueprintPlugin(cm templaterouter.ConfigManager) *BlueprintPlugin {
	return &BlueprintPlugin{manager: cm}
}

// HandleRoute processes watch events on blueprint routes.
func (p *BlueprintPlugin) HandleRoute(eventType watch.EventType, route *routev1.Route) error {
	switch eventType {
	case watch.Added, watch.Modified:
		p.manager.AddBlueprint(route)
	case watch.Deleted:
		p.manager.RemoveBlueprint(route)
	}

	return nil
}

// HandleNode processes watch events on the Node resource.
func (p *BlueprintPlugin) HandleNode(eventType watch.EventType, node *kapi.Node) error {
	return nil
}

// HandleEndpoints processes watch events on the Endpoints resource.
func (p *BlueprintPlugin) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) error {
	return nil
}

// HandleNamespaces processes watch events on namespaces.
func (p *BlueprintPlugin) HandleNamespaces(namespaces sets.String) error {
	return nil
}

// Commit commits the changes made to a watched resource.
func (p *BlueprintPlugin) Commit() error {
	// Nothing to do as the config manager does an automatic commit when
	// any blueprint routes change.
	return nil
}
