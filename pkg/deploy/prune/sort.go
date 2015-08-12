package prune

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// sortableReplicationControllers supports sorting ReplicationController items by most recently created
type sortableReplicationControllers []*kapi.ReplicationController

func (s sortableReplicationControllers) Len() int {
	return len(s)
}

func (s sortableReplicationControllers) Less(i, j int) bool {
	return !s[i].CreationTimestamp.Before(s[j].CreationTimestamp)
}

func (s sortableReplicationControllers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
