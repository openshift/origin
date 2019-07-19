package clusterquotamapping

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kexternalinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

	quotav1 "github.com/openshift/api/quota/v1"
	quotaclient "github.com/openshift/client-go/quota/clientset/versioned/fake"
	quotainformer "github.com/openshift/client-go/quota/informers/externalversions"
)

var (
	keys             = []string{"different", "used", "important", "every", "large"}
	values           = []string{"time", "person"}
	annotationKeys   = []string{"different", "used", "important", "every", "large", "foo.bar.baz/key", "whitespace key"}
	annotationValues = []string{"Person", "time and place", "Thing", "me@example.com", "system:admin"}
	namespaceNames   = []string{
		"tokillamockingbird", "harrypotter", "1984", "prideandprejudice", "thediaryofayounggirl", "animalfarm", "thehobbit",
		"thelittleprince", "thegreatgatsby", "thecatcherintherye", "lordoftherings", "janeeyre", "romeoandjuliet", "thechroniclesofnarnia",
		"lordoftheflies", "thegivingtree", "charlottesweb", "greeneggsandham", "alicesadventuresinwonderland", "littlewomen",
		"ofmiceandmend", "wutheringheights", "thehungergames", "gonewiththewind", "thepictureofdoriangray", "theadventuresofhuckleberryfinn",
		"fahrenheit451", "hamlet", "thehitchhikersguidetothegalaxy", "bravenewworld", "lesmiserables", "crimeandpunishment", "memoirsofageisha",
	}
	quotaNames = []string{"emma", "olivia", "sophia", "ava", "isabella", "mia", "abigail", "emily", "charlotte", "harper"}

	maxSelectorKeys = 2
	maxLabels       = 5
)

func TestClusterQuotaFuzzer(t *testing.T) {
	for j := 0; j < 100; j++ {
		t.Logf("attempt %d", (j + 1))
		runFuzzer(t)
	}
}

func runFuzzer(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	startingNamespaces := CreateStartingNamespaces()
	kubeClient := fake.NewSimpleClientset(startingNamespaces...)
	nsWatch := watch.NewFake()
	kubeClient.PrependWatchReactor("namespaces", clientgotesting.DefaultWatchReactor(nsWatch, nil))

	kubeInformerFactory := kexternalinformers.NewSharedInformerFactory(kubeClient, 10*time.Minute)

	startingQuotas := CreateStartingQuotas()
	quotaWatch := watch.NewFake()
	quotaClient := quotaclient.NewSimpleClientset(startingQuotas...)
	quotaClient.PrependWatchReactor("clusterresourcequotas", clientgotesting.DefaultWatchReactor(quotaWatch, nil))
	quotaFactory := quotainformer.NewSharedInformerFactory(quotaClient, 0)

	controller := NewClusterQuotaMappingController(kubeInformerFactory.Core().V1().Namespaces(), quotaFactory.Quota().V1().ClusterResourceQuotas())
	go controller.Run(5, stopCh)
	quotaFactory.Start(stopCh)
	kubeInformerFactory.Start(stopCh)

	finalNamespaces := map[string]*corev1.Namespace{}
	finalQuotas := map[string]*quotav1.ClusterResourceQuota{}
	quotaActions := map[string][]string{}
	namespaceActions := map[string][]string{}
	finishedNamespaces := make(chan struct{})
	finishedQuotas := make(chan struct{})

	for _, quota := range startingQuotas {
		name := quota.(*quotav1.ClusterResourceQuota).Name
		quotaActions[name] = append(quotaActions[name], fmt.Sprintf("inserting %v to %v", name, quota.(*quotav1.ClusterResourceQuota).Spec.Selector))
		finalQuotas[name] = quota.(*quotav1.ClusterResourceQuota)
	}
	for _, namespace := range startingNamespaces {
		name := namespace.(*corev1.Namespace).Name
		namespaceActions[name] = append(namespaceActions[name], fmt.Sprintf("inserting %v to %v", name, namespace.(*corev1.Namespace).Labels))
		finalNamespaces[name] = namespace.(*corev1.Namespace)
	}

	go func() {
		for i := 0; i < 200; i++ {
			name := quotaNames[rand.Intn(len(quotaNames))]
			_, exists := finalQuotas[name]
			if rand.Intn(50) == 0 {
				if !exists {
					continue
				}
				// due to the compression race (see big comment for impl), clear the queue then delete
				for {
					if len(quotaWatch.ResultChan()) == 0 {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}

				quotaActions[name] = append(quotaActions[name], "deleting "+name)
				quotaWatch.Delete(finalQuotas[name])
				delete(finalQuotas, name)
				continue
			}

			quota := NewQuota(name)
			finalQuotas[name] = quota
			copied := quota.DeepCopy()
			if exists {
				quotaActions[name] = append(quotaActions[name], fmt.Sprintf("updating %v to %v", name, quota.Spec.Selector))
				quotaWatch.Modify(copied)
			} else {
				quotaActions[name] = append(quotaActions[name], fmt.Sprintf("adding %v to %v", name, quota.Spec.Selector))
				quotaWatch.Add(copied)
			}
		}
		close(finishedQuotas)
	}()

	go func() {
		for i := 0; i < 200; i++ {
			name := namespaceNames[rand.Intn(len(namespaceNames))]
			_, exists := finalNamespaces[name]
			if rand.Intn(50) == 0 {
				if !exists {
					continue
				}
				// due to the compression race (see big comment for impl), clear the queue then delete
				for {
					if len(nsWatch.ResultChan()) == 0 {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}

				namespaceActions[name] = append(namespaceActions[name], "deleting "+name)
				nsWatch.Delete(finalNamespaces[name])
				delete(finalNamespaces, name)
				continue
			}

			ns := NewNamespace(name)
			finalNamespaces[name] = ns
			copied := ns.DeepCopy()
			if exists {
				namespaceActions[name] = append(namespaceActions[name], fmt.Sprintf("updating %v to %v", name, ns.Labels))
				nsWatch.Modify(copied)
			} else {
				namespaceActions[name] = append(namespaceActions[name], fmt.Sprintf("adding %v to %v", name, ns.Labels))
				nsWatch.Add(copied)
			}
		}
		close(finishedNamespaces)
	}()

	<-finishedQuotas
	<-finishedNamespaces

	finalFailures := []string{}
	for i := 0; i < 200; i++ {
		// better suggestions for testing doneness?  Check the condition a few times?
		time.Sleep(50 * time.Millisecond)

		finalFailures = checkState(controller, finalNamespaces, finalQuotas, t, quotaActions, namespaceActions)
		if len(finalFailures) == 0 {
			break
		}
	}

	if len(finalFailures) > 0 {
		t.Logf("have %d quotas and %d namespaces", len(quotaWatch.ResultChan()), len(nsWatch.ResultChan()))
		t.Fatalf("failed on \n%v", strings.Join(finalFailures, "\n"))
	}
}

func checkState(controller *ClusterQuotaMappingController, finalNamespaces map[string]*corev1.Namespace, finalQuotas map[string]*quotav1.ClusterResourceQuota, t *testing.T, quotaActions, namespaceActions map[string][]string) []string {
	failures := []string{}

	quotaToNamespaces := map[string]sets.String{}
	for _, quotaName := range quotaNames {
		quotaToNamespaces[quotaName] = sets.String{}
	}
	namespacesToQuota := map[string]sets.String{}
	for _, namespaceName := range namespaceNames {
		namespacesToQuota[namespaceName] = sets.String{}
	}
	for _, quota := range finalQuotas {
		matcherFunc, err := GetMatcher(quota.Spec.Selector)
		if err != nil {
			t.Fatal(err)
		}
		for _, namespace := range finalNamespaces {
			if matches, _ := matcherFunc(namespace); matches {
				quotaToNamespaces[quota.Name].Insert(namespace.Name)
				namespacesToQuota[namespace.Name].Insert(quota.Name)
			}
		}
	}

	for _, quotaName := range quotaNames {
		namespaces, selector := controller.clusterQuotaMapper.GetNamespacesFor(quotaName)
		nsSet := sets.NewString(namespaces...)
		if !nsSet.Equal(quotaToNamespaces[quotaName]) {
			failures = append(failures, fmt.Sprintf("quota %v, expected %v, got %v", quotaName, quotaToNamespaces[quotaName].List(), nsSet.List()))
			failures = append(failures, quotaActions[quotaName]...)
		}
		if quota, ok := finalQuotas[quotaName]; ok && !reflect.DeepEqual(quota.Spec.Selector, selector) {
			failures = append(failures, fmt.Sprintf("quota %v, expected %v, got %v", quotaName, quota.Spec.Selector, selector))
		}
	}

	for _, namespaceName := range namespaceNames {
		quotas, selectionFields := controller.clusterQuotaMapper.GetClusterQuotasFor(namespaceName)
		quotaSet := sets.NewString(quotas...)
		if !quotaSet.Equal(namespacesToQuota[namespaceName]) {
			failures = append(failures, fmt.Sprintf("namespace %v, expected %v, got %v", namespaceName, namespacesToQuota[namespaceName].List(), quotaSet.List()))
			failures = append(failures, namespaceActions[namespaceName]...)
		}
		if namespace, ok := finalNamespaces[namespaceName]; ok && !reflect.DeepEqual(GetSelectionFields(namespace), selectionFields) {
			failures = append(failures, fmt.Sprintf("namespace %v, expected %v, got %v", namespaceName, GetSelectionFields(namespace), selectionFields))
		}
	}

	return failures
}

func CreateStartingQuotas() []runtime.Object {
	count := rand.Intn(len(quotaNames))
	used := sets.String{}
	ret := []runtime.Object{}

	for i := 0; i < count; i++ {
		name := quotaNames[rand.Intn(len(quotaNames))]
		if !used.Has(name) {
			ret = append(ret, NewQuota(name))
			used.Insert(name)
		}
	}

	return ret
}

func CreateStartingNamespaces() []runtime.Object {
	count := rand.Intn(len(namespaceNames))
	used := sets.String{}
	ret := []runtime.Object{}

	for i := 0; i < count; i++ {
		name := namespaceNames[rand.Intn(len(namespaceNames))]
		if !used.Has(name) {
			ret = append(ret, NewNamespace(name))
			used.Insert(name)
		}
	}

	return ret
}

func NewQuota(name string) *quotav1.ClusterResourceQuota {
	ret := &quotav1.ClusterResourceQuota{}
	ret.Name = name

	numSelectorKeys := rand.Intn(maxSelectorKeys) + 1
	if numSelectorKeys == 0 {
		return ret
	}

	ret.Spec.Selector.LabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
	for i := 0; i < numSelectorKeys; i++ {
		key := keys[rand.Intn(len(keys))]
		value := values[rand.Intn(len(values))]

		ret.Spec.Selector.LabelSelector.MatchLabels[key] = value
	}

	ret.Spec.Selector.AnnotationSelector = map[string]string{}
	for i := 0; i < numSelectorKeys; i++ {
		key := annotationKeys[rand.Intn(len(annotationKeys))]
		value := annotationValues[rand.Intn(len(annotationValues))]

		ret.Spec.Selector.AnnotationSelector[key] = value
	}

	return ret
}

func NewNamespace(name string) *corev1.Namespace {
	ret := &corev1.Namespace{}
	ret.Name = name

	numLabels := rand.Intn(maxLabels) + 1
	if numLabels == 0 {
		return ret
	}

	ret.Labels = map[string]string{}
	for i := 0; i < numLabels; i++ {
		key := keys[rand.Intn(len(keys))]
		value := values[rand.Intn(len(values))]

		ret.Labels[key] = value
	}

	ret.Annotations = map[string]string{}
	for i := 0; i < numLabels; i++ {
		key := annotationKeys[rand.Intn(len(annotationKeys))]
		value := annotationValues[rand.Intn(len(annotationValues))]

		ret.Annotations[key] = value
	}

	return ret
}
