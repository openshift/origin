package commatrix

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/liornoy/node-comm-lib/pkg/client"
	"github.com/liornoy/node-comm-lib/pkg/endpointslices"
	"github.com/liornoy/node-comm-lib/pkg/types"
)

type Env int

const (
	Baremetal Env = iota
	AWS
)

// New initializes a ComMatrix using Kubernetes cluster data.
// It takes kubeconfigPath for cluster access to  fetch EndpointSlice objects,
// detailing open ports for ingress traffic.
// customEntriesPath allows adding custom entries from a JSON file to the matrix.
// Returns a pointer to ComMatrix and error. Entries include traffic direction, protocol,
// port number, namespace, service name, pod, container, node role, and flow optionality for OpenShift.
func New(kubeconfigPath string, customEntriesPath string, e Env) (*types.ComMatrix, error) {
	res := make([]types.ComDetails, 0)

	cs, err := client.New(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed creating the k8s client: %w", err)
	}

	epSlicesInfo, err := endpointslices.GetIngressEndpointSlicesInfo(cs)
	if err != nil {
		return nil, fmt.Errorf("failed getting endpointslices: %w", err)
	}

	epSliceComDetails, err := endpointslices.ToComDetails(cs, epSlicesInfo)
	if err != nil {
		return nil, err
	}
	res = append(res, epSliceComDetails...)

	staticEntries, err := getStaticEntries(e)
	if err != nil {
		return nil, err
	}

	res = append(res, staticEntries...)

	if customEntriesPath != "" {
		customComDetails, err := addFromFile(customEntriesPath)
		if err != nil {
			return nil, fmt.Errorf("failed fetching custom entries from file %s err: %w", customEntriesPath, err)
		}

		res = append(res, customComDetails...)
	}

	return &types.ComMatrix{Matrix: res}, nil
}

func addFromFile(fp string) ([]types.ComDetails, error) {
	var res []types.ComDetails
	f, err := os.Open(filepath.Clean(fp))
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %v", fp, err)
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", fp, err)
	}

	err = json.Unmarshal(raw, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal custom entries file: %v", err)
	}

	return res, nil
}

func getStaticEntries(e Env) ([]types.ComDetails, error) {
	var (
		envComDetails     []types.ComDetails
		genericComDetails []types.ComDetails
	)
	switch e {
	case Baremetal:
		err := json.Unmarshal([]byte(baremetalStaticEntries), &envComDetails)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal static entries: %v", err)
		}
	case AWS:
		err := json.Unmarshal([]byte(awsCloudStaticEntries), &envComDetails)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal static entries: %v", err)
		}
	default:
		return nil, fmt.Errorf("invalid value for cluster environment")
	}

	err := json.Unmarshal([]byte(generalStaticEntries), &genericComDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal static entries: %v", err)
	}

	res := append(envComDetails, genericComDetails...)

	return res, nil
}
