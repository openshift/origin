package controller

import "k8s.io/apimachinery/pkg/apis/meta/v1"

type Filter interface {
	Parent(obj v1.Object) (name string)
	Add(obj v1.Object) bool
	Update(oldObj, newObj v1.Object) bool
	Delete(obj v1.Object) bool
}

type FilterFuncs struct {
	ParentFunc func(obj v1.Object) (name string)
	AddFunc    func(obj v1.Object) bool
	UpdateFunc func(oldObj, newObj v1.Object) bool
	DeleteFunc func(obj v1.Object) bool
}

func (f FilterFuncs) Parent(obj v1.Object) string {
	if f.ParentFunc == nil {
		return obj.GetName()
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
