// +build linux

package node

import (
	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
)

type singleTenantPlugin struct{}

func NewSingleTenantPlugin() osdnPolicy {
	return &singleTenantPlugin{}
}

func (sp *singleTenantPlugin) Name() string {
	return network.SingleTenantPluginName
}

func (sp *singleTenantPlugin) SupportsVNIDs() bool {
	return false
}

func (sp *singleTenantPlugin) Start(node *OsdnNode) error {
	otx := node.oc.NewTransaction()
	otx.AddFlow("table=80, priority=200, actions=output:NXM_NX_REG2[]")
	return otx.Commit()
}

func (sp *singleTenantPlugin) AddNetNamespace(netns *networkapi.NetNamespace) {
}

func (sp *singleTenantPlugin) UpdateNetNamespace(netns *networkapi.NetNamespace, oldNetID uint32) {
}

func (sp *singleTenantPlugin) DeleteNetNamespace(netns *networkapi.NetNamespace) {
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

func (sp *singleTenantPlugin) EnsureVNIDRules(vnid uint32) {
}

func (sp *singleTenantPlugin) SyncVNIDRules() {
}
