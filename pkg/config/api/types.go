package api

import kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

// Config implements the Kubernetes api.List
// DEPRECATED: The version v1beta2 should use api.List instead of Config
type Config kapi.List
