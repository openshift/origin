package commatrixcreator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	log "github.com/sirupsen/logrus"

	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	"github.com/openshift-kni/commatrix/pkg/mcp"
	"github.com/openshift-kni/commatrix/pkg/types"
	configv1 "github.com/openshift/api/config/v1"
)

type CommunicationMatrixCreator struct {
	exporter            *endpointslices.EndpointSlicesExporter
	customEntriesPath   string
	customEntriesFormat string
	platformType        configv1.PlatformType
	deployment          types.Deployment
	ipv6Enabled         bool
}

func New(exporter *endpointslices.EndpointSlicesExporter, customEntriesPath string, customEntriesFormat string, platformType configv1.PlatformType, deployment types.Deployment, ipv6Enabled bool) (*CommunicationMatrixCreator, error) {
	return &CommunicationMatrixCreator{
		exporter:            exporter,
		customEntriesPath:   customEntriesPath,
		customEntriesFormat: customEntriesFormat,
		platformType:        platformType,
		deployment:          deployment,
		ipv6Enabled:         ipv6Enabled,
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
	err := cm.exporter.LoadExposedEndpointSlicesInfo()
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
	staticEntries, err := cm.GetStaticEntries()
	if err != nil {
		log.Errorf("Failed adding static entries: %s", err)
		return nil, fmt.Errorf("failed adding static entries: %s", err)
	}

	// List of [master, worker] roles per pool for static entries expansion
	PoolRolesForStaticEntriesExpansion, err := mcp.GetPoolRolesForStaticEntriesExpansion(cm.exporter.ClientSet, cm.exporter.NodeToGroup())
	if err != nil {
		log.Errorf("Failed to extract pool to roles: %v", err)
		return nil, err
	}

	// Expand static entries for all MCPs based on their roles
	staticEntries = expandStaticEntriesByPool(staticEntries, PoolRolesForStaticEntriesExpansion)
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
	res, err := types.ParseToComDetailsList(raw, cm.customEntriesFormat)
	if err != nil {
		log.Errorf("Failed to unmarshal %s file: %v", cm.customEntriesFormat, err)
		return nil, fmt.Errorf("failed to unmarshal custom entries file: %v", err)
	}

	log.Debug("Successfully unmarshalled custom entries")
	return res, nil
}

func (cm *CommunicationMatrixCreator) GetStaticEntries() ([]types.ComDetails, error) {
	log.Debug("Determining static entries based on environment and deployment")
	comDetails := []types.ComDetails{}

	switch cm.platformType {
	case configv1.BareMetalPlatformType:
		log.Debug("Adding Baremetal static entries")
		comDetails = append(comDetails, types.BaremetalStaticEntriesMaster...)
		if cm.deployment == types.SNO {
			break
		}
		comDetails = append(comDetails, types.BaremetalStaticEntriesWorker...)
	case configv1.AWSPlatformType:
		log.Debug("There are no Cloud static entries to be added")
	case configv1.NonePlatformType:
		break
	default:
		log.Errorf("Invalid value for cluster environment: %v", cm.platformType)
		return nil, fmt.Errorf("invalid value for cluster environment")
	}

	log.Debug("Adding general static entries")
	comDetails = append(comDetails, types.GeneralStaticEntriesMaster...)
	if cm.ipv6Enabled {
		comDetails = append(comDetails, types.GeneralIPv6StaticEntriesMaster...)
	}
	if cm.deployment == types.SNO {
		return comDetails, nil
	}

	comDetails = append(comDetails, types.StandardStaticEntries...)
	comDetails = append(comDetails, types.GeneralStaticEntriesWorker...)
	if cm.ipv6Enabled {
		comDetails = append(comDetails, types.GeneralIPv6StaticEntriesWorker...)
	}
	log.Debug("Successfully determined static entries")
	return comDetails, nil
}

// expandStaticEntriesByPool uses MCP-derived role per pool.
func expandStaticEntriesByPool(staticEntries []types.ComDetails, poolToRoles map[string][]string) []types.ComDetails {
	if len(poolToRoles) == 0 {
		return staticEntries
	}
	out := make([]types.ComDetails, 0, len(staticEntries))
	for _, se := range staticEntries {
		for poolName, roles := range poolToRoles {
			// check membership in slice
			if slices.Contains(roles, se.NodeGroup) {
				dup := se
				dup.NodeGroup = poolName
				out = append(out, dup)
			}
		}
	}
	return out
}
