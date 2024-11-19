package certrotation

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// GetCertRotationScale  The normal scale is based on a day.  The value returned by this function
// is used to scale rotation durations instead of a day, so you can set it shorter.
func GetCertRotationScale(ctx context.Context, client kubernetes.Interface, namespace string) (time.Duration, error) {
	certRotationScale := time.Duration(0)
	err := wait.PollImmediate(time.Second, 1*time.Minute, func() (bool, error) {
		certRotationConfig, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, "unsupported-cert-rotation-config", metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		if value, ok := certRotationConfig.Data["base"]; ok {
			certRotationScale, err = time.ParseDuration(value)
			if err != nil {
				return false, err
			}
		}
		return true, nil
	})
	if err != nil {
		return 0, err
	}
	if certRotationScale > 24*time.Hour {
		return 0, fmt.Errorf("scale longer than 24h is not allowed: %v", certRotationScale)
	}
	return certRotationScale, nil
}
