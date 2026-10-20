package cpu_partitioning

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

func shouldCheckPlatformWorkload(ctx context.Context, namespaces coreclient.NamespaceInterface, namespace string, templateAnnotations map[string]string, partitioned bool) (bool, error) {
	platformNamespace := strings.HasPrefix(namespace, "openshift-") && !strings.HasPrefix(namespace, "openshift-e2e-")
	_, annotated := templateAnnotations[workloadAnnotations]
	if platformNamespace && !annotated {
		return false, fmt.Errorf("workload in platform namespace %s must have pod templates annotated with %s", namespace, workloadAnnotations)
	}
	if !partitioned {
		return annotated, nil
	}
	project, err := namespaces.Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) && !platformNamespace {
			return false, nil
		}
		return false, fmt.Errorf("getting workload namespace %s: %w", namespace, err)
	}
	allowsManagement := slices.Contains(strings.Split(project.Annotations[namespaceAnnotationKey], ","), "management")
	if platformNamespace && !allowsManagement {
		return false, fmt.Errorf("platform namespace %s must allow management workloads with %s", namespace, namespaceAnnotationKey)
	}
	return allowsManagement && annotated, nil
}

func deleteNamespaceAndWait(ctx context.Context, namespaces coreclient.NamespaceInterface, namespace string) error {
	if err := namespaces.Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("deleting namespace %s: %w", namespace, err)
	}
	if err := wait.PollUntilContextTimeout(ctx, framework.Poll, framework.DefaultNamespaceDeletionTimeout, true, func(ctx context.Context) (bool, error) {
		_, err := namespaces.Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}); err != nil {
		return fmt.Errorf("waiting for namespace %s deletion: %w", namespace, err)
	}
	return nil
}
