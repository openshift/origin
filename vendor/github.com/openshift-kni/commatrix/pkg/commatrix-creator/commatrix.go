package commatrixcreator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"
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
	log.Debug("Loading EndpointSlices information")
	err := cm.exporter.LoadEndpointSlicesInfo()
	if err != nil {
		log.Errorf("Failed loading endpointslices: %v", err)
		return nil, fmt.Errorf("failed loading endpointslices: %w", err)
	}

	log.Debug("Converting EndpointSlices to ComDetails")
	epSliceComDetails, err := cm.exporter.ToComDetails()
	if err != nil {
		log.Errorf("Failed to convert endpoint slices: %v", err)
		return nil, err
	}

	log.Debug("Getting static entries")
	staticEntries, err := cm.getStaticEntries()
	if err != nil {
		log.Errorf("Failed adding static entries: %s", err)
		return nil, fmt.Errorf("failed adding static entries: %s", err)
	}
	epSliceComDetails = append(epSliceComDetails, staticEntries...)

	if cm.customEntriesPath != "" {
		log.Debug("Loading custom entries from file")
		customComDetails, err := cm.GetComDetailsListFromFile()
		if err != nil {
			log.Errorf("Failed adding custom entries: %s", err)
			return nil, fmt.Errorf("failed adding custom entries: %s", err)
		}
		epSliceComDetails = append(epSliceComDetails, customComDetails...)
	}

	commMatrix := &types.ComMatrix{Matrix: epSliceComDetails}
	log.Debug("Sorting ComMatrix and removing duplicates")
	commMatrix.SortAndRemoveDuplicates()
	return commMatrix, nil
}

func (cm *CommunicationMatrixCreator) GetComDetailsListFromFile() ([]types.ComDetails, error) {
	var res []types.ComDetails
	log.Debugf("Opening file %s", cm.customEntriesPath)
	f, err := os.Open(filepath.Clean(cm.customEntriesPath))
	if err != nil {
		log.Errorf("Failed to open file %s: %v", cm.customEntriesPath, err)
		return nil, fmt.Errorf("failed to open file %s: %v", cm.customEntriesPath, err)
	}
	defer f.Close()

	log.Debugf("Reading file %s", cm.customEntriesPath)
	raw, err := io.ReadAll(f)
	if err != nil {
		log.Errorf("Failed to read file %s: %v", cm.customEntriesPath, err)
		return nil, fmt.Errorf("failed to read file %s: %v", cm.customEntriesPath, err)
	}

	log.Debugf("Unmarshalling file content with format %s", cm.customEntriesFormat)
	switch cm.customEntriesFormat {
	case types.FormatJSON:
		err = json.Unmarshal(raw, &res)
		if err != nil {
			log.Errorf("Failed to unmarshal JSON file: %v", err)
			return nil, fmt.Errorf("failed to unmarshal custom entries file: %v", err)
		}
	case types.FormatYAML:
		err = yaml.Unmarshal(raw, &res)
		if err != nil {
			log.Errorf("Failed to unmarshal YAML file: %v", err)
			return nil, fmt.Errorf("failed to unmarshal custom entries file: %v", err)
		}
	case types.FormatCSV:
		err = gocsv.UnmarshalBytes(raw, &res)
		if err != nil {
			log.Errorf("Failed to unmarshal CSV file: %v", err)
			return nil, fmt.Errorf("failed to unmarshal custom entries file: %v", err)
		}
	default:
		log.Errorf("Invalid format specified: %s", cm.customEntriesFormat)
		return nil, fmt.Errorf("invalid value for format must be (json,yaml,csv)")
	}

	log.Debug("Successfully unmarshalled custom entries")
	return res, nil
}

func (cm *CommunicationMatrixCreator) getStaticEntries() ([]types.ComDetails, error) {
	log.Debug("Determining static entries based on environment and deployment")
	comDetails := []types.ComDetails{}

	switch cm.e {
	case types.Baremetal:
		log.Debug("Adding Baremetal static entries")
		comDetails = append(comDetails, types.BaremetalStaticEntriesMaster...)
		if cm.d == types.SNO {
			break
		}
		comDetails = append(comDetails, types.BaremetalStaticEntriesWorker...)
	case types.Cloud:
		log.Debug("Adding Cloud static entries")
		comDetails = append(comDetails, types.CloudStaticEntriesMaster...)
		if cm.d == types.SNO {
			break
		}
		comDetails = append(comDetails, types.CloudStaticEntriesWorker...)
	default:
		log.Errorf("Invalid value for cluster environment: %v", cm.e)
		return nil, fmt.Errorf("invalid value for cluster environment")
	}

	log.Debug("Adding general static entries")
	comDetails = append(comDetails, types.GeneralStaticEntriesMaster...)
	if cm.d == types.SNO {
		return comDetails, nil
	}

	comDetails = append(comDetails, types.MNOStaticEntries...)
	comDetails = append(comDetails, types.GeneralStaticEntriesWorker...)
	log.Debug("Successfully determined static entries")
	return comDetails, nil
}
