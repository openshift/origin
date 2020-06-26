package resourceapply

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyNamespace merges objectmeta, does not worry about anything else
func ApplyNamespace(client coreclientv1.NamespacesGetter, recorder events.Recorder, required *corev1.Namespace) (*corev1.Namespace, bool, error) {
	existing, err := client.Namespaces().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Namespaces().Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !*modified {
		return existingCopy, false, nil
	}

	if klog.V(4) {
		klog.Infof("Namespace %q changes: %v", required.Name, JSONPatchNoError(existing, existingCopy))
	}

	actual, err := client.Namespaces().Update(existingCopy)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyService merges objectmeta and requires
// TODO, since this cannot determine whether changes are due to legitimate actors (api server) or illegitimate ones (users), we cannot update
// TODO I've special cased the selector for now
func ApplyService(client coreclientv1.ServicesGetter, recorder events.Recorder, required *corev1.Service) (*corev1.Service, bool, error) {
	existing, err := client.Services(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Services(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	selectorSame := equality.Semantic.DeepEqual(existingCopy.Spec.Selector, required.Spec.Selector)

	typeSame := false
	requiredIsEmpty := len(required.Spec.Type) == 0
	existingCopyIsCluster := existingCopy.Spec.Type == corev1.ServiceTypeClusterIP
	if (requiredIsEmpty && existingCopyIsCluster) || equality.Semantic.DeepEqual(existingCopy.Spec.Type, required.Spec.Type) {
		typeSame = true
	}

	if selectorSame && typeSame && !*modified {
		return existingCopy, false, nil
	}

	existingCopy.Spec.Selector = required.Spec.Selector
	existingCopy.Spec.Type = required.Spec.Type // if this is different, the update will fail.  Status will indicate it.

	if klog.V(4) {
		klog.Infof("Service %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, required))
	}

	actual, err := client.Services(required.Namespace).Update(existingCopy)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyPod merges objectmeta, does not worry about anything else
func ApplyPod(client coreclientv1.PodsGetter, recorder events.Recorder, required *corev1.Pod) (*corev1.Pod, bool, error) {
	existing, err := client.Pods(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Pods(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !*modified {
		return existingCopy, false, nil
	}

	if klog.V(4) {
		klog.Infof("Pod %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, required))
	}

	actual, err := client.Pods(required.Namespace).Update(existingCopy)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyServiceAccount merges objectmeta, does not worry about anything else
func ApplyServiceAccount(client coreclientv1.ServiceAccountsGetter, recorder events.Recorder, required *corev1.ServiceAccount) (*corev1.ServiceAccount, bool, error) {
	existing, err := client.ServiceAccounts(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.ServiceAccounts(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !*modified {
		return existingCopy, false, nil
	}
	if klog.V(4) {
		klog.Infof("ServiceAccount %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, required))
	}
	actual, err := client.ServiceAccounts(required.Namespace).Update(existingCopy)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyConfigMap merges objectmeta, requires data
func ApplyConfigMap(client coreclientv1.ConfigMapsGetter, recorder events.Recorder, required *corev1.ConfigMap) (*corev1.ConfigMap, bool, error) {
	existing, err := client.ConfigMaps(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.ConfigMaps(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)

	caBundleInjected := required.Labels["config.openshift.io/inject-trusted-cabundle"] == "true"
	_, newCABundleRequired := required.Data["ca-bundle.crt"]

	var modifiedKeys []string
	for existingCopyKey, existingCopyValue := range existingCopy.Data {
		// if we're injecting a ca-bundle and the required isn't forcing the value, then don't use the value of existing
		// to drive a diff detection. If required has set the value then we need to force the value in order to have apply
		// behave predictably.
		if caBundleInjected && !newCABundleRequired && existingCopyKey == "ca-bundle.crt" {
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
	if dataSame && !*modified {
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

	actual, err := client.ConfigMaps(required.Namespace).Update(existingCopy)

	var details string
	if !dataSame {
		sort.Sort(sort.StringSlice(modifiedKeys))
		details = fmt.Sprintf("cause by changes in %v", strings.Join(modifiedKeys, ","))
	}
	if klog.V(4) {
		klog.Infof("ConfigMap %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, required))
	}
	reportUpdateEvent(recorder, required, err, details)
	return actual, true, err
}

// ApplySecret merges objectmeta, requires data
func ApplySecret(client coreclientv1.SecretsGetter, recorder events.Recorder, required *corev1.Secret) (*corev1.Secret, bool, error) {
	if len(required.StringData) > 0 {
		return nil, false, fmt.Errorf("Secret.stringData is not supported")
	}

	existing, err := client.Secrets(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Secrets(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(resourcemerge.BoolPtr(false), &existingCopy.ObjectMeta, required.ObjectMeta)

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

	if equality.Semantic.DeepEqual(existingCopy, existing) {
		return existing, false, nil
	}

	if klog.V(4) {
		klog.Infof("Secret %s/%s changes: %v", required.Namespace, required.Name, JSONPatchSecretNoError(existing, existingCopy))
	}
	actual, err := client.Secrets(required.Namespace).Update(existingCopy)
	reportUpdateEvent(recorder, existingCopy, err)

	if err == nil {
		return actual, true, err
	}
	if !strings.Contains(err.Error(), "field is immutable") {
		return actual, true, err
	}

	// if the field was immutable on a secret, we're going to be stuck until we delete it.  Try to delete and then create
	deleteErr := client.Secrets(required.Namespace).Delete(existingCopy.Name, nil)
	reportDeleteEvent(recorder, existingCopy, deleteErr)

	// clear the RV and track the original actual and error for the return like our create value.
	existingCopy.ResourceVersion = ""
	actual, err = client.Secrets(required.Namespace).Create(existingCopy)
	reportCreateEvent(recorder, existingCopy, err)

	return actual, true, err
}

func SyncConfigMap(client coreclientv1.ConfigMapsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, ownerRefs []metav1.OwnerReference) (*corev1.ConfigMap, bool, error) {
	source, err := client.ConfigMaps(sourceNamespace).Get(sourceName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		deleteErr := client.ConfigMaps(targetNamespace).Delete(targetName, nil)
		if _, getErr := client.ConfigMaps(targetNamespace).Get(targetName, metav1.GetOptions{}); getErr != nil && apierrors.IsNotFound(getErr) {
			return nil, true, nil
		}
		if apierrors.IsNotFound(deleteErr) {
			return nil, false, nil
		}
		if deleteErr == nil {
			recorder.Eventf("TargetConfigDeleted", "Deleted target configmap %s/%s because source config does not exist", targetNamespace, targetName)
			return nil, true, nil
		}
		return nil, false, deleteErr
	case err != nil:
		return nil, false, err
	default:
		source.Namespace = targetNamespace
		source.Name = targetName
		source.ResourceVersion = ""
		source.OwnerReferences = ownerRefs
		return ApplyConfigMap(client, recorder, source)
	}
}

func SyncSecret(client coreclientv1.SecretsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, ownerRefs []metav1.OwnerReference) (*corev1.Secret, bool, error) {
	source, err := client.Secrets(sourceNamespace).Get(sourceName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		if _, getErr := client.Secrets(targetNamespace).Get(targetName, metav1.GetOptions{}); getErr != nil && apierrors.IsNotFound(getErr) {
			return nil, true, nil
		}
		deleteErr := client.Secrets(targetNamespace).Delete(targetName, nil)
		if apierrors.IsNotFound(deleteErr) {
			return nil, false, nil
		}
		if deleteErr == nil {
			recorder.Eventf("TargetSecretDeleted", "Deleted target secret %s/%s because source config does not exist", targetNamespace, targetName)
			return nil, true, nil
		}
		return nil, false, deleteErr
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

		source.Namespace = targetNamespace
		source.Name = targetName
		source.ResourceVersion = ""
		source.OwnerReferences = ownerRefs
		return ApplySecret(client, recorder, source)
	}
}
