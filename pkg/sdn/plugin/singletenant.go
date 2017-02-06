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
	otx := node.ovs.NewTransaction()
	otx.AddFlow("table=80, priority=200, actions=output:NXM_NX_REG2[]")
	return otx.EndTransaction()
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

func (sp *singleTenantPlugin) GetMulticastEnabled(vnid uint32) bool {
	return false
}

func (sp *singleTenantPlugin) RefVNID(vnid uint32) {
}

func (sp *singleTenantPlugin) UnrefVNID(vnid uint32) {
}
