package v1alpha1

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	CatalogSourceCRDAPIVersion  = GroupName + "/" + GroupVersion
	CatalogSourceKind           = "CatalogSource"
	DefaultRegistryPollDuration = 15 * time.Minute
)

// SourceType indicates the type of backing store for a CatalogSource
type SourceType string

const (
	// SourceTypeInternal (deprecated) specifies a CatalogSource of type SourceTypeConfigmap
	SourceTypeInternal SourceType = "internal"

	// SourceTypeConfigmap specifies a CatalogSource that generates a configmap-server registry
	SourceTypeConfigmap SourceType = "configmap"

	// SourceTypeGrpc specifies a CatalogSource that can use an operator registry image to generate a
	// registry-server or connect to a pre-existing registry at an address.
	SourceTypeGrpc SourceType = "grpc"
)

const (
	// CatalogSourceSpecInvalidError denotes when fields on the spec of the CatalogSource are not valid.
	CatalogSourceSpecInvalidError ConditionReason = "SpecInvalidError"
	// CatalogSourceConfigMapError denotes when there is an issue extracting manifests from the specified ConfigMap.
	CatalogSourceConfigMapError ConditionReason = "ConfigMapError"
	// CatalogSourceRegistryServerError denotes when there is an issue querying the specified registry server.
	CatalogSourceRegistryServerError ConditionReason = "RegistryServerError"
	// CatalogSourceIntervalInvalidError denotes if the registry polling interval is invalid.
	CatalogSourceIntervalInvalidError ConditionReason = "InvalidIntervalError"
)

type CatalogSourceSpec struct {
	// SourceType is the type of source
	SourceType SourceType `json:"sourceType"`

	// Priority field assigns a weight to the catalog source to prioritize them so that it can be consumed by the dependency resolver.
	// Usage:
	// Higher weight indicates that this catalog source is preferred over lower weighted catalog sources during dependency resolution.
	// The range of the priority value can go from positive to negative in the range of int32.
	// The default value to a catalog source with unassigned priority would be 0.
	// The catalog source with the same priority values will be ranked lexicographically based on its name.
	// +optional
	Priority int `json:"priority,omitempty"`

	// ConfigMap is the name of the ConfigMap to be used to back a configmap-server registry.
	// Only used when SourceType = SourceTypeConfigmap or SourceTypeInternal.
	// +optional
	ConfigMap string `json:"configMap,omitempty"`

	// Address is a host that OLM can use to connect to a pre-existing registry.
	// Format: <registry-host or ip>:<port>
	// Only used when SourceType = SourceTypeGrpc.
	// Ignored when the Image field is set.
	// +optional
	Address string `json:"address,omitempty"`

	// Image is an operator-registry container image to instantiate a registry-server with.
	// Only used when SourceType = SourceTypeGrpc.
	// If present, the address field is ignored.
	// +optional
	Image string `json:"image,omitempty"`

	// GrpcPodConfig exposes different overrides for the pod spec of the CatalogSource Pod.
	// Only used when SourceType = SourceTypeGrpc and Image is set.
	// +optional
	GrpcPodConfig *GrpcPodConfig `json:"grpcPodConfig,omitempty"`

	// UpdateStrategy defines how updated catalog source images can be discovered
	// Consists of an interval that defines polling duration and an embedded strategy type
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// Secrets represent set of secrets that can be used to access the contents of the catalog.
	// It is best to keep this list small, since each will need to be tried for every catalog entry.
	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// Metadata
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
	Icon        Icon   `json:"icon,omitempty"`
}

type SecurityConfig string

const (
	Legacy     SecurityConfig = "legacy"
	Restricted SecurityConfig = "restricted"
)

// GrpcPodConfig contains configuration specified for a catalog source
type GrpcPodConfig struct {
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations are the catalog source's pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity is the catalog source's pod's affinity.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// If specified, indicates the pod's priority.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`

	// SecurityContextConfig can be one of `legacy` or `restricted`. The CatalogSource's pod is either injected with the
	// right pod.spec.securityContext and pod.spec.container[*].securityContext values to allow the pod to run in Pod
	// Security Admission (PSA) `restricted` mode, or doesn't set these values at all, in which case the pod can only be
	// run in PSA `baseline` or `privileged` namespaces. If the SecurityContextConfig is unspecified, the mode will be
	// determined by the namespace's PSA configuration. If the namespace is enforcing `restricted` mode, then the pod
	// will be configured as if `restricted` was specified. Otherwise, it will be configured as if `legacy` was
	// specified. Specifying a value other than `legacy` or `restricted` result in a validation error. When using older
	// catalog images, which can not run in `restricted` mode, the SecurityContextConfig should be set to `legacy`.
	//
	// More information about PSA can be found here: https://kubernetes.io/docs/concepts/security/pod-security-admission/
	// +optional
	// +kubebuilder:validation:Enum=legacy;restricted
	SecurityContextConfig SecurityConfig `json:"securityContextConfig,omitempty"`

	// MemoryTarget configures the $GOMEMLIMIT value for the gRPC catalog Pod. This is a soft memory limit for the server,
	// which the runtime will attempt to meet but makes no guarantees that it will do so. If this value is set, the Pod
	// will have the following modifications made to the container running the server:
	// - the $GOMEMLIMIT environment variable will be set to this value in bytes
	// - the memory request will be set to this value
	//
	// This field should be set if it's desired to reduce the footprint of a catalog server as much as possible, or if
	// a catalog being served is very large and needs more than the default allocation. If your index image has a file-
	// system cache, determine a good approximation for this value by doubling the size of the package cache at
	// /tmp/cache/cache/packages.json in the index image.
	//
	// This field is best-effort; if unset, no default will be used and no Pod memory limit or $GOMEMLIMIT value will be set.
	// +optional
	MemoryTarget *resource.Quantity `json:"memoryTarget,omitempty"`

	// ExtractContent configures the gRPC catalog Pod to extract catalog metadata from the provided index image and
	// use a well-known version of the `opm` server to expose it. The catalog index image that this CatalogSource is
	// configured to use *must* be using the file-based catalogs in order to utilize this feature.
	// +optional
	ExtractContent *ExtractContentConfig `json:"extractContent,omitempty"`
}

// ExtractContentConfig configures context extraction from a file-based catalog index image.
type ExtractContentConfig struct {
	// CacheDir is the (optional) directory storing the pre-calculated API cache.
	CacheDir string `json:"cacheDir,omitempty"`
	// CatalogDir is the directory storing the file-based catalog contents.
	CatalogDir string `json:"catalogDir"`
}

// UpdateStrategy holds all the different types of catalog source update strategies
// Currently only registry polling strategy is implemented
type UpdateStrategy struct {
	*RegistryPoll `json:"registryPoll,omitempty"`
}

type RegistryPoll struct {
	// Interval is used to determine the time interval between checks of the latest catalog source version.
	// The catalog operator polls to see if a new version of the catalog source is available.
	// If available, the latest image is pulled and gRPC traffic is directed to the latest catalog source.
	RawInterval  string           `json:"interval,omitempty"`
	Interval     *metav1.Duration `json:"-"`
	ParsingError string           `json:"-"`
}

// UnmarshalJSON implements the encoding/json.Unmarshaler interface.
func (u *UpdateStrategy) UnmarshalJSON(data []byte) (err error) {
	type alias struct {
		*RegistryPoll `json:"registryPoll,omitempty"`
	}
	us := alias{}
	if err = json.Unmarshal(data, &us); err != nil {
		return err
	}
	registryPoll := &RegistryPoll{
		RawInterval: us.RegistryPoll.RawInterval,
	}
	duration, err := time.ParseDuration(registryPoll.RawInterval)
	if err != nil {
		registryPoll.ParsingError = fmt.Sprintf("error parsing spec.updateStrategy.registryPoll.interval. Using the default value of %s instead. Error: %s", DefaultRegistryPollDuration, err)
		registryPoll.Interval = &metav1.Duration{Duration: DefaultRegistryPollDuration}
	} else {
		registryPoll.Interval = &metav1.Duration{Duration: duration}
	}
	u.RegistryPoll = registryPoll
	return nil
}

type RegistryServiceStatus struct {
	Protocol         string      `json:"protocol,omitempty"`
	ServiceName      string      `json:"serviceName,omitempty"`
	ServiceNamespace string      `json:"serviceNamespace,omitempty"`
	Port             string      `json:"port,omitempty"`
	CreatedAt        metav1.Time `json:"createdAt,omitempty"`
}

func (s *RegistryServiceStatus) Address() string {
	return fmt.Sprintf("%s.%s.svc:%s", s.ServiceName, s.ServiceNamespace, s.Port)
}

type GRPCConnectionState struct {
	Address           string      `json:"address,omitempty"`
	LastObservedState string      `json:"lastObservedState"`
	LastConnectTime   metav1.Time `json:"lastConnect,omitempty"`
}

type CatalogSourceStatus struct {
	// A human readable message indicating details about why the CatalogSource is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// Reason is the reason the CatalogSource was transitioned to its current state.
	// +optional
	Reason ConditionReason `json:"reason,omitempty"`

	// The last time the CatalogSource image registry has been polled to ensure the image is up-to-date
	LatestImageRegistryPoll *metav1.Time `json:"latestImageRegistryPoll,omitempty"`

	// ConfigMapReference (deprecated) is the reference to the ConfigMap containing the catalog source's configuration, when the catalog source is a ConfigMap
	ConfigMapResource *ConfigMapResourceReference `json:"configMapReference,omitempty"`
	// RegistryService represents the current state of the GRPC service used to serve the catalog
	RegistryServiceStatus *RegistryServiceStatus `json:"registryService,omitempty"`
	// ConnectionState represents the current state of the CatalogSource's connection to the registry
	GRPCConnectionState *GRPCConnectionState `json:"connectionState,omitempty"`

	// Represents the state of a CatalogSource. Note that Message and Reason represent the original
	// status information, which may be migrated to be conditions based in the future. Any new features
	// introduced will use conditions.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

type ConfigMapResourceReference struct {
	Name            string      `json:"name"`
	Namespace       string      `json:"namespace"`
	UID             types.UID   `json:"uid,omitempty"`
	ResourceVersion string      `json:"resourceVersion,omitempty"`
	LastUpdateTime  metav1.Time `json:"lastUpdateTime,omitempty"`
}

func (r *ConfigMapResourceReference) IsAMatch(object *metav1.ObjectMeta) bool {
	return r.UID == object.GetUID() && r.ResourceVersion == object.GetResourceVersion()
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:resource:shortName=catsrc,categories=olm
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display",type=string,JSONPath=`.spec.displayName`,description="The pretty name of the catalog"
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.sourceType`,description="The type of the catalog"
// +kubebuilder:printcolumn:name="Publisher",type=string,JSONPath=`.spec.publisher`,description="The publisher of the catalog"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CatalogSource is a repository of CSVs, CRDs, and operator packages.
type CatalogSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec CatalogSourceSpec `json:"spec"`
	// +optional
	Status CatalogSourceStatus `json:"status"`
}

func (c *CatalogSource) Address() string {
	if c.Spec.Address != "" {
		return c.Spec.Address
	}
	return c.Status.RegistryServiceStatus.Address()
}

func (c *CatalogSource) SetError(reason ConditionReason, err error) {
	c.Status.Reason = reason
	c.Status.Message = ""
	if err != nil {
		c.Status.Message = err.Error()
	}
}

func (c *CatalogSource) SetLastUpdateTime() {
	now := metav1.Now()
	c.Status.LatestImageRegistryPoll = &now
}

// Check if it is time to update based on polling setting
func (c *CatalogSource) Update() bool {
	if !c.Poll() {
		return false
	}
	interval := c.Spec.UpdateStrategy.Interval.Duration
	latest := c.Status.LatestImageRegistryPoll
	if latest == nil {
		logrus.WithField("CatalogSource", c.Name).Debugf("latest poll %v", latest)
	} else {
		logrus.WithField("CatalogSource", c.Name).Debugf("latest poll %v", *c.Status.LatestImageRegistryPoll)
	}

	if c.Status.LatestImageRegistryPoll.IsZero() {
		logrus.WithField("CatalogSource", c.Name).Debugf("creation timestamp plus interval before now %t", c.CreationTimestamp.Add(interval).Before(time.Now()))
		if c.CreationTimestamp.Add(interval).Before(time.Now()) {
			return true
		}
	} else {
		logrus.WithField("CatalogSource", c.Name).Debugf("latest poll plus interval before now %t", c.Status.LatestImageRegistryPoll.Add(interval).Before(time.Now()))
		if c.Status.LatestImageRegistryPoll.Add(interval).Before(time.Now()) {
			return true
		}
	}

	return false
}

// Poll determines whether the polling feature is enabled on the particular catalog source
func (c *CatalogSource) Poll() bool {
	if c.Spec.UpdateStrategy == nil {
		return false
	}
	// if polling interval is zero polling will not be done
	if c.Spec.UpdateStrategy.RegistryPoll == nil {
		return false
	}
	// if catalog source is not backed by an image polling will not be done
	if c.Spec.Image == "" {
		return false
	}
	// if image is not type gRPC polling will not be done
	if c.Spec.SourceType != SourceTypeGrpc {
		return false
	}
	return true
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CatalogSourceList is a repository of CSVs, CRDs, and operator packages.
type CatalogSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CatalogSource `json:"items"`
}
