package storage

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager"
	"sigs.k8s.io/structured-merge-diff/v4/typed"
)

var typeConverter fieldmanager.TypeConverter = fieldmanager.DeducedTypeConverter{}

func modifiedFields(oldRuntimeObject, newRuntimeObject *unstructured.Unstructured) (*typed.Comparison, error) {
	oldObject, err := typeConverter.ObjectToTyped(oldRuntimeObject)
	if err != nil {
		return nil, fmt.Errorf("failed to convert live object (%v) to smd typed: %v", objectGVKNN(oldRuntimeObject), err)
	}
	newObject, err := typeConverter.ObjectToTyped(newRuntimeObject)
	if err != nil {
		return nil, fmt.Errorf("failed to convert new object (%v) to smd typed: %v", objectGVKNN(newRuntimeObject), err)
	}

	compare, err := oldObject.Compare(newObject)
	if err != nil {
		return nil, fmt.Errorf("failed to compare objects: %v", err)
	}

	return compare, nil
}

func whichUsersOwnModifiedFields(obj *unstructured.Unstructured, comparison typed.Comparison) ([]string, error) {
	users := sets.NewString()

	managers, err := fieldmanager.DecodeManagedFields(obj.GetManagedFields())
	if err != nil {
		return nil, err
	}

	for manager, managerSet := range managers.Fields() {
		setByThisManager := managerSet.Set().Intersection(comparison.Modified.Union(comparison.Added).Union(comparison.Removed))
		if !setByThisManager.Empty() {
			users.Insert(manager)
			continue
		}
	}

	return users.List(), nil
}

func objectGVKNN(obj runtime.Object) string {
	name := "<unknown>"
	namespace := "<unknown>"
	if accessor, err := meta.Accessor(obj); err == nil {
		name = accessor.GetName()
		namespace = accessor.GetNamespace()
	}

	return fmt.Sprintf("%v/%v; %v", namespace, name, obj.GetObjectKind().GroupVersionKind())
}
