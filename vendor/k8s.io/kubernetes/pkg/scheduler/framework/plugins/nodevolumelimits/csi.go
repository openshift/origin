/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodevolumelimits

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	ephemeral "k8s.io/component-helpers/storage/ephemeral"
	storagehelpers "k8s.io/component-helpers/storage/volume"
	csitrans "k8s.io/csi-translation-lib"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/feature"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/names"
	"k8s.io/kubernetes/pkg/scheduler/util"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
)

// InTreeToCSITranslator contains methods required to check migratable status
// and perform translations from InTree PV's to CSI
type InTreeToCSITranslator interface {
	IsPVMigratable(pv *v1.PersistentVolume) bool
	IsInlineMigratable(vol *v1.Volume) bool
	IsMigratableIntreePluginByName(inTreePluginName string) bool
	GetInTreePluginNameFromSpec(pv *v1.PersistentVolume, vol *v1.Volume) (string, error)
	GetCSINameFromInTreeName(pluginName string) (string, error)
	TranslateInTreePVToCSI(pv *v1.PersistentVolume) (*v1.PersistentVolume, error)
	TranslateInTreeInlineVolumeToCSI(volume *v1.Volume, podNamespace string) (*v1.PersistentVolume, error)
}

// CSILimits is a plugin that checks node volume limits.
type CSILimits struct {
	csiNodeLister storagelisters.CSINodeLister
	pvLister      corelisters.PersistentVolumeLister
	pvcLister     corelisters.PersistentVolumeClaimLister
	scLister      storagelisters.StorageClassLister

	randomVolumeIDPrefix string

	translator InTreeToCSITranslator
}

var _ framework.PreFilterPlugin = &CSILimits{}
var _ framework.FilterPlugin = &CSILimits{}
var _ framework.EnqueueExtensions = &CSILimits{}

// CSIName is the name of the plugin used in the plugin registry and configurations.
const CSIName = names.NodeVolumeLimits

// Name returns name of the plugin. It is used in logs, etc.
func (pl *CSILimits) Name() string {
	return CSIName
}

// EventsToRegister returns the possible events that may make a Pod.
// failed by this plugin schedulable.
func (pl *CSILimits) EventsToRegister(_ context.Context) ([]framework.ClusterEventWithHint, error) {
	return []framework.ClusterEventWithHint{
		// We don't register any `QueueingHintFn` intentionally
		// because any new CSINode could make pods that were rejected by CSI volumes schedulable.
		{Event: framework.ClusterEvent{Resource: framework.CSINode, ActionType: framework.Add}},
		{Event: framework.ClusterEvent{Resource: framework.Pod, ActionType: framework.Delete}, QueueingHintFn: pl.isSchedulableAfterPodDeleted},
		{Event: framework.ClusterEvent{Resource: framework.PersistentVolumeClaim, ActionType: framework.Add}},
	}, nil
}

func (pl *CSILimits) isSchedulableAfterPodDeleted(logger klog.Logger, pod *v1.Pod, oldObj, newObj interface{}) (framework.QueueingHint, error) {
	deletedPod, _, err := util.As[*v1.Pod](oldObj, newObj)
	if err != nil {
		return framework.Queue, fmt.Errorf("unexpected objects in isSchedulableAfterPodDeleted: %w", err)
	}

	if len(deletedPod.Spec.Volumes) == 0 {
		return framework.QueueSkip, nil
	}

	if deletedPod.Spec.NodeName == "" {
		return framework.QueueSkip, nil
	}

	for _, vol := range deletedPod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil || vol.Ephemeral != nil || pl.translator.IsInlineMigratable(&vol) {
			return framework.Queue, nil
		}
	}

	logger.V(5).Info("The deleted pod does not impact the scheduling of the unscheduled pod", "deletedPod", klog.KObj(pod), "pod", klog.KObj(deletedPod))
	return framework.QueueSkip, nil
}

// PreFilter invoked at the prefilter extension point
//
// If the pod haven't those types of volumes, we'll skip the Filter phase
func (pl *CSILimits) PreFilter(ctx context.Context, _ *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	volumes := pod.Spec.Volumes
	for i := range volumes {
		vol := &volumes[i]
		if vol.PersistentVolumeClaim != nil || vol.Ephemeral != nil || pl.translator.IsInlineMigratable(vol) {
			return nil, nil
		}
	}

	return nil, framework.NewStatus(framework.Skip)
}

// PreFilterExtensions returns prefilter extensions, pod add and remove.
func (pl *CSILimits) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Filter invoked at the filter extension point.
func (pl *CSILimits) Filter(ctx context.Context, _ *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	// If the new pod doesn't have any volume attached to it, the predicate will always be true
	if len(pod.Spec.Volumes) == 0 {
		return nil
	}

	node := nodeInfo.Node()

	logger := klog.FromContext(ctx)

	// If CSINode doesn't exist, the predicate may read the limits from Node object
	csiNode, err := pl.csiNodeLister.Get(node.Name)
	if err != nil {
		// TODO: return the error once CSINode is created by default (2 releases)
		logger.V(5).Info("Could not get a CSINode object for the node", "node", klog.KObj(node), "err", err)
	}

	newVolumes := make(map[string]string)
	if err := pl.filterAttachableVolumes(logger, pod, csiNode, true /* new pod */, newVolumes); err != nil {
		if apierrors.IsNotFound(err) {
			// PVC is not found. This Pod will never be schedulable until PVC is created.
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
		}
		return framework.AsStatus(err)
	}

	// If the pod doesn't have any new CSI volumes, the predicate will always be true
	if len(newVolumes) == 0 {
		return nil
	}

	// If the node doesn't have volume limits, the predicate will always be true
	nodeVolumeLimits := getVolumeLimits(nodeInfo, csiNode)
	if len(nodeVolumeLimits) == 0 {
		return nil
	}

	attachedVolumes := make(map[string]string)
	for _, existingPod := range nodeInfo.Pods {
		if err := pl.filterAttachableVolumes(logger, existingPod.Pod, csiNode, false /* existing pod */, attachedVolumes); err != nil {
			return framework.AsStatus(err)
		}
	}

	attachedVolumeCount := map[string]int{}
	for volumeUniqueName, volumeLimitKey := range attachedVolumes {
		// Don't count single volume used in multiple pods more than once
		delete(newVolumes, volumeUniqueName)
		attachedVolumeCount[volumeLimitKey]++
	}

	newVolumeCount := map[string]int{}
	for _, volumeLimitKey := range newVolumes {
		newVolumeCount[volumeLimitKey]++
	}

	for volumeLimitKey, count := range newVolumeCount {
		maxVolumeLimit, ok := nodeVolumeLimits[v1.ResourceName(volumeLimitKey)]
		if ok {
			currentVolumeCount := attachedVolumeCount[volumeLimitKey]
			logger.V(5).Info("Found plugin volume limits", "node", node.Name, "volumeLimitKey", volumeLimitKey,
				"maxLimits", maxVolumeLimit, "currentVolumeCount", currentVolumeCount, "newVolumeCount", count,
				"pod", klog.KObj(pod))
			if currentVolumeCount+count > int(maxVolumeLimit) {
				return framework.NewStatus(framework.Unschedulable, ErrReasonMaxVolumeCountExceeded)
			}
		}
	}

	return nil
}

func (pl *CSILimits) filterAttachableVolumes(
	logger klog.Logger, pod *v1.Pod, csiNode *storagev1.CSINode, newPod bool, result map[string]string) error {
	for _, vol := range pod.Spec.Volumes {
		pvcName := ""
		isEphemeral := false
		switch {
		case vol.PersistentVolumeClaim != nil:
			// Normal CSI volume can only be used through PVC
			pvcName = vol.PersistentVolumeClaim.ClaimName
		case vol.Ephemeral != nil:
			// Generic ephemeral inline volumes also use a PVC,
			// just with a computed name and certain ownership.
			// That is checked below once the pvc object is
			// retrieved.
			pvcName = ephemeral.VolumeClaimName(pod, &vol)
			isEphemeral = true
		default:
			// Inline Volume does not have PVC.
			// Need to check if CSI migration is enabled for this inline volume.
			// - If the volume is migratable and CSI migration is enabled, need to count it
			// as well.
			// - If the volume is not migratable, it will be count in non_csi filter.
			if err := pl.checkAttachableInlineVolume(logger, &vol, csiNode, pod, result); err != nil {
				return err
			}

			continue
		}

		if pvcName == "" {
			return fmt.Errorf("PersistentVolumeClaim had no name")
		}

		pvc, err := pl.pvcLister.PersistentVolumeClaims(pod.Namespace).Get(pvcName)

		if err != nil {
			if newPod {
				// The PVC is required to proceed with
				// scheduling of a new pod because it cannot
				// run without it. Bail out immediately.
				return fmt.Errorf("looking up PVC %s/%s: %w", pod.Namespace, pvcName, err)
			}
			// If the PVC is invalid, we don't count the volume because
			// there's no guarantee that it belongs to the running predicate.
			logger.V(5).Info("Unable to look up PVC info", "pod", klog.KObj(pod), "PVC", klog.KRef(pod.Namespace, pvcName))
			continue
		}

		// The PVC for an ephemeral volume must be owned by the pod.
		if isEphemeral {
			if err := ephemeral.VolumeIsForPod(pod, pvc); err != nil {
				return err
			}
		}

		driverName, volumeHandle := pl.getCSIDriverInfo(logger, csiNode, pvc)
		if driverName == "" || volumeHandle == "" {
			logger.V(5).Info("Could not find a CSI driver name or volume handle, not counting volume")
			continue
		}

		volumeUniqueName := fmt.Sprintf("%s/%s", driverName, volumeHandle)
		volumeLimitKey := volumeutil.GetCSIAttachLimitKey(driverName)
		result[volumeUniqueName] = volumeLimitKey
	}
	return nil
}

// checkAttachableInlineVolume takes an inline volume and add to the result map if the
// volume is migratable and CSI migration for this plugin has been enabled.
func (pl *CSILimits) checkAttachableInlineVolume(logger klog.Logger, vol *v1.Volume, csiNode *storagev1.CSINode,
	pod *v1.Pod, result map[string]string) error {
	if !pl.translator.IsInlineMigratable(vol) {
		return nil
	}
	// Check if the intree provisioner CSI migration has been enabled.
	inTreeProvisionerName, err := pl.translator.GetInTreePluginNameFromSpec(nil, vol)
	if err != nil {
		return fmt.Errorf("looking up provisioner name for volume %s: %w", vol.Name, err)
	}
	if !isCSIMigrationOn(csiNode, inTreeProvisionerName) {
		csiNodeName := ""
		if csiNode != nil {
			csiNodeName = csiNode.Name
		}
		logger.V(5).Info("CSI Migration is not enabled for provisioner", "provisioner", inTreeProvisionerName,
			"pod", klog.KObj(pod), "csiNode", csiNodeName)
		return nil
	}
	// Do translation for the in-tree volume.
	translatedPV, err := pl.translator.TranslateInTreeInlineVolumeToCSI(vol, pod.Namespace)
	if err != nil || translatedPV == nil {
		return fmt.Errorf("converting volume(%s) from inline to csi: %w", vol.Name, err)
	}
	driverName, err := pl.translator.GetCSINameFromInTreeName(inTreeProvisionerName)
	if err != nil {
		return fmt.Errorf("looking up CSI driver name for provisioner %s: %w", inTreeProvisionerName, err)
	}
	// TranslateInTreeInlineVolumeToCSI should translate inline volume to CSI. If it is not set,
	// the volume does not support inline. Skip the count.
	if translatedPV.Spec.PersistentVolumeSource.CSI == nil {
		return nil
	}
	volumeUniqueName := fmt.Sprintf("%s/%s", driverName, translatedPV.Spec.PersistentVolumeSource.CSI.VolumeHandle)
	volumeLimitKey := volumeutil.GetCSIAttachLimitKey(driverName)
	result[volumeUniqueName] = volumeLimitKey
	return nil
}

// getCSIDriverInfo returns the CSI driver name and volume ID of a given PVC.
// If the PVC is from a migrated in-tree plugin, this function will return
// the information of the CSI driver that the plugin has been migrated to.
func (pl *CSILimits) getCSIDriverInfo(logger klog.Logger, csiNode *storagev1.CSINode, pvc *v1.PersistentVolumeClaim) (string, string) {
	pvName := pvc.Spec.VolumeName

	if pvName == "" {
		logger.V(5).Info("Persistent volume had no name for claim", "PVC", klog.KObj(pvc))
		return pl.getCSIDriverInfoFromSC(logger, csiNode, pvc)
	}

	pv, err := pl.pvLister.Get(pvName)
	if err != nil {
		logger.V(5).Info("Unable to look up PV info for PVC and PV", "PVC", klog.KObj(pvc), "PV", klog.KRef("", pvName))
		// If we can't fetch PV associated with PVC, may be it got deleted
		// or PVC was prebound to a PVC that hasn't been created yet.
		// fallback to using StorageClass for volume counting
		return pl.getCSIDriverInfoFromSC(logger, csiNode, pvc)
	}

	csiSource := pv.Spec.PersistentVolumeSource.CSI
	if csiSource == nil {
		// We make a fast path for non-CSI volumes that aren't migratable
		if !pl.translator.IsPVMigratable(pv) {
			return "", ""
		}

		pluginName, err := pl.translator.GetInTreePluginNameFromSpec(pv, nil)
		if err != nil {
			logger.V(5).Info("Unable to look up plugin name from PV spec", "err", err)
			return "", ""
		}

		if !isCSIMigrationOn(csiNode, pluginName) {
			logger.V(5).Info("CSI Migration of plugin is not enabled", "plugin", pluginName)
			return "", ""
		}

		csiPV, err := pl.translator.TranslateInTreePVToCSI(pv)
		if err != nil {
			logger.V(5).Info("Unable to translate in-tree volume to CSI", "err", err)
			return "", ""
		}

		if csiPV.Spec.PersistentVolumeSource.CSI == nil {
			logger.V(5).Info("Unable to get a valid volume source for translated PV", "PV", pvName)
			return "", ""
		}

		csiSource = csiPV.Spec.PersistentVolumeSource.CSI
	}

	return csiSource.Driver, csiSource.VolumeHandle
}

// getCSIDriverInfoFromSC returns the CSI driver name and a random volume ID of a given PVC's StorageClass.
func (pl *CSILimits) getCSIDriverInfoFromSC(logger klog.Logger, csiNode *storagev1.CSINode, pvc *v1.PersistentVolumeClaim) (string, string) {
	namespace := pvc.Namespace
	pvcName := pvc.Name
	scName := storagehelpers.GetPersistentVolumeClaimClass(pvc)

	// If StorageClass is not set or not found, then PVC must be using immediate binding mode
	// and hence it must be bound before scheduling. So it is safe to not count it.
	if scName == "" {
		logger.V(5).Info("PVC has no StorageClass", "PVC", klog.KObj(pvc))
		return "", ""
	}

	storageClass, err := pl.scLister.Get(scName)
	if err != nil {
		logger.V(5).Info("Could not get StorageClass for PVC", "PVC", klog.KObj(pvc), "err", err)
		return "", ""
	}

	// We use random prefix to avoid conflict with volume IDs. If PVC is bound during the execution of the
	// predicate and there is another pod on the same node that uses same volume, then we will overcount
	// the volume and consider both volumes as different.
	volumeHandle := fmt.Sprintf("%s-%s/%s", pl.randomVolumeIDPrefix, namespace, pvcName)

	provisioner := storageClass.Provisioner
	if pl.translator.IsMigratableIntreePluginByName(provisioner) {
		if !isCSIMigrationOn(csiNode, provisioner) {
			logger.V(5).Info("CSI Migration of provisioner is not enabled", "provisioner", provisioner)
			return "", ""
		}

		driverName, err := pl.translator.GetCSINameFromInTreeName(provisioner)
		if err != nil {
			logger.V(5).Info("Unable to look up driver name from provisioner name", "provisioner", provisioner, "err", err)
			return "", ""
		}
		return driverName, volumeHandle
	}

	return provisioner, volumeHandle
}

// NewCSI initializes a new plugin and returns it.
func NewCSI(_ context.Context, _ runtime.Object, handle framework.Handle, fts feature.Features) (framework.Plugin, error) {
	informerFactory := handle.SharedInformerFactory()
	pvLister := informerFactory.Core().V1().PersistentVolumes().Lister()
	pvcLister := informerFactory.Core().V1().PersistentVolumeClaims().Lister()
	csiNodesLister := informerFactory.Storage().V1().CSINodes().Lister()
	scLister := informerFactory.Storage().V1().StorageClasses().Lister()
	csiTranslator := csitrans.New()

	return &CSILimits{
		csiNodeLister:        csiNodesLister,
		pvLister:             pvLister,
		pvcLister:            pvcLister,
		scLister:             scLister,
		randomVolumeIDPrefix: rand.String(32),
		translator:           csiTranslator,
	}, nil
}

func getVolumeLimits(nodeInfo *framework.NodeInfo, csiNode *storagev1.CSINode) map[v1.ResourceName]int64 {
	// TODO: stop getting values from Node object in v1.18
	nodeVolumeLimits := volumeLimits(nodeInfo)
	if csiNode != nil {
		for i := range csiNode.Spec.Drivers {
			d := csiNode.Spec.Drivers[i]
			if d.Allocatable != nil && d.Allocatable.Count != nil {
				// TODO: drop GetCSIAttachLimitKey once we don't get values from Node object (v1.18)
				k := v1.ResourceName(volumeutil.GetCSIAttachLimitKey(d.Name))
				nodeVolumeLimits[k] = int64(*d.Allocatable.Count)
			}
		}
	}
	return nodeVolumeLimits
}
