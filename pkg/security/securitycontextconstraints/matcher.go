package securitycontextconstraints

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	allocator "github.com/openshift/origin/pkg/security"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitylisters "github.com/openshift/origin/pkg/security/generated/listers/security/internalversion"
	"github.com/openshift/origin/pkg/security/uid"
)

type SCCMatcher interface {
	FindApplicableSCCs(user user.Info, namespace string) ([]*securityapi.SecurityContextConstraints, error)
}

type defaultSCCMatcher struct {
	cache      securitylisters.SecurityContextConstraintsLister
	authorizer authorizer.Authorizer
}

func NewDefaultSCCMatcher(c securitylisters.SecurityContextConstraintsLister, authorizer authorizer.Authorizer) SCCMatcher {
	return &defaultSCCMatcher{cache: c, authorizer: authorizer}
}

// FindApplicableSCCs implements SCCMatcher interface
func (d *defaultSCCMatcher) FindApplicableSCCs(userInfo user.Info, namespace string) ([]*securityapi.SecurityContextConstraints, error) {
	var matchedConstraints []*securityapi.SecurityContextConstraints
	constraints, err := d.cache.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, constraint := range constraints {
		if ConstraintAppliesTo(constraint, userInfo, namespace, d.authorizer) {
			matchedConstraints = append(matchedConstraints, constraint)
		}
	}
	return matchedConstraints, nil
}

// authorizedForSCC returns true if info is authorized to perform the "use" verb on the SCC resource.
func authorizedForSCC(constraint *securityapi.SecurityContextConstraints, info user.Info, namespace string, a authorizer.Authorizer) bool {
	// check against the namespace that the pod is being created in to allow per-namespace SCC grants.
	attr := authorizer.AttributesRecord{
		User:            info,
		Verb:            "use",
		Namespace:       namespace,
		Name:            constraint.Name,
		APIGroup:        securityapi.GroupName,
		Resource:        "securitycontextconstraints",
		ResourceRequest: true,
	}
	decision, reason, err := a.Authorize(attr)
	if err != nil {
		glog.V(5).Infof("cannot authorize for SCC: %v %q %v", decision, reason, err)
		return false
	}
	return decision == authorizer.DecisionAllow
}

// ConstraintAppliesTo inspects the constraint's users and groups against the userInfo to determine
// if it is usable by the userInfo.
// TODO make this private and have the router SA check do a SAR check instead.
// Anything we do here needs to work with a deny authorizer so the choices are limited to SAR / Authorizer
func ConstraintAppliesTo(constraint *securityapi.SecurityContextConstraints, userInfo user.Info, namespace string, a authorizer.Authorizer) bool {
	for _, user := range constraint.Users {
		if userInfo.GetName() == user {
			return true
		}
	}
	for _, userGroup := range userInfo.GetGroups() {
		if constraintSupportsGroup(userGroup, constraint.Groups) {
			return true
		}
	}
	if a != nil {
		return authorizedForSCC(constraint, userInfo, namespace, a)
	}
	return false
}

// AssignSecurityContext creates a security context for each container in the pod
// and validates that the sc falls within the scc constraints.  All containers must validate against
// the same scc or is not considered valid.
func AssignSecurityContext(provider SecurityContextConstraintsProvider, pod *kapi.Pod, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	psc, generatedAnnotations, err := provider.CreatePodSecurityContext(pod)
	if err != nil {
		errs = append(errs, field.Invalid(fldPath.Child("spec", "securityContext"), pod.Spec.SecurityContext, err.Error()))
	}

	pod.Spec.SecurityContext = psc
	pod.Annotations = generatedAnnotations
	errs = append(errs, provider.ValidatePodSecurityContext(pod, fldPath.Child("spec", "securityContext"))...)

	for i := range pod.Spec.InitContainers {
		sc, err := provider.CreateContainerSecurityContext(pod, &pod.Spec.InitContainers[i])
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("spec", "initContainers").Index(i).Child("securityContext"), "", err.Error()))
			continue
		}
		pod.Spec.InitContainers[i].SecurityContext = sc
		errs = append(errs, provider.ValidateContainerSecurityContext(pod, &pod.Spec.InitContainers[i], field.NewPath("spec", "initContainers").Index(i).Child("securityContext"))...)
	}

	for i := range pod.Spec.Containers {
		sc, err := provider.CreateContainerSecurityContext(pod, &pod.Spec.Containers[i])
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("spec", "containers").Index(i).Child("securityContext"), "", err.Error()))
			continue
		}
		pod.Spec.Containers[i].SecurityContext = sc
		errs = append(errs, provider.ValidateContainerSecurityContext(pod, &pod.Spec.Containers[i], field.NewPath("spec", "containers").Index(i).Child("securityContext"))...)
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// constraintSupportsGroup checks that group is in constraintGroups.
func constraintSupportsGroup(group string, constraintGroups []string) bool {
	for _, g := range constraintGroups {
		if g == group {
			return true
		}
	}
	return false
}

// DeduplicateSecurityContextConstraints ensures we have a unique slice of constraints.
func DeduplicateSecurityContextConstraints(sccs []*securityapi.SecurityContextConstraints) []*securityapi.SecurityContextConstraints {
	deDuped := []*securityapi.SecurityContextConstraints{}
	added := sets.NewString()

	for _, s := range sccs {
		if !added.Has(s.Name) {
			deDuped = append(deDuped, s)
			added.Insert(s.Name)
		}
	}
	return deDuped
}

// getNamespaceByName retrieves a namespace only if ns is nil.
func getNamespaceByName(name string, ns *kapi.Namespace, client clientset.Interface) (*kapi.Namespace, error) {
	if ns != nil && name == ns.Name {
		return ns, nil
	}
	return client.Core().Namespaces().Get(name, metav1.GetOptions{})
}

// CreateProvidersFromConstraints creates providers from the constraints supplied, including
// looking up pre-allocated values if necessary using the pod's namespace.
func CreateProvidersFromConstraints(ns string, sccs []*securityapi.SecurityContextConstraints, client clientset.Interface) ([]SecurityContextConstraintsProvider, []error) {
	var (
		// namespace is declared here for reuse but we will not fetch it unless required by the matched constraints
		namespace *kapi.Namespace
		// collected providers
		providers []SecurityContextConstraintsProvider
		// collected errors to return
		errs []error
	)

	// set pre-allocated values on constraints
	for _, constraint := range sccs {
		var (
			provider SecurityContextConstraintsProvider
			err      error
		)
		provider, namespace, err = CreateProviderFromConstraint(ns, namespace, constraint, client)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		providers = append(providers, provider)
	}
	return providers, errs
}

// CreateProviderFromConstraint creates a SecurityContextConstraintProvider from a SecurityContextConstraint
func CreateProviderFromConstraint(ns string, namespace *kapi.Namespace, constraint *securityapi.SecurityContextConstraints, client clientset.Interface) (SecurityContextConstraintsProvider, *kapi.Namespace, error) {
	var err error
	resolveUIDRange := requiresPreAllocatedUIDRange(constraint)
	resolveSELinuxLevel := requiresPreAllocatedSELinuxLevel(constraint)
	resolveFSGroup := requiresPreallocatedFSGroup(constraint)
	resolveSupplementalGroups := requiresPreallocatedSupplementalGroups(constraint)
	requiresNamespaceAllocations := resolveUIDRange || resolveSELinuxLevel || resolveFSGroup || resolveSupplementalGroups

	if requiresNamespaceAllocations {
		// Ensure we have the namespace
		namespace, err = getNamespaceByName(ns, namespace, client)
		if err != nil {
			return nil, namespace, fmt.Errorf("error fetching namespace %s required to preallocate values for %s: %v", ns, constraint.Name, err)
		}
	}

	// Make a copy of the constraint so we don't mutate the store's cache
	var constraintCopy securityapi.SecurityContextConstraints = *constraint
	constraint = &constraintCopy

	// Resolve the values from the namespace
	if resolveUIDRange {
		constraint.RunAsUser.UIDRangeMin, constraint.RunAsUser.UIDRangeMax, err = getPreallocatedUIDRange(namespace)
		if err != nil {
			return nil, namespace, fmt.Errorf("unable to find pre-allocated uid annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err)
		}
	}
	if resolveSELinuxLevel {
		var level string
		if level, err = getPreallocatedLevel(namespace); err != nil {
			return nil, namespace, fmt.Errorf("unable to find pre-allocated mcs annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err)
		}

		// SELinuxOptions is a pointer, if we are resolving and it is already initialized
		// we need to make a copy of it so we don't manipulate the store's cache.
		if constraint.SELinuxContext.SELinuxOptions != nil {
			var seLinuxOptionsCopy kapi.SELinuxOptions = *constraint.SELinuxContext.SELinuxOptions
			constraint.SELinuxContext.SELinuxOptions = &seLinuxOptionsCopy
		} else {
			constraint.SELinuxContext.SELinuxOptions = &kapi.SELinuxOptions{}
		}
		constraint.SELinuxContext.SELinuxOptions.Level = level
	}
	if resolveFSGroup {
		fsGroup, err := getPreallocatedFSGroup(namespace)
		if err != nil {
			return nil, namespace, fmt.Errorf("unable to find pre-allocated group annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err)
		}
		constraint.FSGroup.Ranges = fsGroup
	}
	if resolveSupplementalGroups {
		supplementalGroups, err := getPreallocatedSupplementalGroups(namespace)
		if err != nil {
			return nil, namespace, fmt.Errorf("unable to find pre-allocated group annotation for namespace %s while trying to configure SCC %s: %v", namespace.Name, constraint.Name, err)
		}
		constraint.SupplementalGroups.Ranges = supplementalGroups
	}

	// Create the provider
	provider, err := NewSimpleProvider(constraint)
	if err != nil {
		return nil, namespace, fmt.Errorf("error creating provider for SCC %s in namespace %s: %v", constraint.Name, ns, err)
	}
	return provider, namespace, nil
}

// getPreallocatedUIDRange retrieves the annotated value from the namespace, splits it to make
// the min/max and formats the data into the necessary types for the strategy options.
func getPreallocatedUIDRange(ns *kapi.Namespace) (*int64, *int64, error) {
	annotationVal, ok := ns.Annotations[allocator.UIDRangeAnnotation]
	if !ok {
		return nil, nil, fmt.Errorf("unable to find annotation %s", allocator.UIDRangeAnnotation)
	}
	if len(annotationVal) == 0 {
		return nil, nil, fmt.Errorf("found annotation %s but it was empty", allocator.UIDRangeAnnotation)
	}
	uidBlock, err := uid.ParseBlock(annotationVal)
	if err != nil {
		return nil, nil, err
	}

	var min int64 = int64(uidBlock.Start)
	var max int64 = int64(uidBlock.End)
	glog.V(4).Infof("got preallocated values for min: %d, max: %d for uid range in namespace %s", min, max, ns.Name)
	return &min, &max, nil
}

// getPreallocatedLevel gets the annotated value from the namespace.
func getPreallocatedLevel(ns *kapi.Namespace) (string, error) {
	level, ok := ns.Annotations[allocator.MCSAnnotation]
	if !ok {
		return "", fmt.Errorf("unable to find annotation %s", allocator.MCSAnnotation)
	}
	if len(level) == 0 {
		return "", fmt.Errorf("found annotation %s but it was empty", allocator.MCSAnnotation)
	}
	glog.V(4).Infof("got preallocated value for level: %s for selinux options in namespace %s", level, ns.Name)
	return level, nil
}

// getSupplementalGroupsAnnotation provides a backwards compatible way to get supplemental groups
// annotations from a namespace by looking for SupplementalGroupsAnnotation and falling back to
// UIDRangeAnnotation if it is not found.
func getSupplementalGroupsAnnotation(ns *kapi.Namespace) (string, error) {
	groups, ok := ns.Annotations[allocator.SupplementalGroupsAnnotation]
	if !ok {
		glog.V(4).Infof("unable to find supplemental group annotation %s falling back to %s", allocator.SupplementalGroupsAnnotation, allocator.UIDRangeAnnotation)

		groups, ok = ns.Annotations[allocator.UIDRangeAnnotation]
		if !ok {
			return "", fmt.Errorf("unable to find supplemental group or uid annotation for namespace %s", ns.Name)
		}
	}

	if len(groups) == 0 {
		return "", fmt.Errorf("unable to find groups using %s and %s annotations", allocator.SupplementalGroupsAnnotation, allocator.UIDRangeAnnotation)
	}
	return groups, nil
}

// getPreallocatedFSGroup gets the annotated value from the namespace.
func getPreallocatedFSGroup(ns *kapi.Namespace) ([]securityapi.IDRange, error) {
	groups, err := getSupplementalGroupsAnnotation(ns)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("got preallocated value for groups: %s in namespace %s", groups, ns.Name)

	blocks, err := parseSupplementalGroupAnnotation(groups)
	if err != nil {
		return nil, err
	}
	return []securityapi.IDRange{
		{
			Min: int64(blocks[0].Start),
			Max: int64(blocks[0].Start),
		},
	}, nil
}

// getPreallocatedSupplementalGroups gets the annotated value from the namespace.
func getPreallocatedSupplementalGroups(ns *kapi.Namespace) ([]securityapi.IDRange, error) {
	groups, err := getSupplementalGroupsAnnotation(ns)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("got preallocated value for groups: %s in namespace %s", groups, ns.Name)

	blocks, err := parseSupplementalGroupAnnotation(groups)
	if err != nil {
		return nil, err
	}

	idRanges := []securityapi.IDRange{}
	for _, block := range blocks {
		rng := securityapi.IDRange{
			Min: int64(block.Start),
			Max: int64(block.End),
		}
		idRanges = append(idRanges, rng)
	}
	return idRanges, nil
}

// parseSupplementalGroupAnnotation parses the group annotation into blocks.
func parseSupplementalGroupAnnotation(groups string) ([]uid.Block, error) {
	blocks := []uid.Block{}
	segments := strings.Split(groups, ",")
	for _, segment := range segments {
		block, err := uid.ParseBlock(segment)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no blocks parsed from annotation %s", groups)
	}
	return blocks, nil
}

// requiresPreAllocatedUIDRange returns true if the strategy is must run in range and the min or max
// is not set.
func requiresPreAllocatedUIDRange(constraint *securityapi.SecurityContextConstraints) bool {
	if constraint.RunAsUser.Type != securityapi.RunAsUserStrategyMustRunAsRange {
		return false
	}
	return constraint.RunAsUser.UIDRangeMin == nil && constraint.RunAsUser.UIDRangeMax == nil
}

// requiresPreAllocatedSELinuxLevel returns true if the strategy is must run as and the level is not set.
func requiresPreAllocatedSELinuxLevel(constraint *securityapi.SecurityContextConstraints) bool {
	if constraint.SELinuxContext.Type != securityapi.SELinuxStrategyMustRunAs {
		return false
	}
	if constraint.SELinuxContext.SELinuxOptions == nil {
		return true
	}
	return constraint.SELinuxContext.SELinuxOptions.Level == ""
}

// requiresPreAllocatedSELinuxLevel returns true if the strategy is must run as and there is no
// range specified.
func requiresPreallocatedSupplementalGroups(constraint *securityapi.SecurityContextConstraints) bool {
	if constraint.SupplementalGroups.Type != securityapi.SupplementalGroupsStrategyMustRunAs {
		return false
	}
	return len(constraint.SupplementalGroups.Ranges) == 0
}

// requiresPreallocatedFSGroup returns true if the strategy is must run as and there is no
// range specified.
func requiresPreallocatedFSGroup(constraint *securityapi.SecurityContextConstraints) bool {
	if constraint.FSGroup.Type != securityapi.FSGroupStrategyMustRunAs {
		return false
	}
	return len(constraint.FSGroup.Ranges) == 0
}
