package router

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	oclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util"
	controllerfactory "github.com/openshift/origin/pkg/router/controller/factory"
)

// RouterSelection controls what routes and resources on the server are considered
// part of this router.
type RouterSelection struct {
	LabelSelector string
	Labels        labels.Selector
	FieldSelector string
	Fields        fields.Selector

	Namespace string

	ResyncInterval time.Duration
}

// Bind sets the appropriate labels
func (o *RouterSelection) Bind(flag *pflag.FlagSet) {
	flag.StringVar(&o.LabelSelector, "labels", util.Env("ROUTE_LABELS", ""), "A label selector to apply to the routes to watch")
	flag.StringVar(&o.FieldSelector, "fields", util.Env("ROUTE_FIELDS", ""), "A field selector to apply to routes to watch")
	flag.DurationVar(&o.ResyncInterval, "resync-interval", 10*time.Minute, "The interval at which the route list should be fully refreshed")
}

// Complete converts string representations of field and label selectors to their parsed equivalent, or
// returns an error.
func (o *RouterSelection) Complete() error {
	if len(o.LabelSelector) > 0 {
		s, err := labels.Parse(o.LabelSelector)
		if err != nil {
			return fmt.Errorf("label selector is not valid: %v", err)
		}
		o.Labels = s
	} else {
		o.Labels = labels.Everything()
	}

	if len(o.FieldSelector) > 0 {
		s, err := fields.ParseSelector(o.FieldSelector)
		if err != nil {
			return fmt.Errorf("field selector is not valid: %v", err)
		}
		o.Fields = s
	} else {
		o.Fields = fields.Everything()
	}
	return nil
}

// NewFactory initializes a factory that will watch the requested routes
func (o *RouterSelection) NewFactory(oc oclient.Interface, kc kclient.Interface) *controllerfactory.RouterControllerFactory {
	factory := controllerfactory.NewDefaultRouterControllerFactory(oc, kc)
	factory.Labels = o.Labels
	factory.Fields = o.Fields
	factory.Namespace = o.Namespace
	factory.ResyncInterval = o.ResyncInterval
	if len(factory.Namespace) > 0 {
		glog.Infof("Router is only using resources in namespace %s", factory.Namespace)
	}
	return factory
}
