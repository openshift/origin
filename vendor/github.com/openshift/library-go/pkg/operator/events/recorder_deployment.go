package events

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/reference"
)

// eventSourceReplicaSetNameEnv is a name of environment variable inside container that specifies the name of the current replica set.
// This replica set name is then used as a source/involved object for operator events.
const eventSourceReplicaSetNameEnv = "EVENT_SOURCE_REPLICASET_NAME"

// eventSourceReplicaSetNameEnvFunc allows to override the way we get the environment variable value (for unit tests).
var eventSourceReplicaSetNameEnvFunc = func() string {
	return os.Getenv(eventSourceReplicaSetNameEnv)
}

// GetReplicaSetOwnerReference returns an object reference for a replica set specified in EVENT_SOURCE_REPLICASET_NAME environment variable.
// This object reference can be used as involvedObjectRef in event recorder. This method should be called once, in factory.
func GetReplicaSetOwnerReference(client appsv1client.ReplicaSetInterface) (*corev1.ObjectReference, error) {
	replicaSetName := eventSourceReplicaSetNameEnvFunc()
	if len(replicaSetName) == 0 {
		return nil, fmt.Errorf("unable to setup event recorder as %q env variable is not set", eventSourceReplicaSetNameEnv)
	}
	rs, err := client.Get(replicaSetName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return reference.GetReference(nil, rs)
}
