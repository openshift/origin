package commatrixcreator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gocarina/gocsv"
	"sigs.k8s.io/yaml"

	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	"github.com/openshift-kni/commatrix/pkg/types"
)

type CommunicationMatrixCreator struct {
	exporter            *endpointslices.EndpointSlicesExporter
	customEntriesPath   string
	customEntriesFormat string
	e                   types.Env
	d                   types.Deployment
}

func New(exporter *endpointslices.EndpointSlicesExporter, customEntriesPath string, customEntriesFormat string, e types.Env, d types.Deployment) (*CommunicationMatrixCreator, error) {
	return &CommunicationMatrixCreator{
		exporter:            exporter,
		customEntriesPath:   customEntriesPath,
		customEntriesFormat: customEntriesFormat,
		e:                   e,
		d:                   d,
	}, nil
}

// CreateEndpointMatrix initializes a ComMatrix using Kubernetes cluster data.
// It takes kubeconfigPath for cluster access to  fetch EndpointSlice objects,
// detailing open ports for ingress traffic.
// Custom entries from a JSON file can be added to the matrix by setting `customEntriesPath`.
// Returns a pointer to ComMatrix and error. Entries include traffic direction, protocol,
// port number, namespace, service name, pod, container, node role, and flow optionality for OpenShift.
func (cm *CommunicationMatrixCreator) CreateEndpointMatrix() (*types.ComMatrix, error) {
	err := cm.exporter.LoadEndpointSlicesInfo()
	if err != nil {
		return nil, fmt.Errorf("failed loading endpointslices: %w", err)
	}

	epSliceComDetails, err := cm.exporter.ToComDetails()
	if err != nil {
		return nil, err
	}

	staticEntries, err := cm.getStaticEntries()
	if err != nil {
		return nil, fmt.Errorf("failed adding static entries: %s", err)
	}
	epSliceComDetails = append(epSliceComDetails, staticEntries...)

	if cm.customEntriesPath != "" {
		customComDetails, err := cm.GetComDetailsListFromFile()
		if err != nil {
			return nil, fmt.Errorf("failed adding custom entries: %s", err)
		}
		epSliceComDetails = append(epSliceComDetails, customComDetails...)
	}

	commMatrix := &types.ComMatrix{Matrix: epSliceComDetails}
	commMatrix.CleanComDetails()
	return commMatrix, nil
}

func (cm *CommunicationMatrixCreator) GetComDetailsListFromFile() ([]types.ComDetails, error) {
	var res []types.ComDetails
	f, err := os.Open(filepath.Clean(cm.customEntriesPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %v", cm.customEntriesPath, err)
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", cm.customEntriesPath, err)
	}

	switch cm.customEntriesFormat {
	case types.FormatJSON:
		err = json.Unmarshal(raw, &res)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom entries file: %v", err)
		}
	case types.FormatYAML:
		err = yaml.Unmarshal(raw, &res)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom entries file: %v", err)
		}
	case types.FormatCSV:
		err = gocsv.UnmarshalBytes(raw, &res)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom entries file: %v", err)
		}
	default:
		return nil, fmt.Errorf("invalid value for format must be (json,yaml,csv)")
	}

	return res, nil
}

func (cm *CommunicationMatrixCreator) getStaticEntries() ([]types.ComDetails, error) {
	comDetails := []types.ComDetails{}

	switch cm.e {
	case types.Baremetal:
		comDetails = append(comDetails, types.BaremetalStaticEntriesMaster...)
		if cm.d == types.SNO {
			break
		}
		comDetails = append(comDetails, types.BaremetalStaticEntriesWorker...)
	case types.Cloud:
		comDetails = append(comDetails, types.CloudStaticEntriesMaster...)
		if cm.d == types.SNO {
			break
		}
		comDetails = append(comDetails, types.CloudStaticEntriesWorker...)
	default:
		return nil, fmt.Errorf("invalid value for cluster environment")
	}

	comDetails = append(comDetails, types.GeneralStaticEntriesMaster...)
	if cm.d == types.SNO {
		return comDetails, nil
	}

	comDetails = append(comDetails, types.MNOStaticEntries...)
	comDetails = append(comDetails, types.GeneralStaticEntriesWorker...)

	return comDetails, nil
}
