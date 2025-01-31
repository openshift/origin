package resourceapply

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

// TODO find  way to create a registry of these based on struct mapping or some such that forces users to get this right
//
//	for creating an ApplyGeneric
//	Perhaps a struct containing the apply function and the getKind
func getCoreGroupKind(obj runtime.Object) *schema.GroupKind {
	switch obj.(type) {
	case *corev1.Namespace:
		return &schema.GroupKind{
			Kind: "Namespace",
		}
	case *corev1.Service:
		return &schema.GroupKind{
			Kind: "Service",
		}
	case *corev1.Pod:
		return &schema.GroupKind{
			Kind: "Pod",
		}
	case *corev1.ServiceAccount:
		return &schema.GroupKind{
			Kind: "ServiceAccount",
		}
	case *corev1.ConfigMap:
		return &schema.GroupKind{
			Kind: "ConfigMap",
		}
	case *corev1.Secret:
		return &schema.GroupKind{
			Kind: "Secret",
		}
	default:
		return nil
	}
}

// ApplyNamespace merges objectmeta, does not worry about anything else
func ApplyNamespace(ctx context.Context, client coreclientv1.NamespacesGetter, recorder events.Recorder, required *corev1.Namespace) (*corev1.Namespace, bool, error) {
	return ApplyNamespaceImproved(ctx, client, recorder, required, noCache)
}

// ApplyService merges objectmeta and requires
// TODO, since this cannot determine whether changes are due to legitimate actors (api server) or illegitimate ones (users), we cannot update
// TODO I've special cased the selector for now
func ApplyService(ctx context.Context, client coreclientv1.ServicesGetter, recorder events.Recorder, required *corev1.Service) (*corev1.Service, bool, error) {
	return ApplyServiceImproved(ctx, client, recorder, required, noCache)
}

// ApplyPod merges objectmeta, does not worry about anything else
func ApplyPod(ctx context.Context, client coreclientv1.PodsGetter, recorder events.Recorder, required *corev1.Pod) (*corev1.Pod, bool, error) {
	return ApplyPodImproved(ctx, client, recorder, required, noCache)
}

// ApplyServiceAccount merges objectmeta, does not worry about anything else
func ApplyServiceAccount(ctx context.Context, client coreclientv1.ServiceAccountsGetter, recorder events.Recorder, required *corev1.ServiceAccount) (*corev1.ServiceAccount, bool, error) {
	return ApplyServiceAccountImproved(ctx, client, recorder, required, noCache)
}

// ApplyConfigMap merges objectmeta, requires data
func ApplyConfigMap(ctx context.Context, client coreclientv1.ConfigMapsGetter, recorder events.Recorder, required *corev1.ConfigMap) (*corev1.ConfigMap, bool, error) {
	return ApplyConfigMapImproved(ctx, client, recorder, required, noCache)
}

// ApplySecret merges objectmeta, requires data
func ApplySecret(ctx context.Context, client coreclientv1.SecretsGetter, recorder events.Recorder, required *corev1.Secret) (*corev1.Secret, bool, error) {
	return ApplySecretImproved(ctx, client, recorder, required, noCache)
}

// ApplyNamespace merges objectmeta, does not worry about anything else
func ApplyNamespaceImproved(ctx context.Context, client coreclientv1.NamespacesGetter, recorder events.Recorder, required *corev1.Namespace, cache ResourceCache) (*corev1.Namespace, bool, error) {
	existing, err := client.Namespaces().Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.Namespaces().
			Create(ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*corev1.Namespace), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, requiredCopy, err)
		cache.UpdateCachedResourceMetadata(required, actual)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(required, existing) {
		return existing, false, nil
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !modified {
		cache.UpdateCachedResourceMetadata(required, existingCopy)
		return existingCopy, false, nil
	}

	if klog.V(2).Enabled() {
		klog.Infof("Namespace %q changes: %v", required.Name, JSONPatchNoError(existing, existingCopy))
	}

	actual, err := client.Namespaces().Update(ctx, existingCopy, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	cache.UpdateCachedResourceMetadata(required, actual)
	return actual, true, err
}

// ApplyService merges objectmeta and requires.
// It detects changes in `required`, i.e. an operator needs .spec changes and overwrites existing .spec with those.
// TODO, since this cannot determine whether changes in `existing` are due to legitimate actors (api server) or illegitimate ones (users), we cannot update.
// TODO I've special cased the selector for now
func ApplyServiceImproved(ctx context.Context, client coreclientv1.ServicesGetter, recorder events.Recorder, requiredOriginal *corev1.Service, cache ResourceCache) (*corev1.Service, bool, error) {
	required := requiredOriginal.DeepCopy()
	err := SetSpecHashAnnotation(&required.ObjectMeta, required.Spec)
	if err != nil {
		return nil, false, err
	}

	existing, err := client.Services(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.Services(requiredCopy.Namespace).
			Create(ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*corev1.Service), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, requiredCopy, err)
		cache.UpdateCachedResourceMetadata(required, actual)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(required, existing) {
		return existing, false, nil
	}

	modified := false
	existingCopy := existing.DeepCopy()

	// This will catch also changes between old `required.spec` and current `required.spec`, because
	// the annotation from SetSpecHashAnnotation will be different.
	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	selectorSame := equality.Semantic.DeepEqual(existingCopy.Spec.Selector, required.Spec.Selector)

	typeSame := false
	requiredIsEmpty := len(required.Spec.Type) == 0
	existingCopyIsCluster := existingCopy.Spec.Type == corev1.ServiceTypeClusterIP
	if (requiredIsEmpty && existingCopyIsCluster) || equality.Semantic.DeepEqual(existingCopy.Spec.Type, required.Spec.Type) {
		typeSame = true
	}

	if selectorSame && typeSame && !modified {
		cache.UpdateCachedResourceMetadata(required, existingCopy)
		return existingCopy, false, nil
	}

	// Either (user changed selector or type) or metadata changed (incl. spec hash). Stomp over
	// any user *and* Kubernetes changes, hoping that Kubernetes will restore its values.
	existingCopy.Spec = required.Spec
	if klog.V(4).Enabled() {
		klog.Infof("Service %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, required))
	}

	actual, err := client.Services(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	cache.UpdateCachedResourceMetadata(required, actual)
	return actual, true, err
}

// ApplyPod merges objectmeta, does not worry about anything else
func ApplyPodImproved(ctx context.Context, client coreclientv1.PodsGetter, recorder events.Recorder, required *corev1.Pod, cache ResourceCache) (*corev1.Pod, bool, error) {
	existing, err := client.Pods(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.Pods(requiredCopy.Namespace).
			Create(ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*corev1.Pod), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, requiredCopy, err)
		cache.UpdateCachedResourceMetadata(required, actual)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(required, existing) {
		return existing, false, nil
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !modified {
		cache.UpdateCachedResourceMetadata(required, existingCopy)
		return existingCopy, false, nil
	}

	if klog.V(2).Enabled() {
		klog.Infof("Pod %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, required))
	}

	actual, err := client.Pods(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	cache.UpdateCachedResourceMetadata(required, actual)
	return actual, true, err
}

// ApplyServiceAccount merges objectmeta, does not worry about anything else
func ApplyServiceAccountImproved(ctx context.Context, client coreclientv1.ServiceAccountsGetter, recorder events.Recorder, required *corev1.ServiceAccount, cache ResourceCache) (*corev1.ServiceAccount, bool, error) {
	existing, err := client.ServiceAccounts(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.ServiceAccounts(requiredCopy.Namespace).
			Create(ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*corev1.ServiceAccount), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, requiredCopy, err)
		cache.UpdateCachedResourceMetadata(required, actual)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(required, existing) {
		return existing, false, nil
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !modified {
		cache.UpdateCachedResourceMetadata(required, existingCopy)
		return existingCopy, false, nil
	}
	if klog.V(2).Enabled() {
		klog.Infof("ServiceAccount %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, required))
	}
	actual, err := client.ServiceAccounts(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	cache.UpdateCachedResourceMetadata(required, actual)
	return actual, true, err
}

// ApplyConfigMap merges objectmeta, requires data
func ApplyConfigMapImproved(ctx context.Context, client coreclientv1.ConfigMapsGetter, recorder events.Recorder, required *corev1.ConfigMap, cache ResourceCache) (*corev1.ConfigMap, bool, error) {
	existing, err := client.ConfigMaps(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.ConfigMaps(requiredCopy.Namespace).
			Create(ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*corev1.ConfigMap), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, requiredCopy, err)
		cache.UpdateCachedResourceMetadata(required, actual)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(required, existing) {
		return existing, false, nil
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)

	// injected by cluster-network-operator: https://github.com/openshift/cluster-network-operator/blob/acc819ee0f3424a341b9ad4e1e83ca0a742c230a/docs/architecture.md?L192#configmap-ca-injector
	caBundleInjected := required.Labels["config.openshift.io/inject-trusted-cabundle"] == "true"
	_, newCABundleRequired := required.Data["ca-bundle.crt"]

	// injected by service-ca-operator: https://github.com/openshift/service-ca-operator/blob/f409fb9e308ace1e5f8596add187d2239b073e23/README.md#openshift-service-ca-operator
	serviceCAInjected := required.Annotations["service.beta.openshift.io/inject-cabundle"] == "true"
	_, newServiceCARequired := required.Data["service-ca.crt"]

	var modifiedKeys []string
	for existingCopyKey, existingCopyValue := range existingCopy.Data {
		// if we're injecting a ca-bundle or a service-ca and the required isn't forcing the value, then don't use the value of existing
		// to drive a diff detection. If required has set the value then we need to force the value in order to have apply
		// behave predictably.
		if caBundleInjected && !newCABundleRequired && existingCopyKey == "ca-bundle.crt" {
			continue
		}
		if serviceCAInjected && !newServiceCARequired && existingCopyKey == "service-ca.crt" {
			continue
		}

		if requiredValue, ok := required.Data[existingCopyKey]; !ok || (existingCopyValue != requiredValue) {
			modifiedKeys = append(modifiedKeys, "data."+existingCopyKey)
		}
	}
	for existingCopyKey, existingCopyBinValue := range existingCopy.BinaryData {
		if requiredBinValue, ok := required.BinaryData[existingCopyKey]; !ok || !bytes.Equal(existingCopyBinValue, requiredBinValue) {
			modifiedKeys = append(modifiedKeys, "binaryData."+existingCopyKey)
		}
	}
	for requiredKey := range required.Data {
		if _, ok := existingCopy.Data[requiredKey]; !ok {
			modifiedKeys = append(modifiedKeys, "data."+requiredKey)
		}
	}
	for requiredBinKey := range required.BinaryData {
		if _, ok := existingCopy.BinaryData[requiredBinKey]; !ok {
			modifiedKeys = append(modifiedKeys, "binaryData."+requiredBinKey)
		}
	}

	dataSame := len(modifiedKeys) == 0
	if dataSame && !modified {
		cache.UpdateCachedResourceMetadata(required, existingCopy)
		return existingCopy, false, nil
	}
	existingCopy.Data = required.Data
	existingCopy.BinaryData = required.BinaryData
	// if we're injecting a cabundle, and we had a previous value, and the required object isn't setting the value, then set back to the previous
	if existingCABundle, existedBefore := existing.Data["ca-bundle.crt"]; caBundleInjected && existedBefore && !newCABundleRequired {
		if existingCopy.Data == nil {
			existingCopy.Data = map[string]string{}
		}
		existingCopy.Data["ca-bundle.crt"] = existingCABundle
	}

	actual, err := client.ConfigMaps(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})

	var details string
	if !dataSame {
		sort.Sort(sort.StringSlice(modifiedKeys))
		details = fmt.Sprintf("cause by changes in %v", strings.Join(modifiedKeys, ","))
	}
	if klog.V(2).Enabled() {
		klog.Infof("ConfigMap %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, required))
	}
	resourcehelper.ReportUpdateEvent(recorder, required, err, details)
	cache.UpdateCachedResourceMetadata(required, actual)
	return actual, true, err
}

// ApplySecret merges objectmeta, requires data
func ApplySecretImproved(ctx context.Context, client coreclientv1.SecretsGetter, recorder events.Recorder, requiredInput *corev1.Secret, cache ResourceCache) (*corev1.Secret, bool, error) {
	// copy the stringData to data.  Error on a data content conflict inside required.  This is usually a bug.

	existing, err := client.Secrets(requiredInput.Namespace).Get(ctx, requiredInput.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, false, err
	}

	if cache.SafeToSkipApply(requiredInput, existing) {
		return existing, false, nil
	}

	required := requiredInput.DeepCopy()
	if required.Data == nil {
		required.Data = map[string][]byte{}
	}
	for k, v := range required.StringData {
		if dataV, ok := required.Data[k]; ok {
			if string(dataV) != v {
				return nil, false, fmt.Errorf("Secret.stringData[%q] conflicts with Secret.data[%q]", k, k)
			}
		}
		required.Data[k] = []byte(v)
	}
	required.StringData = nil

	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.Secrets(requiredCopy.Namespace).
			Create(ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*corev1.Secret), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, requiredCopy, err)
		cache.UpdateCachedResourceMetadata(requiredInput, actual)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(ptr.To(false), &existingCopy.ObjectMeta, required.ObjectMeta)

	switch required.Type {
	case corev1.SecretTypeServiceAccountToken:
		// Secrets for ServiceAccountTokens will have data injected by kube controller manager.
		// We will apply only the explicitly set keys.
		if existingCopy.Data == nil {
			existingCopy.Data = map[string][]byte{}
		}

		for k, v := range required.Data {
			existingCopy.Data[k] = v
		}

	default:
		existingCopy.Data = required.Data
	}

	existingCopy.Type = required.Type

	// Server defaults some values and we need to do it as well or it will never equal.
	if existingCopy.Type == "" {
		existingCopy.Type = corev1.SecretTypeOpaque
	}

	if equality.Semantic.DeepEqual(existingCopy, existing) {
		cache.UpdateCachedResourceMetadata(requiredInput, existingCopy)
		return existing, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("Secret %s/%s changes: %v", required.Namespace, required.Name, JSONPatchSecretNoError(existing, existingCopy))
	}

	var actual *corev1.Secret
	/*
	 * Kubernetes validation silently hides failures to update secret type.
	 * https://github.com/kubernetes/kubernetes/blob/98e65951dccfd40d3b4f31949c2ab8df5912d93e/pkg/apis/core/validation/validation.go#L5048
	 * We need to explicitly opt for delete+create in that case.
	 */
	if existingCopy.Type == existing.Type {
		actual, err = client.Secrets(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
		resourcehelper.ReportUpdateEvent(recorder, existingCopy, err)

		if err == nil {
			return actual, true, err
		}
		if !strings.Contains(err.Error(), "field is immutable") {
			return actual, true, err
		}
	}

	// if the field was immutable on a secret, we're going to be stuck until we delete it.  Try to delete and then create
	deleteErr := client.Secrets(required.Namespace).Delete(ctx, existingCopy.Name, metav1.DeleteOptions{})
	resourcehelper.ReportDeleteEvent(recorder, existingCopy, deleteErr)

	// clear the RV and track the original actual and error for the return like our create value.
	existingCopy.ResourceVersion = ""
	actual, err = client.Secrets(required.Namespace).Create(ctx, existingCopy, metav1.CreateOptions{})
	resourcehelper.ReportCreateEvent(recorder, existingCopy, err)
	cache.UpdateCachedResourceMetadata(requiredInput, actual)
	return actual, true, err
}

// SyncConfigMap applies a ConfigMap from a location `sourceNamespace/sourceName` to `targetNamespace/targetName`
func SyncConfigMap(ctx context.Context, client coreclientv1.ConfigMapsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, ownerRefs []metav1.OwnerReference) (*corev1.ConfigMap, bool, error) {
	return syncPartialConfigMap(ctx, client, recorder, sourceNamespace, sourceName, targetNamespace, targetName, nil, ownerRefs, nil)
}

// SyncConfigMapWithLabels does what SyncConfigMap does, but adds additional labels to the target ConfigMap.
func SyncConfigMapWithLabels(ctx context.Context, client coreclientv1.ConfigMapsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, ownerRefs []metav1.OwnerReference, labels map[string]string) (*corev1.ConfigMap, bool, error) {
	return syncPartialConfigMap(ctx, client, recorder, sourceNamespace, sourceName, targetNamespace, targetName, nil, ownerRefs, labels)
}

// SyncPartialConfigMap does what SyncConfigMap does but it only synchronizes a subset of keys given by `syncedKeys`.
// SyncPartialConfigMap will delete the target if `syncedKeys` are set but the source does not contain any of these keys.
func SyncPartialConfigMap(ctx context.Context, client coreclientv1.ConfigMapsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, syncedKeys sets.Set[string], ownerRefs []metav1.OwnerReference) (*corev1.ConfigMap, bool, error) {
	return syncPartialConfigMap(ctx, client, recorder, sourceNamespace, sourceName, targetNamespace, targetName, syncedKeys, ownerRefs, nil)
}

func syncPartialConfigMap(ctx context.Context, client coreclientv1.ConfigMapsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, syncedKeys sets.Set[string], ownerRefs []metav1.OwnerReference, labels map[string]string) (*corev1.ConfigMap, bool, error) {
	source, err := client.ConfigMaps(sourceNamespace).Get(ctx, sourceName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		modified, err := deleteConfigMapSyncTarget(ctx, client, recorder, targetNamespace, targetName)
		return nil, modified, err
	case err != nil:
		return nil, false, err
	default:
		if len(syncedKeys) > 0 {
			for sourceKey := range source.Data {
				if !syncedKeys.Has(sourceKey) {
					delete(source.Data, sourceKey)
				}
			}
			for sourceKey := range source.BinaryData {
				if !syncedKeys.Has(sourceKey) {
					delete(source.BinaryData, sourceKey)
				}
			}

			// remove the synced CM if the requested fields are not present in source
			if len(source.Data)+len(source.BinaryData) == 0 {
				modified, err := deleteConfigMapSyncTarget(ctx, client, recorder, targetNamespace, targetName)
				return nil, modified, err
			}
		}

		source.Namespace = targetNamespace
		source.Name = targetName
		source.ResourceVersion = ""
		source.OwnerReferences = ownerRefs
		if labels != nil && source.Labels == nil {
			source.Labels = map[string]string{}
		}
		for k, v := range labels {
			source.Labels[k] = v
		}
		return ApplyConfigMap(ctx, client, recorder, source)
	}
}

func deleteConfigMapSyncTarget(ctx context.Context, client coreclientv1.ConfigMapsGetter, recorder events.Recorder, targetNamespace, targetName string) (bool, error) {
	// This goal of this additional GET is to avoid reaching the API with a DELETE request
	// in case the target doesn't exist. This is useful when using a cached client.
	_, err := client.ConfigMaps(targetNamespace).Get(ctx, targetName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	err = client.ConfigMaps(targetNamespace).Delete(ctx, targetName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err == nil {
		recorder.Eventf("TargetConfigDeleted", "Deleted target configmap %s/%s because source config does not exist", targetNamespace, targetName)
		return true, nil
	}
	return false, err
}

// SyncSecret applies a Secret from a location `sourceNamespace/sourceName` to `targetNamespace/targetName`
func SyncSecret(ctx context.Context, client coreclientv1.SecretsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, ownerRefs []metav1.OwnerReference) (*corev1.Secret, bool, error) {
	return syncPartialSecret(ctx, client, recorder, sourceNamespace, sourceName, targetNamespace, targetName, nil, ownerRefs, nil)
}

// SyncSecretWithLabels does what SyncSecret does, but adds additional labels to the target Secret.
func SyncSecretWithLabels(ctx context.Context, client coreclientv1.SecretsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, ownerRefs []metav1.OwnerReference, labels map[string]string) (*corev1.Secret, bool, error) {
	return syncPartialSecret(ctx, client, recorder, sourceNamespace, sourceName, targetNamespace, targetName, nil, ownerRefs, labels)
}

// SyncPartialSecret does what SyncSecret does but it only synchronizes a subset of keys given by `syncedKeys`.
// SyncPartialSecret will delete the target if `syncedKeys` are set but the source does not contain any of these keys.
func SyncPartialSecret(ctx context.Context, client coreclientv1.SecretsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, syncedKeys sets.Set[string], ownerRefs []metav1.OwnerReference) (*corev1.Secret, bool, error) {
	return syncPartialSecret(ctx, client, recorder, sourceNamespace, sourceName, targetNamespace, targetName, syncedKeys, ownerRefs, nil)
}

func syncPartialSecret(ctx context.Context, client coreclientv1.SecretsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, syncedKeys sets.Set[string], ownerRefs []metav1.OwnerReference, labels map[string]string) (*corev1.Secret, bool, error) {
	source, err := client.Secrets(sourceNamespace).Get(ctx, sourceName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		modified, err := deleteSecretSyncTarget(ctx, client, recorder, targetNamespace, targetName)
		return nil, modified, err
	case err != nil:
		return nil, false, err
	default:
		if source.Type == corev1.SecretTypeServiceAccountToken {

			// Make sure the token is already present, otherwise we have to wait before creating the target
			if len(source.Data[corev1.ServiceAccountTokenKey]) == 0 {
				return nil, false, fmt.Errorf("secret %s/%s doesn't have a token yet", source.Namespace, source.Name)
			}

			if source.Annotations != nil {
				// When syncing a service account token we have to remove the SA annotation to disable injection into copies
				delete(source.Annotations, corev1.ServiceAccountNameKey)
				// To make it clean, remove the dormant annotations as well
				delete(source.Annotations, corev1.ServiceAccountUIDKey)
			}

			// SecretTypeServiceAccountToken implies required fields and injection which we do not want in copies
			source.Type = corev1.SecretTypeOpaque
		}

		if len(syncedKeys) > 0 {
			for sourceKey := range source.Data {
				if !syncedKeys.Has(sourceKey) {
					delete(source.Data, sourceKey)
				}
			}
			for sourceKey := range source.StringData {
				if !syncedKeys.Has(sourceKey) {
					delete(source.StringData, sourceKey)
				}
			}

			// remove the synced secret if the requested fields are not present in source
			if len(source.Data)+len(source.StringData) == 0 {
				modified, err := deleteSecretSyncTarget(ctx, client, recorder, targetNamespace, targetName)
				return nil, modified, err
			}
		}

		source.Namespace = targetNamespace
		source.Name = targetName
		source.ResourceVersion = ""
		source.OwnerReferences = ownerRefs
		if labels != nil && source.Labels == nil {
			source.Labels = map[string]string{}
		}
		for k, v := range labels {
			source.Labels[k] = v
		}
		return ApplySecret(ctx, client, recorder, source)
	}
}

func deleteSecretSyncTarget(ctx context.Context, client coreclientv1.SecretsGetter, recorder events.Recorder, targetNamespace, targetName string) (bool, error) {
	err := client.Secrets(targetNamespace).Delete(ctx, targetName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err == nil {
		recorder.Eventf("TargetSecretDeleted", "Deleted target secret %s/%s because source config does not exist", targetNamespace, targetName)
		return true, nil
	}
	return false, err
}

func DeleteNamespace(ctx context.Context, client coreclientv1.NamespacesGetter, recorder events.Recorder, required *corev1.Namespace) (*corev1.Namespace, bool, error) {
	err := client.Namespaces().Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func DeleteService(ctx context.Context, client coreclientv1.ServicesGetter, recorder events.Recorder, required *corev1.Service) (*corev1.Service, bool, error) {
	err := client.Services(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func DeletePod(ctx context.Context, client coreclientv1.PodsGetter, recorder events.Recorder, required *corev1.Pod) (*corev1.Pod, bool, error) {
	err := client.Pods(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func DeleteServiceAccount(ctx context.Context, client coreclientv1.ServiceAccountsGetter, recorder events.Recorder, required *corev1.ServiceAccount) (*corev1.ServiceAccount, bool, error) {
	err := client.ServiceAccounts(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func DeleteConfigMap(ctx context.Context, client coreclientv1.ConfigMapsGetter, recorder events.Recorder, required *corev1.ConfigMap) (*corev1.ConfigMap, bool, error) {
	err := client.ConfigMaps(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func DeleteSecret(ctx context.Context, client coreclientv1.SecretsGetter, recorder events.Recorder, required *corev1.Secret) (*corev1.Secret, bool, error) {
	err := client.Secrets(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}
