package cluster

import (
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	authorizationtypedclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/util"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/route/apis/route/validation"
	clientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
)

// RouteCertificateValidation is a Diagnostic to check that there is a working router.
type RouteCertificateValidation struct {
	SARClient  authorizationtypedclient.SelfSubjectAccessReviewsGetter
	RESTConfig *rest.Config
}

const (
	RouteCertificateValidationName = "RouteCertificateValidation"

	clGetRoutesFailed = `
Client error while retrieving all routes. Client retrieved records
before, so this is likely to be a transient error. Try running
diagnostics again. If this message persists, there may be a permissions
problem with getting records. The error was:

(%[1]T) %[1]v`
)

func (d *RouteCertificateValidation) Name() string {
	return "RouteCertificateValidation"
}

func (d *RouteCertificateValidation) Description() string {
	return "Check all route certificates for certificates that might be rejected by extended validation."
}

func (d *RouteCertificateValidation) Requirements() (client bool, host bool) {
	return true, false
}

func (d *RouteCertificateValidation) CanRun() (bool, error) {
	if d.RESTConfig == nil || d.SARClient == nil {
		return false, errors.New("must have Kube client configuration")
	}
	can, err := util.UserCan(d.SARClient, &authorization.ResourceAttributes{
		Namespace: metav1.NamespaceAll,
		Verb:      "get",
		Group:     routeapi.GroupName,
		Resource:  "routes",
	})
	if err != nil {
		return false, types.DiagnosticError{ID: "DRouCert2010", LogMessage: fmt.Sprintf(clientAccessError, err), Cause: err}
	} else if !can {
		return false, types.DiagnosticError{ID: "DRouCert2011", LogMessage: "Client does not have cluster-admin access", Cause: err}
	}
	return true, nil
}

func (d *RouteCertificateValidation) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(RouteCertificateValidationName)

	client, err := clientset.NewForConfig(d.RESTConfig)
	if err != nil {
		r.Error("DRouCert2012", err, fmt.Sprintf(clientAccessError, err))
		return r
	}

	routes, err := client.Route().Routes(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		r.Error("DRouCert2013", err, fmt.Sprintf(clGetRoutesFailed, err))
		return r
	}

	for _, route := range routes.Items {
		original := route.DeepCopy()
		errs := validation.ExtendedValidateRoute(&route)

		if len(errs) == 0 {
			if !kapihelper.Semantic.DeepEqual(original, &route) {
				err := fmt.Errorf("Route was normalized when extended validation was run (route/%s -n %s).\nPlease verify that this route certificate contains no invalid data.\n", route.Name, route.Namespace)
				r.Warn("DRouCert2004", nil, err.Error())
			}
			continue
		}
		err = fmt.Errorf("Route failed extended validation (route/%s -n %s):\n%s", route.Name, route.Namespace, flattenErrors(errs))
		r.Error("DRouCert2005", nil, err.Error())
	}
	return r
}

func flattenErrors(errs field.ErrorList) string {
	var out []string
	for i := range errs {
		out = append(out, fmt.Sprintf("* %s", errs[i].Error()))
	}
	return strings.Join(out, "\n")
}
