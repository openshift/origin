package resourceapply

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"

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

	if glog.V(4) {
		glog.Infof("Namespace %q changes: %v", required.Name, JSONPatch(existing, existingCopy))
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

	if glog.V(4) {
		glog.Infof("Service %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, required))
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

	if glog.V(4) {
		glog.Infof("Pod %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, required))
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
	if glog.V(4) {
		glog.Infof("ServiceAccount %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, required))
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

	var modifiedKeys []string
	for existingCopyKey, existingCopyValue := range existingCopy.Data {
		if requiredValue, ok := required.Data[existingCopyKey]; !ok || (existingCopyValue != requiredValue) {
			modifiedKeys = append(modifiedKeys, "data."+existingCopyKey)
		}
	}
	for requiredKey := range required.Data {
		if _, ok := existingCopy.Data[requiredKey]; !ok {
			modifiedKeys = append(modifiedKeys, "data."+requiredKey)
		}
	}

	dataSame := len(modifiedKeys) == 0
	if dataSame && !*modified {
		return existingCopy, false, nil
	}
	existingCopy.Data = required.Data

	actual, err := client.ConfigMaps(required.Namespace).Update(existingCopy)

	var details string
	if !dataSame {
		sort.Sort(sort.StringSlice(modifiedKeys))
		details = fmt.Sprintf("cause by changes in %v", strings.Join(modifiedKeys, ","))
	}
	if glog.V(4) {
		glog.Infof("ConfigMap %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, required))
	}
	reportUpdateEvent(recorder, required, err, details)
	return actual, true, err
}

// ApplySecret merges objectmeta, requires data
func ApplySecret(client coreclientv1.SecretsGetter, recorder events.Recorder, required *corev1.Secret) (*corev1.Secret, bool, error) {
	existing, err := client.Secrets(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Secrets(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	dataSame := equality.Semantic.DeepEqual(existingCopy.Data, required.Data)
	if dataSame && !*modified {
		return existingCopy, false, nil
	}
	existingCopy.Data = required.Data

	if glog.V(4) {
		glog.Infof("Secret %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, required))
	}
	actual, err := client.Secrets(required.Namespace).Update(existingCopy)

	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

func SyncConfigMap(client coreclientv1.ConfigMapsGetter, recorder events.Recorder, sourceNamespace, sourceName, targetNamespace, targetName string, ownerRefs []metav1.OwnerReference) (*corev1.ConfigMap, bool, error) {
	source, err := client.ConfigMaps(sourceNamespace).Get(sourceName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		deleteErr := client.ConfigMaps(targetNamespace).Delete(targetName, nil)
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
		source.Namespace = targetNamespace
		source.Name = targetName
		source.ResourceVersion = ""
		source.OwnerReferences = ownerRefs
		return ApplySecret(client, recorder, source)
	}
}
