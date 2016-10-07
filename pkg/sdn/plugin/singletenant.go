package plugin

import (
	osapi "github.com/openshift/origin/pkg/sdn/api"
)

type singleTenantPlugin struct{}

func NewSingleTenantPlugin() osdnPolicy {
	return &singleTenantPlugin{}
}

func (sp *singleTenantPlugin) Name() string {
	return osapi.SingleTenantPluginName
}

func (sp *singleTenantPlugin) Start(node *OsdnNode) error {
	return nil
}

func (sp *singleTenantPlugin) AddNetNamespace(netns *osapi.NetNamespace) {
}

func (sp *singleTenantPlugin) UpdateNetNamespace(netns *osapi.NetNamespace, oldNetID uint32) {
}

func (sp *singleTenantPlugin) DeleteNetNamespace(netns *osapi.NetNamespace) {
}

func (sp *singleTenantPlugin) GetVNID(namespace string) (uint32, error) {
	return 0, nil
}

func (sp *singleTenantPlugin) GetNamespaces(vnid uint32) []string {
	return nil
}
