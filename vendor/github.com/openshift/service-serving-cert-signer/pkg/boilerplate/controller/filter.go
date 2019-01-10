package controller

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ParentFilter interface {
	Parent(obj v1.Object) (namespace, name string)
	Filter
}

type Filter interface {
	Add(obj v1.Object) bool
	Update(oldObj, newObj v1.Object) bool
	Delete(obj v1.Object) bool
}

type ParentFunc func(obj v1.Object) (namespace, name string)

type FilterFuncs struct {
	ParentFunc ParentFunc
	AddFunc    func(obj v1.Object) bool
	UpdateFunc func(oldObj, newObj v1.Object) bool
	DeleteFunc func(obj v1.Object) bool
}

func (f FilterFuncs) Parent(obj v1.Object) (namespace, name string) {
	if f.ParentFunc == nil {
		return obj.GetNamespace(), obj.GetName()
	}
	return f.ParentFunc(obj)
}

func (f FilterFuncs) Add(obj v1.Object) bool {
	if f.AddFunc == nil {
		return false
	}
	return f.AddFunc(obj)
}

func (f FilterFuncs) Update(oldObj, newObj v1.Object) bool {
	if f.UpdateFunc == nil {
		return false
	}
	return f.UpdateFunc(oldObj, newObj)
}

func (f FilterFuncs) Delete(obj v1.Object) bool {
	if f.DeleteFunc == nil {
		return false
	}
	return f.DeleteFunc(obj)
}

func FilterByNames(parentFunc ParentFunc, names ...string) ParentFilter {
	set := sets.NewString(names...)
	has := func(obj v1.Object) bool {
		return set.Has(obj.GetName())
	}
	return FilterFuncs{
		ParentFunc: parentFunc,
		AddFunc:    has,
		UpdateFunc: func(oldObj, newObj v1.Object) bool {
			return has(newObj)
		},
		DeleteFunc: has,
	}
}
