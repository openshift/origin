package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	kapi "k8s.io/kubernetes/pkg/api"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kextensionsclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/extensions/v1beta1"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deployclient "github.com/openshift/origin/pkg/deploy/generated/internalclientset/typed/apps/internalversion"
	unidlingapi "github.com/openshift/origin/pkg/unidling/api"
	utilunidling "github.com/openshift/origin/pkg/unidling/util"
	utilerrors "github.com/openshift/origin/pkg/util/errors"
)

var (
	idleLong = templates.LongDesc(`
		Idle scalable resources

		Idling discovers the scalable resources (such as deployment configs and replication controllers)
		associated with a series of services by examining the endpoints of the service.
		Each service is then marked as idled, the associated resources are recorded, and the resources
		are scaled down to zero replicas.

		Upon receiving network traffic, the services (and any associated routes) will "wake up" the
		associated resources by scaling them back up to their previous scale.`)

	idleExample = templates.Examples(`
		# Idle the scalable controllers associated with the services listed in to-idle.txt
	  $ %[1]s idle --resource-names-file to-idle.txt`)
)

// NewCmdIdle implements the OpenShift cli idle command
func NewCmdIdle(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	o := &IdleOptions{
		out:         out,
		errOut:      errOut,
		cmdFullName: fullName,
	}

	cmd := &cobra.Command{
		Use:     "idle (SERVICE_ENDPOINTS... | -l label | --all | --resource-names-file FILENAME)",
		Short:   "Idle scalable resources",
		Long:    idleLong,
		Example: fmt.Sprintf(idleExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			err := o.RunIdle(f)
			if err == cmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "If true, only print the annotations that would be written, without annotating or idling the relevant objects")
	cmd.Flags().StringVar(&o.filename, "resource-names-file", o.filename, "file containing list of services whose scalable resources to idle")
	cmd.Flags().StringVarP(&o.selector, "selector", "l", o.selector, "Selector (label query) to use to select services")
	cmd.Flags().BoolVar(&o.all, "all", o.all, "if true, select all services in the namespace")
	cmd.Flags().BoolVar(&o.allNamespaces, "all-namespaces", o.allNamespaces, "if true, select services across all namespaces")
	cmd.MarkFlagFilename("resource-names-file")

	// TODO: take the `-o name` argument, and only print out names instead of the summary

	return cmd
}

type IdleOptions struct {
	out, errOut io.Writer

	dryRun bool

	filename      string
	all           bool
	selector      string
	allNamespaces bool
	resources     string

	cmdFullName string

	nowTime    time.Time
	svcBuilder *resource.Builder
}

func (o *IdleOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.nowTime = time.Now().UTC()

	// NB: our filename arg is different from usual, since it's just a list of service names
	if o.filename != "" && (o.selector != "" || len(args) > 0 || o.all) {
		return fmt.Errorf("resource names, selectors, and the all flag may not be be specified if a filename is specified")
	}

	mapper, typer := f.Object()
	o.svcBuilder = resource.NewBuilder(mapper, f.CategoryExpander(), typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace().AllNamespaces(o.allNamespaces).
		Flatten().
		SingleResourceType()

	if o.filename != "" {
		targetServiceNames, err := scanLinesFromFile(o.filename)
		if err != nil {
			return err
		}
		o.svcBuilder.ResourceNames("endpoints", targetServiceNames...)
	} else {
		// NB: this is a bit weird because the resource builder will complain if we use ResourceTypes and ResourceNames when len(args) > 0
		if o.selector != "" {
			o.svcBuilder.SelectorParam(o.selector).ResourceTypes("endpoints")
		}

		o.svcBuilder.ResourceNames("endpoints", args...)

		if o.all {
			o.svcBuilder.ResourceTypes("endpoints").SelectAllParam(o.all)
		}
	}

	return nil
}

// scanLinesFromFile loads lines from either standard in or a file
func scanLinesFromFile(filename string) ([]string, error) {
	var targetsInput io.Reader
	if filename == "-" {
		targetsInput = os.Stdin
	} else if filename == "" {
		return nil, fmt.Errorf("you must specify an list of resources to idle")
	} else {
		inputFile, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer inputFile.Close()
		targetsInput = inputFile
	}

	lines := []string{}

	// grab the raw resources from the file
	lineScanner := bufio.NewScanner(targetsInput)
	for lineScanner.Scan() {
		line := lineScanner.Text()
		if line == "" {
			// skip empty lines
			continue
		}
		lines = append(lines, line)
	}
	if err := lineScanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// idleUpdateInfo contains the required info to annotate an endpoints object
// with the scalable resources that it should unidle
type idleUpdateInfo struct {
	obj       *kapi.Endpoints
	scaleRefs map[unidlingapi.CrossGroupObjectReference]struct{}
}

// calculateIdlableAnnotationsByService calculates the list of objects involved in the idling process from a list of services in a file.
// Using the list of services, it figures out the associated scalable objects, and returns a map from the endpoints object for the services to
// the list of scalable resources associated with that endpoints object, as well as a map from CrossGroupObjectReferences to scale to 0 to the
// name of the associated service.
func (o *IdleOptions) calculateIdlableAnnotationsByService(f *clientcmd.Factory) (map[types.NamespacedName]idleUpdateInfo, map[unidlingapi.CrossGroupObjectReference]types.NamespacedName, error) {
	// load our set of services
	client, err := f.ClientSet()
	if err != nil {
		return nil, nil, err
	}

	mapper, _ := f.Object()

	podsLoaded := make(map[kapi.ObjectReference]*kapi.Pod)
	getPod := func(ref kapi.ObjectReference) (*kapi.Pod, error) {
		if pod, ok := podsLoaded[ref]; ok {
			return pod, nil
		}
		pod, err := client.Core().Pods(ref.Namespace).Get(ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		podsLoaded[ref] = pod

		return pod, nil
	}

	controllersLoaded := make(map[kapi.ObjectReference]runtime.Object)
	helpers := make(map[schema.GroupKind]*resource.Helper)
	getController := func(ref kapi.ObjectReference) (runtime.Object, error) {
		if controller, ok := controllersLoaded[ref]; ok {
			return controller, nil
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		// just get the unversioned version of this
		gk := schema.GroupKind{Group: gv.Group, Kind: ref.Kind}
		helper, ok := helpers[gk]
		if !ok {
			var mapping *meta.RESTMapping
			mapping, err = mapper.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: ref.Kind}, "")
			if err != nil {
				return nil, err
			}
			var client resource.RESTClient
			client, err = f.ClientForMapping(mapping)
			if err != nil {
				return nil, err
			}
			helper = resource.NewHelper(client, mapping)
			helpers[gk] = helper
		}

		var controller runtime.Object
		controller, err = helper.Get(ref.Namespace, ref.Name, false)
		if err != nil {
			return nil, err
		}

		controllersLoaded[ref] = controller

		return controller, nil
	}

	targetScaleRefs := make(map[unidlingapi.CrossGroupObjectReference]types.NamespacedName)
	endpointsInfo := make(map[types.NamespacedName]idleUpdateInfo)

	decoder := f.Decoder(true)
	err = o.svcBuilder.Do().Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		endpoints, isEndpoints := info.Object.(*kapi.Endpoints)
		if !isEndpoints {
			return fmt.Errorf("you must specify endpoints, not %v (view available endpoints with \"%s get endpoints\").", info.Mapping.Resource, o.cmdFullName)
		}

		endpointsName := types.NamespacedName{
			Namespace: endpoints.Namespace,
			Name:      endpoints.Name,
		}
		scaleRefs, err := findScalableResourcesForEndpoints(endpoints, decoder, getPod, getController)
		if err != nil {
			return fmt.Errorf("unable to calculate scalable resources for service %s/%s: %v", endpoints.Namespace, endpoints.Name, err)
		}

		for ref := range scaleRefs {
			targetScaleRefs[ref] = endpointsName
		}

		idleInfo := idleUpdateInfo{
			obj:       endpoints,
			scaleRefs: scaleRefs,
		}

		endpointsInfo[endpointsName] = idleInfo

		return nil
	})

	return endpointsInfo, targetScaleRefs, err
}

// getControllerRef returns a subresource reference to the owning controller of the given object.
// It will use both the CreatedByAnnotation from Kubernetes, as well as the DeploymentConfigAnnotation
// from Origin to look this up.  If neither are found, it will return nil.
func getControllerRef(obj runtime.Object, decoder runtime.Decoder) (*kapi.ObjectReference, error) {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	annotations := objMeta.GetAnnotations()

	creatorRefRaw, creatorListed := annotations[kapi.CreatedByAnnotation]
	if !creatorListed {
		// if we don't have a creator listed, try the openshift-specific Deployment annotation
		dcName, dcNameListed := annotations[deployapi.DeploymentConfigAnnotation]
		if !dcNameListed {
			return nil, nil
		}

		return &kapi.ObjectReference{
			Name:      dcName,
			Namespace: objMeta.GetNamespace(),
			Kind:      "DeploymentConfig",
		}, nil
	}

	serializedRef := &kapi.SerializedReference{}
	if err := runtime.DecodeInto(decoder, []byte(creatorRefRaw), serializedRef); err != nil {
		return nil, fmt.Errorf("could not decoded pod's creator reference: %v", err)
	}

	return &serializedRef.Reference, nil
}

func makeCrossGroupObjRef(ref *kapi.ObjectReference) (unidlingapi.CrossGroupObjectReference, error) {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return unidlingapi.CrossGroupObjectReference{}, err
	}

	return unidlingapi.CrossGroupObjectReference{
		Kind:  ref.Kind,
		Name:  ref.Name,
		Group: gv.Group,
	}, nil
}

// findScalableResourcesForEndpoints takes an Endpoints object and looks for the associated
// scalable objects by checking each address in each subset to see if it has a pod
// reference, and the following that pod reference to find the owning controller,
// and returning the unique set of controllers found this way.
func findScalableResourcesForEndpoints(endpoints *kapi.Endpoints, decoder runtime.Decoder, getPod func(kapi.ObjectReference) (*kapi.Pod, error), getController func(kapi.ObjectReference) (runtime.Object, error)) (map[unidlingapi.CrossGroupObjectReference]struct{}, error) {
	// To find all RCs and DCs for an endpoint, we first figure out which pods are pointed to by that endpoint...
	podRefs := map[kapi.ObjectReference]*kapi.Pod{}
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
				pod, err := getPod(*addr.TargetRef)
				if utilerrors.TolerateNotFoundError(err) != nil {
					return nil, fmt.Errorf("unable to find controller for pod %s/%s: %v", addr.TargetRef.Namespace, addr.TargetRef.Name, err)
				}

				if pod != nil {
					podRefs[*addr.TargetRef] = pod
				}
			}
		}
	}

	// ... then, for each pod, we check the controller, and find the set of unique controllers...
	immediateControllerRefs := make(map[kapi.ObjectReference]struct{})
	for _, pod := range podRefs {
		controllerRef, err := getControllerRef(pod, decoder)
		if err != nil {
			return nil, fmt.Errorf("unable to find controller for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		} else if controllerRef == nil {
			return nil, fmt.Errorf("unable to find controller for pod %s/%s: no creator reference listed", pod.Namespace, pod.Name)
		}

		immediateControllerRefs[*controllerRef] = struct{}{}
	}

	// ... finally, for each controller, we load it, and see if there is a corresponding owner (to cover cases like DCs, Deployments, etc)
	controllerRefs := make(map[unidlingapi.CrossGroupObjectReference]struct{})
	for controllerRef := range immediateControllerRefs {
		controller, err := getController(controllerRef)
		if utilerrors.TolerateNotFoundError(err) != nil {
			return nil, fmt.Errorf("unable to load %s %q: %v", controllerRef.Kind, controllerRef.Name, err)
		}

		if controller != nil {
			var parentControllerRef *kapi.ObjectReference
			parentControllerRef, err = getControllerRef(controller, decoder)
			if err != nil {
				return nil, fmt.Errorf("unable to load the creator of %s %q: %v", controllerRef.Kind, controllerRef.Name, err)
			}

			var crossGroupObjRef unidlingapi.CrossGroupObjectReference
			if parentControllerRef == nil {
				// if this is just a plain RC, use it
				crossGroupObjRef, err = makeCrossGroupObjRef(&controllerRef)
			} else {
				crossGroupObjRef, err = makeCrossGroupObjRef(parentControllerRef)
			}

			if err != nil {
				return nil, fmt.Errorf("unable to load the creator of %s %q: %v", controllerRef.Kind, controllerRef.Name, err)
			}
			controllerRefs[crossGroupObjRef] = struct{}{}
		}
	}

	return controllerRefs, nil
}

// pairScalesWithScaleRefs takes some subresource references, a map of new scales for those subresource references,
// and annotations from an existing object.  It merges the scales and references found in the existing annotations
// with the new data (using the new scale in case of conflict if present and not 0, and the old scale otherwise),
// and returns a slice of RecordedScaleReferences suitable for using as the new annotation value.
func pairScalesWithScaleRefs(serviceName types.NamespacedName, annotations map[string]string, rawScaleRefs map[unidlingapi.CrossGroupObjectReference]struct{}, scales map[unidlingapi.CrossGroupObjectReference]int32) ([]unidlingapi.RecordedScaleReference, error) {
	oldTargetsRaw, hasOldTargets := annotations[unidlingapi.UnidleTargetAnnotation]

	scaleRefs := make([]unidlingapi.RecordedScaleReference, 0, len(rawScaleRefs))

	// initialize the list of new annotations
	for rawScaleRef := range rawScaleRefs {
		scaleRefs = append(scaleRefs, unidlingapi.RecordedScaleReference{
			CrossGroupObjectReference: rawScaleRef,
			Replicas:                  0,
		})
	}

	// if the new preserved scale would be 0, see if we have an old scale that we can use instead
	if hasOldTargets {
		var oldTargets []unidlingapi.RecordedScaleReference
		oldTargetsSet := make(map[unidlingapi.CrossGroupObjectReference]int)
		if err := json.Unmarshal([]byte(oldTargetsRaw), &oldTargets); err != nil {
			return nil, fmt.Errorf("unable to extract existing scale information from endpoints %s: %v", serviceName.String(), err)
		}

		for i, target := range oldTargets {
			oldTargetsSet[target.CrossGroupObjectReference] = i
		}

		// figure out which new targets were already present...
		for _, newScaleRef := range scaleRefs {
			if oldTargetInd, ok := oldTargetsSet[newScaleRef.CrossGroupObjectReference]; ok {
				if newScale, ok := scales[newScaleRef.CrossGroupObjectReference]; !ok || newScale == 0 {
					scales[newScaleRef.CrossGroupObjectReference] = oldTargets[oldTargetInd].Replicas
				}
				delete(oldTargetsSet, newScaleRef.CrossGroupObjectReference)
			}
		}

		// ...and add in any existing targets not already on the new list to the new list
		for _, ind := range oldTargetsSet {
			scaleRefs = append(scaleRefs, oldTargets[ind])
		}
	}

	for i := range scaleRefs {
		scaleRef := &scaleRefs[i]
		newScale, ok := scales[scaleRef.CrossGroupObjectReference]
		if !ok || newScale == 0 {
			newScale = 1
			if scaleRef.Replicas != 0 {
				newScale = scaleRef.Replicas
			}
		}

		scaleRef.Replicas = newScale
	}

	return scaleRefs, nil
}

// setIdleAnnotations sets the given annotation on the given object to the marshaled list of CrossGroupObjectReferences
func setIdleAnnotations(serviceName types.NamespacedName, annotations map[string]string, scaleRefs []unidlingapi.RecordedScaleReference, nowTime time.Time) error {
	var scaleRefsBytes []byte
	var err error
	if scaleRefsBytes, err = json.Marshal(scaleRefs); err != nil {
		return err
	}

	annotations[unidlingapi.UnidleTargetAnnotation] = string(scaleRefsBytes)
	annotations[unidlingapi.IdledAtAnnotation] = nowTime.Format(time.RFC3339)

	return nil
}

// patchObj patches calculates a patch between the given new object and the existing marshaled object
func patchObj(obj runtime.Object, metadata metav1.Object, oldData []byte, mapping *meta.RESTMapping, f *clientcmd.Factory) (runtime.Object, error) {
	versionedObj, err := mapping.ObjectConvertor.ConvertToVersion(obj, schema.GroupVersions{mapping.GroupVersionKind.GroupVersion()})
	if err != nil {
		return nil, err
	}
	newData, err := json.Marshal(versionedObj)
	if err != nil {
		return nil, err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, versionedObj)
	if err != nil {
		return nil, err
	}

	client, err := f.ClientForMapping(mapping)
	if err != nil {
		return nil, err
	}
	helper := resource.NewHelper(client, mapping)

	return helper.Patch(metadata.GetNamespace(), metadata.GetName(), types.StrategicMergePatchType, patchBytes)
}

type scaleInfo struct {
	namespace string
	scale     *extensions.Scale
	obj       runtime.Object
}

// RunIdle runs the idling command logic, taking a list of resources or services in a file, scaling the associated
// scalable resources to zero, and annotating the associated endpoints objects with the scalable resources to unidle
// when they receive traffic.
func (o *IdleOptions) RunIdle(f *clientcmd.Factory) error {
	hadError := false
	nowTime := time.Now().UTC()

	dryRunText := ""
	if o.dryRun {
		dryRunText = "(dry run)"
	}

	// figure out which endpoints and resources we need to idle
	byService, byScalable, err := o.calculateIdlableAnnotationsByService(f)

	if err != nil {
		if len(byService) == 0 || len(byScalable) == 0 {
			return fmt.Errorf("no valid scalable resources found to idle: %v", err)
		}
		fmt.Fprintf(o.errOut, "warning: continuing on for valid scalable resources, but an error occurred while finding scalable resources to idle: %v", err)
	}

	oclient, kclient, err := f.Clients()
	if err != nil {
		return err
	}

	externalKubeExtensionClient := kextensionsclient.New(kclient.Core().RESTClient())
	delegScaleGetter := osclient.NewDelegatingScaleNamespacer(oclient, externalKubeExtensionClient)
	dcGetter := deployclient.New(oclient.RESTClient)

	scaleAnnotater := utilunidling.NewScaleAnnotater(delegScaleGetter, dcGetter, kclient.Core(), func(currentReplicas int32, annotations map[string]string) {
		annotations[unidlingapi.IdledAtAnnotation] = nowTime.UTC().Format(time.RFC3339)
		annotations[unidlingapi.PreviousScaleAnnotation] = fmt.Sprintf("%v", currentReplicas)
	})

	replicas := make(map[unidlingapi.CrossGroupObjectReference]int32, len(byScalable))
	toScale := make(map[unidlingapi.CrossGroupObjectReference]scaleInfo)

	mapper, typer := f.Object()

	// first, collect the scale info
	for scaleRef, svcName := range byScalable {
		obj, scale, err := scaleAnnotater.GetObjectWithScale(svcName.Namespace, scaleRef)
		if err != nil {
			fmt.Fprintf(o.errOut, "error: unable to get scale for %s %s/%s, not marking that scalable as idled: %v\n", scaleRef.Kind, svcName.Namespace, scaleRef.Name, err)
			svcInfo := byService[svcName]
			delete(svcInfo.scaleRefs, scaleRef)
			hadError = true
			continue
		}
		replicas[scaleRef] = scale.Spec.Replicas
		toScale[scaleRef] = scaleInfo{scale: scale, obj: obj, namespace: svcName.Namespace}
	}

	// annotate the endpoints objects to indicate which scalable resources need to be unidled on traffic
	for serviceName, info := range byService {
		if info.obj.Annotations == nil {
			info.obj.Annotations = make(map[string]string)
		}
		refsWithScale, err := pairScalesWithScaleRefs(serviceName, info.obj.Annotations, info.scaleRefs, replicas)
		if err != nil {
			fmt.Fprintf(o.errOut, "error: unable to mark service %q as idled: %v", serviceName.String(), err)
			continue
		}

		if !o.dryRun {
			if len(info.scaleRefs) == 0 {
				fmt.Fprintf(o.errOut, "error: unable to mark the service %q as idled.\n", serviceName.String())
				fmt.Fprintf(o.errOut, "Make sure that the service is not already marked as idled and that it is associated with resources that can be scaled.\n")
				fmt.Fprintf(o.errOut, "See 'oc idle -h' for help and examples.\n")
				hadError = true
				continue
			}

			metadata, err := meta.Accessor(info.obj)
			if err != nil {
				fmt.Fprintf(o.errOut, "error: unable to mark service %q as idled: %v", serviceName.String(), err)
				hadError = true
				continue
			}
			gvks, _, err := typer.ObjectKinds(info.obj)
			if err != nil {
				fmt.Fprintf(o.errOut, "error: unable to mark service %q as idled: %v", serviceName.String(), err)
				hadError = true
				continue
			}
			// we need a versioned obj to properly marshal to JSON, so that we can compute the patch
			mapping, err := mapper.RESTMapping(gvks[0].GroupKind(), gvks[0].Version)
			if err != nil {
				fmt.Fprintf(o.errOut, "error: unable to mark service %q as idled: %v", serviceName.String(), err)
				hadError = true
				continue
			}

			versionedObj, err := mapping.ObjectConvertor.ConvertToVersion(info.obj, schema.GroupVersions{gvks[0].GroupVersion()})
			if err != nil {
				fmt.Fprintf(o.errOut, "error: unable to mark service %q as idled: %v", serviceName.String(), err)
				hadError = true
				continue
			}

			oldData, err := json.Marshal(versionedObj)
			if err != nil {
				fmt.Fprintf(o.errOut, "error: unable to mark service %q as idled: %v", serviceName.String(), err)
				hadError = true
				continue
			}

			if err = setIdleAnnotations(serviceName, info.obj.Annotations, refsWithScale, nowTime); err != nil {
				fmt.Fprintf(o.errOut, "error: unable to mark service %q as idled: %v", serviceName.String(), err)
				hadError = true
				continue
			}
			if _, err := patchObj(info.obj, metadata, oldData, mapping, f); err != nil {
				fmt.Fprintf(o.errOut, "error: unable to mark service %q as idled: %v", serviceName.String(), err)
				hadError = true
				continue
			}
		}

		fmt.Fprintf(o.out, "The service %q has been marked as idled %s\n", serviceName.String(), dryRunText)

		for _, scaleRef := range refsWithScale {
			fmt.Fprintf(o.out, "The service will unidle %s \"%s/%s\" to %v replicas once it receives traffic %s\n", scaleRef.Kind, serviceName.Namespace, scaleRef.Name, scaleRef.Replicas, dryRunText)
		}
	}

	// actually "idle" the scalable resources by scaling them down to zero
	// (scale down to zero *after* we've applied the annotation so that we don't miss any traffic)
	for scaleRef, info := range toScale {
		if !o.dryRun {
			info.scale.Spec.Replicas = 0
			scaleUpdater := utilunidling.NewScaleUpdater(f.JSONEncoder(), info.namespace, dcGetter, kclient.Core())
			if err := scaleAnnotater.UpdateObjectScale(scaleUpdater, info.namespace, scaleRef, info.obj, info.scale); err != nil {
				fmt.Fprintf(o.errOut, "error: unable to scale %s %s/%s to 0, but still listed as target for unidling: %v\n", scaleRef.Kind, info.namespace, scaleRef.Name, err)
				hadError = true
				continue
			}
		}

		fmt.Fprintf(o.out, "%s \"%s/%s\" has been idled %s\n", scaleRef.Kind, info.namespace, scaleRef.Name, dryRunText)
	}

	if hadError {
		return cmdutil.ErrExit
	}

	return nil
}
