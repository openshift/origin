package hcsoci

import (
	"github.com/Microsoft/hcsshim/internal/hns"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/sirupsen/logrus"
)

func createNetworkNamespace(coi *createOptionsInternal, resources *Resources) error {
	op := "hcsoci::createNetworkNamespace"
	log := logrus.WithField(logfields.ContainerID, coi.ID)
	log.Debug(op + " - Begin")
	defer func() {
		log.Debug(op + " - End")
	}()

	netID, err := hns.CreateNamespace()
	if err != nil {
		return err
	}
	logrus.Infof("created network namespace %s for %s", netID, coi.ID)
	resources.netNS = netID
	resources.createdNetNS = true
	for _, endpointID := range coi.Spec.Windows.Network.EndpointList {
		err = hns.AddNamespaceEndpoint(netID, endpointID)
		if err != nil {
			return err
		}
		logrus.Infof("added network endpoint %s to namespace %s", endpointID, netID)
		resources.networkEndpoints = append(resources.networkEndpoints, endpointID)
	}
	return nil
}

// GetNamespaceEndpoints gets all endpoints in `netNS`
func GetNamespaceEndpoints(netNS string) ([]*hns.HNSEndpoint, error) {
	op := "hcsoci::GetNamespaceEndpoints"
	log := logrus.WithField("netns-id", netNS)
	log.Debug(op + " - Begin")
	defer func() {
		log.Debug(op + " - End")
	}()

	ids, err := hns.GetNamespaceEndpoints(netNS)
	if err != nil {
		return nil, err
	}
	var endpoints []*hns.HNSEndpoint
	for _, id := range ids {
		endpoint, err := hns.GetHNSEndpointByID(id)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}
