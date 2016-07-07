package clusterquotamapping

import (
	"sync"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

type ClusterQuotaMapper interface {
	// GetClusterQuotasFor returns the list of clusterquota names that this namespace matches.  It also
	// returns the labels associated with the namespace for the check so that callers can determine staleness
	GetClusterQuotasFor(namespaceName string) ([]string, map[string]string)
	// GetNamespacesFor returns the list of namespace names that this cluster quota matches.  It also
	// returns the selector associated with the clusterquota for the check so that callers can determine staleness
	GetNamespacesFor(quotaName string) ([]string, *unversioned.LabelSelector)

	AddListener(listener MappingChangeListener)
}

// MappingChangeListener is notified of changes to the mapping.  It must not block.
type MappingChangeListener interface {
	AddMapping(quotaName, namespaceName string)
	RemoveMapping(quotaName, namespaceName string)
}

// clusterQuotaMapper gives thread safe access to the actual mappings that are being stored.
// Many method use a shareable read lock to check status followed by a non-shareable
// write lock which double checks the condition before proceding.  Since locks aren't escalatable
// you have to perform the recheck because someone could have beaten you in.
type clusterQuotaMapper struct {
	lock sync.RWMutex

	// requiredQuotaToSelector indicates the latest label selector this controller has observed for a quota
	requiredQuotaToSelector map[string]*unversioned.LabelSelector
	// requiredNamespaceToLabels indicates the latest labels this controller has observed for a namespace
	requiredNamespaceToLabels map[string]map[string]string
	// completedQuotaToSelector indicates the latest label selector this controller has scanned against namespaces
	completedQuotaToSelector map[string]*unversioned.LabelSelector
	// completedNamespaceToLabels indicates the latest labels this controller has scanned against cluster quotas
	completedNamespaceToLabels map[string]map[string]string

	quotaToNamespaces map[string]sets.String
	namespaceToQuota  map[string]sets.String

	listeners []MappingChangeListener
}

func NewClusterQuotaMapper() *clusterQuotaMapper {
	return &clusterQuotaMapper{
		requiredQuotaToSelector:    map[string]*unversioned.LabelSelector{},
		requiredNamespaceToLabels:  map[string]map[string]string{},
		completedQuotaToSelector:   map[string]*unversioned.LabelSelector{},
		completedNamespaceToLabels: map[string]map[string]string{},

		quotaToNamespaces: map[string]sets.String{},
		namespaceToQuota:  map[string]sets.String{},
	}
}

func (m *clusterQuotaMapper) GetClusterQuotasFor(namespaceName string) ([]string, map[string]string) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	quotas, ok := m.namespaceToQuota[namespaceName]
	if !ok {
		return []string{}, m.completedNamespaceToLabels[namespaceName]
	}
	return quotas.List(), m.completedNamespaceToLabels[namespaceName]
}

func (m *clusterQuotaMapper) GetNamespacesFor(quotaName string) ([]string, *unversioned.LabelSelector) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	namespaces, ok := m.quotaToNamespaces[quotaName]
	if !ok {
		return []string{}, m.completedQuotaToSelector[quotaName]
	}
	return namespaces.List(), m.completedQuotaToSelector[quotaName]
}

func (m *clusterQuotaMapper) AddListener(listener MappingChangeListener) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.listeners = append(m.listeners, listener)
}

// requireQuota updates the selector requirements for the given quota.  This prevents stale updates to the mapping itself.
// returns true if a modification was made
func (m *clusterQuotaMapper) requireQuota(quota *quotaapi.ClusterResourceQuota) bool {
	m.lock.RLock()
	selector, exists := m.requiredQuotaToSelector[quota.Name]
	m.lock.RUnlock()

	if selectorMatches(selector, exists, quota) {
		return false
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	selector, exists = m.requiredQuotaToSelector[quota.Name]
	if selectorMatches(selector, exists, quota) {
		return false
	}

	m.requiredQuotaToSelector[quota.Name] = quota.Spec.Selector
	return true
}

// completeQuota updates the latest selector used to generate the mappings for this quota.  The value is returned
// by the Get methods for the mapping so that callers can determine staleness
func (m *clusterQuotaMapper) completeQuota(quota *quotaapi.ClusterResourceQuota) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.completedQuotaToSelector[quota.Name] = quota.Spec.Selector
}

// removeQuota deletes a quota from all mappings
func (m *clusterQuotaMapper) removeQuota(quotaName string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.requiredQuotaToSelector, quotaName)
	delete(m.completedQuotaToSelector, quotaName)
	delete(m.quotaToNamespaces, quotaName)
	for _, quotas := range m.namespaceToQuota {
		quotas.Delete(quotaName)
	}
}

// requireNamespace updates the label requirements for the given namespace.  This prevents stale updates to the mapping itself.
// returns true if a modification was made
func (m *clusterQuotaMapper) requireNamespace(namespace *kapi.Namespace) bool {
	m.lock.RLock()
	labels, exists := m.requiredNamespaceToLabels[namespace.Name]
	m.lock.RUnlock()

	if labelsMatch(labels, exists, namespace) {
		return false
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	labels, exists = m.requiredNamespaceToLabels[namespace.Name]
	if labelsMatch(labels, exists, namespace) {
		return false
	}

	m.requiredNamespaceToLabels[namespace.Name] = namespace.Labels
	return true
}

// completeNamespace updates the latest labels used to generate the mappings for this namespace.  The value is returned
// by the Get methods for the mapping so that callers can determine staleness
func (m *clusterQuotaMapper) completeNamespace(namespace *kapi.Namespace) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.completedNamespaceToLabels[namespace.Name] = namespace.Labels
}

// removeNamespace deletes a namespace from all mappings
func (m *clusterQuotaMapper) removeNamespace(namespaceName string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.requiredNamespaceToLabels, namespaceName)
	delete(m.completedNamespaceToLabels, namespaceName)
	delete(m.namespaceToQuota, namespaceName)
	for _, namespaces := range m.quotaToNamespaces {
		namespaces.Delete(namespaceName)
	}
}

func selectorMatches(selector *unversioned.LabelSelector, exists bool, quota *quotaapi.ClusterResourceQuota) bool {
	return exists && kapi.Semantic.DeepEqual(selector, quota.Spec.Selector)
}
func labelsMatch(labels map[string]string, exists bool, namespace *kapi.Namespace) bool {
	return exists && kapi.Semantic.DeepEqual(labels, namespace.Labels)
}

// setMapping maps (or removes a mapping) between a clusterquota and a namespace
// It returns whether the action worked, whether the quota is out of date, whether the namespace is out of date
// This allows callers to decide whether to pull new information from the cache or simply skip execution
func (m *clusterQuotaMapper) setMapping(quota *quotaapi.ClusterResourceQuota, namespace *kapi.Namespace, remove bool) (bool /*added*/, bool /*quota matches*/, bool /*namespace matches*/) {
	m.lock.RLock()
	selector, selectorExists := m.requiredQuotaToSelector[quota.Name]
	labels, labelsExist := m.requiredNamespaceToLabels[namespace.Name]
	m.lock.RUnlock()

	if !selectorMatches(selector, selectorExists, quota) {
		return false, false, labelsMatch(labels, labelsExist, namespace)
	}
	if !labelsMatch(labels, labelsExist, namespace) {
		return false, true, false
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	selector, selectorExists = m.requiredQuotaToSelector[quota.Name]
	labels, labelsExist = m.requiredNamespaceToLabels[namespace.Name]
	if !selectorMatches(selector, selectorExists, quota) {
		return false, false, labelsMatch(labels, labelsExist, namespace)
	}
	if !labelsMatch(labels, labelsExist, namespace) {
		return false, true, false
	}

	if remove {
		mutated := false

		namespaces, ok := m.quotaToNamespaces[quota.Name]
		if !ok {
			m.quotaToNamespaces[quota.Name] = sets.String{}
		} else {
			mutated = namespaces.Has(namespace.Name)
			namespaces.Delete(namespace.Name)
		}

		quotas, ok := m.namespaceToQuota[namespace.Name]
		if !ok {
			m.namespaceToQuota[namespace.Name] = sets.String{}
		} else {
			mutated = mutated || quotas.Has(quota.Name)
			quotas.Delete(quota.Name)
		}

		if mutated {
			for _, listener := range m.listeners {
				listener.RemoveMapping(quota.Name, namespace.Name)
			}
		}

		return true, true, true
	}

	mutated := false

	namespaces, ok := m.quotaToNamespaces[quota.Name]
	if !ok {
		mutated = true
		m.quotaToNamespaces[quota.Name] = sets.NewString(namespace.Name)
	} else {
		mutated = !namespaces.Has(namespace.Name)
		namespaces.Insert(namespace.Name)
	}

	quotas, ok := m.namespaceToQuota[namespace.Name]
	if !ok {
		mutated = true
		m.namespaceToQuota[namespace.Name] = sets.NewString(quota.Name)
	} else {
		mutated = mutated || !quotas.Has(quota.Name)
		quotas.Insert(quota.Name)
	}

	if mutated {
		for _, listener := range m.listeners {
			listener.AddMapping(quota.Name, namespace.Name)
		}
	}

	return true, true, true

}
