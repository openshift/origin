package helpers

import (
	daemonconsts "github.com/openshift/machine-config-operator/pkg/daemon/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeBuilder provides a more fluent API for creating Node objects for tests.
// Instead of creating multiple specific functions for each particular node
// configuration, this allows one to create nodes thusly:
//
// NewNodeBuilder("node-0").WithCurrentConfig(currentConfig).WithDesiredConfig(desiredConfig).Node()
type NodeBuilder struct {
	node          *corev1.Node
	currentConfig string
	desiredConfig string
	currentImage  string
	desiredImage  string
	mcdState      string
}

func NewNodeBuilder(name string) *NodeBuilder {
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	return &NodeBuilder{node: n}
}

func (n *NodeBuilder) WithEqualConfigsAndImages(config, image string) *NodeBuilder {
	return n.WithEqualConfigs(config).WithEqualImages(image)
}

func (n *NodeBuilder) WithEqualImages(image string) *NodeBuilder {
	return n.WithImages(image, image)
}

func (n *NodeBuilder) WithEqualConfigs(config string) *NodeBuilder {
	return n.WithConfigs(config, config)
}

func (n *NodeBuilder) WithAnnotations(annos map[string]string) *NodeBuilder {
	if n.node.Annotations == nil {
		n.node.Annotations = map[string]string{}
	}

	for k, v := range annos {
		if k == daemonconsts.MachineConfigDaemonStateAnnotationKey {
			n.mcdState = v
		}

		if k == daemonconsts.CurrentImageAnnotationKey {
			n.currentImage = v
		}

		if k == daemonconsts.DesiredImageAnnotationKey {
			n.desiredImage = v
		}

		if k == daemonconsts.CurrentMachineConfigAnnotationKey {
			n.currentConfig = v
		}

		if k == daemonconsts.DesiredMachineConfigAnnotationKey {
			n.desiredConfig = v
		}

		n.node.Annotations[k] = v
	}

	return n
}

func (n *NodeBuilder) WithTaint(taint corev1.Taint) *NodeBuilder {
	if n.node.Spec.Taints == nil {
		n.node.Spec.Taints = []corev1.Taint{}
	}

	n.node.Spec.Taints = append(n.node.Spec.Taints, taint)
	return n
}

func (n *NodeBuilder) WithConfigsAndImages(currentConfig, desiredConfig, currentImage, desiredImage string) *NodeBuilder {
	return n.WithConfigs(currentConfig, desiredConfig).WithImages(currentImage, desiredImage)
}

func (n *NodeBuilder) WithConfigs(current, desired string) *NodeBuilder {
	return n.WithCurrentConfig(current).WithDesiredConfig(desired)
}

func (n *NodeBuilder) WithCurrentConfig(current string) *NodeBuilder {
	return n.WithAnnotations(map[string]string{
		daemonconsts.CurrentMachineConfigAnnotationKey: current,
	})
}

func (n *NodeBuilder) WithDesiredConfig(desired string) *NodeBuilder {
	return n.WithAnnotations(map[string]string{
		daemonconsts.DesiredMachineConfigAnnotationKey: desired,
	})
}

func (n *NodeBuilder) WithImages(current, desired string) *NodeBuilder {
	return n.WithCurrentImage(current).WithDesiredImage(desired)
}

func (n *NodeBuilder) WithCurrentImage(current string) *NodeBuilder {
	return n.WithAnnotations(map[string]string{
		daemonconsts.CurrentImageAnnotationKey: current,
	})
}

func (n *NodeBuilder) WithDesiredImage(desired string) *NodeBuilder {
	return n.WithAnnotations(map[string]string{
		daemonconsts.DesiredImageAnnotationKey: desired,
	})
}

func (n *NodeBuilder) WithMCDState(state string) *NodeBuilder {
	return n.WithAnnotations(map[string]string{
		daemonconsts.MachineConfigDaemonStateAnnotationKey: state,
	})
}

func (n *NodeBuilder) WithLabels(labels map[string]string) *NodeBuilder {
	if n.node.Labels == nil {
		n.node.Labels = map[string]string{}
	}

	for k, v := range labels {
		n.node.Labels[k] = v
	}

	return n
}

func (n *NodeBuilder) WithNodeReady() *NodeBuilder {
	return n.withNodeReady(corev1.ConditionTrue)
}

func (n *NodeBuilder) WithNodeNotReady() *NodeBuilder {
	return n.withNodeReady(corev1.ConditionFalse)
}

func (n *NodeBuilder) withNodeReady(status corev1.ConditionStatus) *NodeBuilder {
	return n.WithStatus(corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}})
}

func (n *NodeBuilder) WithStatus(s corev1.NodeStatus) *NodeBuilder {
	n.node.Status = s
	return n
}

func (n *NodeBuilder) Node() *corev1.Node {
	if n.mcdState != "" {
		return n.node
	}

	if n.currentConfig != "" || n.desiredConfig != "" || n.currentImage != "" || n.desiredImage != "" {
		var state string
		if n.currentImage == n.desiredImage && n.currentConfig == n.desiredConfig {
			state = daemonconsts.MachineConfigDaemonStateDone
		} else {
			state = daemonconsts.MachineConfigDaemonStateWorking
		}
		n.node.Annotations[daemonconsts.MachineConfigDaemonStateAnnotationKey] = state
	}

	return n.node.DeepCopy()
}
