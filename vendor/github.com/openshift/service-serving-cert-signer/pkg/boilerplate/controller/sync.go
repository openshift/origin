package controller

import "k8s.io/apimachinery/pkg/apis/meta/v1"

type KeySyncer interface {
	Key(namespace, name string) (v1.Object, error)
	Syncer
}

type Syncer interface {
	Sync(v1.Object) error
}

type SyncFunc func(v1.Object) error
