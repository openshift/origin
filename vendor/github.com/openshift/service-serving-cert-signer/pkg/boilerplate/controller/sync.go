package controller

import "k8s.io/apimachinery/pkg/apis/meta/v1"

type Syncer interface {
	Key(namespace, name string) (v1.Object, error)
	Sync(v1.Object) error
}

type SyncFunc func(v1.Object) error
