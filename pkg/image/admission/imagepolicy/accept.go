package imagepolicy

import (
	"fmt"

	"github.com/golang/glog"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/api/meta"
	"github.com/openshift/origin/pkg/image/admission/apis/imagepolicy"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/rules"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

var errRejectByPolicy = fmt.Errorf("this image is prohibited by policy")

type policyDecisions map[kapi.ObjectReference]policyDecision

type policyDecision struct {
	attrs         *rules.ImagePolicyAttributes
	tested        bool
	resolutionErr error
}

func accept(accepter rules.Accepter, policy imageResolutionPolicy, resolver imageResolver, m meta.ImageReferenceMutator, annotations meta.AnnotationAccessor, attr admission.Attributes, excludedRules sets.String) error {
	decisions := policyDecisions{}

	gr := attr.GetResource().GroupResource()

	var resolveAllNames bool
	if annotations != nil {
		if a, ok := annotations.TemplateAnnotations(); ok {
			resolveAllNames = a[imagepolicy.ResolveNamesAnnotation] == "*"
		}
		if !resolveAllNames {
			resolveAllNames = annotations.Annotations()[imagepolicy.ResolveNamesAnnotation] == "*"
		}
	}

	errs := m.Mutate(func(ref *kapi.ObjectReference) error {
		// create the attribute set for this particular reference, if we have never seen the reference
		// before
		decision, ok := decisions[*ref]
		if !ok {
			if policy.RequestsResolution(gr) {
				resolvedAttrs, err := resolver.ResolveObjectReference(ref, attr.GetNamespace(), resolveAllNames)
				switch {
				case err != nil && policy.FailOnResolutionFailure(gr):
					glog.V(5).Infof("resource failed on error during required image resolution: %v", err)
					// if we had a resolution error and we're supposed to fail, fail
					decision.resolutionErr = err
					decision.tested = true
					decisions[*ref] = decision
					return err

				case err != nil:
					glog.V(5).Infof("error during optional image resolution: %v", err)
					// if we had an error, but aren't supposed to fail, just don't do anything else and keep track of
					// the resolution failure
					decision.resolutionErr = err

				case err == nil:
					// if we resolved properly, assign the attributes and rewrite the pull spec if we need to
					decision.attrs = resolvedAttrs

					if policy.RewriteImagePullSpec(resolvedAttrs, attr.GetOperation() == admission.Update, gr) {
						ref.Namespace = ""
						ref.Name = decision.attrs.Name.Exact()
						ref.Kind = "DockerImage"
					}
				}
			}
			// if we don't have any image policy attributes, attempt a best effort parse for the remaining tests
			if decision.attrs == nil {
				decision.attrs = &rules.ImagePolicyAttributes{}

				// an objectref that is DockerImage ref will have a name that corresponds to its pull spec.  We can parse that
				// to a docker image ref
				if ref != nil && ref.Kind == "DockerImage" {
					decision.attrs.Name, _ = imageapi.ParseDockerImageReference(ref.Name)
				}
			}
			decision.attrs.Resource = gr
			decision.attrs.ExcludedRules = excludedRules
			glog.V(5).Infof("post resolution, ref=%s:%s/%s, image attributes=%#v, resolution err=%v", ref.Kind, ref.Name, ref.Namespace, *decision.attrs, decision.resolutionErr)
		}

		// we only need to test a given input once for acceptance
		if !decision.tested {
			accepted := accepter.Accepts(decision.attrs)
			glog.V(5).Infof("Made decision for %v (as: %v, resolution err: %v): accept=%t", ref, decision.attrs.Name, decision.resolutionErr, accepted)

			decision.tested = true
			decisions[*ref] = decision

			if !accepted {
				// if the image is rejected due to a resolution error, return the resolution error
				// This is a dubious result.  It's entirely possible we had an error resolving the image,
				// but no rule actually requires image resolution and the image was rejected for some other
				// reason.  The user will then see that it was rejected due to the resolution error, but
				// that isn't really why it was rejected.  Better logic would check if the rule that
				// rejected the image, required resolution, and only then report the resolution falure.
				if decision.resolutionErr != nil {
					return decision.resolutionErr
				}
				// otherwise the image is being rejected by policy
				return errRejectByPolicy
			}
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
		glog.V(5).Infof("image policy admission rejecting due to: %v", errs)
		return apierrs.NewInvalid(attr.GetKind().GroupKind(), attr.GetName(), errs)
	}
	return nil
}
