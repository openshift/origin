package operators

import (
	"context"
	"fmt"
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/api/equality"

	applyoperatorv1 "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// TODO replace with the library-go impl once created
type dynamicOperatorClient struct {
	gvk        schema.GroupVersionKind
	configName string
	client     dynamic.ResourceInterface

	extractApplySpec   StaticPodOperatorSpecExtractorFunc
	extractApplyStatus StaticPodOperatorStatusExtractorFunc
}

type StaticPodOperatorSpecExtractorFunc func(obj *unstructured.Unstructured, fieldManager string) (*applyoperatorv1.StaticPodOperatorSpecApplyConfiguration, error)
type StaticPodOperatorStatusExtractorFunc func(obj *unstructured.Unstructured, fieldManager string) (*applyoperatorv1.StaticPodOperatorStatusApplyConfiguration, error)
type OperatorSpecExtractorFunc func(obj *unstructured.Unstructured, fieldManager string) (*applyoperatorv1.OperatorSpecApplyConfiguration, error)
type OperatorStatusExtractorFunc func(obj *unstructured.Unstructured, fieldManager string) (*applyoperatorv1.OperatorStatusApplyConfiguration, error)

func convertOperatorSpecToStaticPodOperatorSpec(extractApplySpec OperatorSpecExtractorFunc) StaticPodOperatorSpecExtractorFunc {
	return func(obj *unstructured.Unstructured, fieldManager string) (*applyoperatorv1.StaticPodOperatorSpecApplyConfiguration, error) {
		operatorSpec, err := extractApplySpec(obj, fieldManager)
		if err != nil {
			return nil, err
		}
		if operatorSpec == nil {
			return nil, nil
		}
		return &applyoperatorv1.StaticPodOperatorSpecApplyConfiguration{
			OperatorSpecApplyConfiguration: *operatorSpec,
		}, nil
	}
}

func convertOperatorStatusToStaticPodOperatorStatus(extractApplyStatus OperatorStatusExtractorFunc) StaticPodOperatorStatusExtractorFunc {
	return func(obj *unstructured.Unstructured, fieldManager string) (*applyoperatorv1.StaticPodOperatorStatusApplyConfiguration, error) {
		operatorStatus, err := extractApplyStatus(obj, fieldManager)
		if err != nil {
			return nil, err
		}
		if operatorStatus == nil {
			return nil, nil
		}
		return &applyoperatorv1.StaticPodOperatorStatusApplyConfiguration{
			OperatorStatusApplyConfiguration: *operatorStatus,
		}, nil
	}
}

func (c dynamicOperatorClient) ApplyOperatorSpec(ctx context.Context, fieldManager string, applyConfiguration *applyoperatorv1.OperatorSpecApplyConfiguration) (err error) {
	applyMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(applyConfiguration)
	if err != nil {
		return fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	return c.applyOperatorSpec(ctx, fieldManager, applyMap)
}

func (c dynamicOperatorClient) applyOperatorSpec(ctx context.Context, fieldManager string, applyMap map[string]interface{}) (err error) {
	applyUnstructured := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": applyMap,
		},
	}
	applyUnstructured.SetGroupVersionKind(c.gvk)
	applyUnstructured.SetName(c.configName)

	original, err := c.client.Get(ctx, c.configName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		// do nothing and proceed with the apply
	case err != nil:
		return fmt.Errorf("unable to read existing %q: %w", c.configName, err)
	default:
		if c.extractApplySpec == nil {
			return fmt.Errorf("extractApplySpec is nil")
		}
		previouslySetFields, err := c.extractApplySpec(original, fieldManager)
		if err != nil {
			return fmt.Errorf("unable to extract spec for %q: %w", fieldManager, err)
		}
		currentApplyConfiguration := &applyoperatorv1.StaticPodOperatorSpecApplyConfiguration{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(applyMap, applyUnstructured); err != nil {
			return fmt.Errorf("unable to convert to staticpodoperatorspec: %w", err)
		}
		if equality.Semantic.DeepEqual(previouslySetFields, currentApplyConfiguration) {
			// nothing to apply, so return early
			return nil
		}
	}

	_, err = c.client.Apply(ctx, c.configName, applyUnstructured, metav1.ApplyOptions{
		Force:        true,
		FieldManager: fieldManager,
	})
	if err != nil {
		return fmt.Errorf("unable to Apply for operator using fieldManager %q: %w", fieldManager, err)
	}

	return nil
}

func (c dynamicOperatorClient) ApplyOperatorStatus(ctx context.Context, fieldManager string, applyConfiguration *applyoperatorv1.OperatorStatusApplyConfiguration) (string, error) {
	if applyConfiguration == nil {
		return "no input", fmt.Errorf("desired status must have value")
	}
	desiredConfiguration := applyoperatorv1.StaticPodOperatorStatus()
	desiredConfiguration.OperatorStatusApplyConfiguration = *applyConfiguration
	return c.applyOperatorStatus(ctx, fieldManager, desiredConfiguration)
}

func (c dynamicOperatorClient) applyOperatorStatus(ctx context.Context, fieldManager string, desiredConfiguration *applyoperatorv1.StaticPodOperatorStatusApplyConfiguration) (string, error) {
	applyMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(desiredConfiguration)
	if err != nil {
		return "", fmt.Errorf("failed to convert to unstructured: %w", err)
	}
	applyUnstructured := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": applyMap,
		},
	}
	applyUnstructured.SetGroupVersionKind(c.gvk)
	applyUnstructured.SetName(c.configName)

	original, err := c.client.Get(ctx, c.configName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		// do nothing and proceed with the apply
	case err != nil:
		return "", fmt.Errorf("unable to read existing %q: %w", c.configName, err)
	default:
		if c.extractApplyStatus == nil {
			return "", fmt.Errorf("extractApplyStatus is nil")
		}
		previouslySetFields, err := c.extractApplyStatus(original, fieldManager)
		if err != nil {
			return "", fmt.Errorf("unable to extract status for %q: %w", fieldManager, err)
		}
		if equality.Semantic.DeepEqual(previouslySetFields, desiredConfiguration) {
			// nothing to apply, so return early
			return "nothing to apply", nil
		}
	}

	_, err = c.client.ApplyStatus(ctx, c.configName, applyUnstructured, metav1.ApplyOptions{
		Force:        true,
		FieldManager: fieldManager,
	})
	if err != nil {
		return "modification attempted", fmt.Errorf("unable to ApplyStatus for operator using fieldManager %q: %w", fieldManager, err)
	}

	return "modification done", nil
}

func extractOperatorSpec(obj *unstructured.Unstructured, fieldManager string) (*applyoperatorv1.OperatorSpecApplyConfiguration, error) {
	castObj := &operatorv1.OpenShiftAPIServer{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, castObj); err != nil {
		return nil, fmt.Errorf("unable to convert to OpenShiftAPIServer: %w", err)
	}
	ret, err := applyoperatorv1.ExtractOpenShiftAPIServer(castObj, fieldManager)
	if err != nil {
		return nil, fmt.Errorf("unable to extract fields for %q: %w", fieldManager, err)
	}
	if ret.Spec == nil {
		return nil, nil
	}
	return &ret.Spec.OperatorSpecApplyConfiguration, nil
}

func extractOperatorStatus(obj *unstructured.Unstructured, fieldManager string) (*applyoperatorv1.OperatorStatusApplyConfiguration, error) {
	castObj := &operatorv1.OpenShiftAPIServer{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, castObj); err != nil {
		return nil, fmt.Errorf("unable to convert to OpenShiftAPIServer: %w", err)
	}
	ret, err := applyoperatorv1.ExtractOpenShiftAPIServerStatus(castObj, fieldManager)
	if err != nil {
		return nil, fmt.Errorf("unable to extract fields for %q: %w", fieldManager, err)
	}

	if ret.Status == nil {
		return nil, nil
	}
	return &ret.Status.OperatorStatusApplyConfiguration, nil
}
