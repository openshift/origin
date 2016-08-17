package imagepolicy

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/api/meta"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/rules"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

var errRejectByPolicy = fmt.Errorf("this image is prohibited by policy")

type policyDecisions map[kapi.ObjectReference]policyDecision

type policyDecision struct {
	attrs  *rules.ImagePolicyAttributes
	tested bool
	err    error
}

func accept(accepter rules.Accepter, resolver imageResolver, m meta.ImageReferenceMutator, attr admission.Attributes, excludedRules sets.String) error {
	var decisions policyDecisions

	gr := attr.GetResource().GroupResource()
	requiresImage := accepter.RequiresImage(gr)
	resolvesImage := accepter.ResolvesImage(gr)

	errs := m.Mutate(func(ref *kapi.ObjectReference) error {
		// create the attribute set for this particular reference, if we have never seen the reference
		// before
		decision, ok := decisions[*ref]
		if !ok {
			var attrs *rules.ImagePolicyAttributes
			var err error
			if requiresImage || resolvesImage {
				// convert the incoming reference into attributes to pass to the accepter
				attrs, err = resolver.ResolveObjectReference(ref, attr.GetNamespace())
			}
			// if the incoming reference is of a Kind that needed a lookup, but that lookup failed,
			// use the most generic policy rule here because we don't even know the image name
			if attrs == nil {
				attrs = &rules.ImagePolicyAttributes{}

				// an objectref that is DockerImage ref will have a name that corresponds to its pull spec.  We can parse that
				// to a docker image ref
				if ref != nil && ref.Kind == "DockerImage" {
					attrs.Name, _ = imageapi.ParseDockerImageReference(ref.Name)
				}
			}
			attrs.Resource = gr
			attrs.ExcludedRules = excludedRules

			decision.attrs = attrs
			decision.err = err
		}

		// we only need to test a given input once for acceptance
		if !decision.tested {
			accepted := accepter.Accepts(decision.attrs)
			glog.V(5).Infof("Made decision for %v (as: %v, err: %v): %t", ref, decision.attrs.Name, decision.err, accepted)

			// remember this decision for any identical reference
			if decisions == nil {
				decisions = make(policyDecisions)
			}
			decision.tested = true
			decisions[*ref] = decision

			if !accepted {
				// if the image is rejected, return the resolution error, if any
				if decision.err != nil {
					return decision.err
				}
				return errRejectByPolicy
			}
		}

		// if resolution was requested, and no error was present, transform the
		// reference back into a string to a DockerImage
		if resolvesImage && decision.err == nil {
			ref.Namespace = ""
			ref.Name = decision.attrs.Name.Exact()
			ref.Kind = "DockerImage"
		}

		if decision.err != nil {
			glog.V(5).Infof("Ignored resolution error for %v: %v", ref, decision.err)
		}

		return nil
	})

	for i := range errs {
		errs[i].Type = field.ErrorTypeForbidden
		if errs[i].Detail != errRejectByPolicy.Error() {
			errs[i].Detail = fmt.Sprintf("this image is prohibited by policy: %s", errs[i].Detail)
		}
	}

	if len(errs) > 0 {
		glog.V(5).Infof("failed to create: %v", errs)
		return apierrs.NewInvalid(attr.GetKind().GroupKind(), attr.GetName(), errs)
	}
	glog.V(5).Infof("allowed: %#v", attr)
	return nil
}
