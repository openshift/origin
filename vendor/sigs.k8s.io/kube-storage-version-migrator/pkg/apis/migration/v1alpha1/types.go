/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// StorageVersionMigration represents a migration of stored data to the latest
// storage version.
type StorageVersionMigration struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the migration.
	// +optional
	Spec StorageVersionMigrationSpec `json:"spec,omitempty"`
	// Status of the migration.
	// +optional
	Status StorageVersionMigrationStatus `json:"status,omitempty"`
}

// The names of the group, the version, and the resource.
type GroupVersionResource struct {
	// The name of the group.
	Group string `json:"group,omitempty"`
	// The name of the version.
	Version string `json:"version,omitempty"`
	// The name of the resource.
	Resource string `json:"resource,omitempty"`
}

// Spec of the storage version migration.
type StorageVersionMigrationSpec struct {
	// The resource that is being migrated. The migrator sends requests to
	// the endpoint serving the resource.
	// Immutable.
	Resource GroupVersionResource `json:"resource"`
	// The token used in the list options to get the next chunk of objects
	// to migrate. When the .status.conditions indicates the migration is
	// "Running", users can use this token to check the progress of the
	// migration.
	// +optional
	ContinueToken string `json:"continueToken,omitempty"`
	// TODO: consider recording the storage version hash when the migration
	// is created. It can avoid races.
}

type MigrationConditionType string

const (
	// Indicates that the migration is running.
	MigrationRunning MigrationConditionType = "Running"
	// Indicates that the migration has completed successfully.
	MigrationSucceeded MigrationConditionType = "Succeeded"
	// Indicates that the migration has failed.
	MigrationFailed MigrationConditionType = "Failed"
)

// Describes the state of a migration at a certain point.
type MigrationCondition struct {
	// Type of the condition.
	Type MigrationConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// Status of the storage version migration.
type StorageVersionMigrationStatus struct {
	// The latest available observations of the migration's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []MigrationCondition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StorageVersionMigrationList is a collection of storage version migrations.
type StorageVersionMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of StorageVersionMigration
	Items []StorageVersionMigration `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// The state of the storage of a specific resource.
type StorageState struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the storage state.
	// +optional
	Spec StorageStateSpec `json:"spec,omitempty"`
	// Status of the storage state.
	// +optional
	Status StorageStateStatus `json:"status,omitempty"`
}

// The names of the group and the resource.
type GroupResource struct {
	// The name of the group.
	Group string `json:"group,omitempty"`
	// The name of the resource.
	Resource string `json:"resource,omitempty"`
}

// Specification of the storage state.
type StorageStateSpec struct {
	// The resource this storageState is about.
	Resource GroupResource `json:"resource,omitempty"`
}

// Unknown is a valid value in persistedStorageVersionHashes.
const Unknown = "Unknown"

// Status of the storage state.
type StorageStateStatus struct {
	// The hash values of storage versions that persisted instances of
	// spec.resource might still be encoded in.
	// "Unknown" is a valid value in the list, and is the default value.
	// It is not safe to upgrade or downgrade to an apiserver binary that does not
	// support all versions listed in this field, or if "Unknown" is listed.
	// Once the storage version migration for this resource has completed, the
	// value of this field is refined to only contain the
	// currentStorageVersionHash.
	// Once the apiserver has changed the storage version, the new storage version
	// is appended to the list.
	// +optional
	PersistedStorageVersionHashes []string `json:"persistedStorageVersionHashes,omitempty"`
	// The hash value of the current storage version, as shown in the discovery
	// document served by the API server.
	// Storage Version is the version to which objects are converted to
	// before persisted.
	// +optional
	CurrentStorageVersionHash string `json:"currentStorageVersionHash,omitempty"`
	// LastHeartbeatTime is the last time the storage migration triggering
	// controller checks the storage version hash of this resource in the
	// discovery document and updates this field.
	// +optional
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StorageStateList is a collection of storage state.
type StorageStateList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of StorageState
	Items []StorageState `json:"items"`
}
