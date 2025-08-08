// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VAPI REST Paths
const (
	SessionPath                    = "/com/vmware/cis/session"
	CategoryPath                   = "/com/vmware/cis/tagging/category"
	TagPath                        = "/com/vmware/cis/tagging/tag"
	AssociationPath                = "/com/vmware/cis/tagging/tag-association"
	LibraryPath                    = "/com/vmware/content/library"
	LibraryItemFileData            = "/com/vmware/cis/data"
	LibraryItemPath                = "/com/vmware/content/library/item"
	LibraryItemFilePath            = "/com/vmware/content/library/item/file"
	LibraryItemStoragePath         = "/com/vmware/content/library/item/storage"
	LibraryItemUpdateSession       = "/com/vmware/content/library/item/update-session"
	LibraryItemUpdateSessionFile   = "/com/vmware/content/library/item/updatesession/file"
	LibraryItemDownloadSession     = "/com/vmware/content/library/item/download-session"
	LibraryItemDownloadSessionFile = "/com/vmware/content/library/item/downloadsession/file"
	LocalLibraryPath               = "/com/vmware/content/local-library"
	SubscribedLibraryPath          = "/com/vmware/content/subscribed-library"
	SecurityPoliciesPath           = "/api/content/security-policies"
	SubscribedLibraryItem          = "/com/vmware/content/library/subscribed-item"
	Subscriptions                  = "/com/vmware/content/library/subscriptions"
	TrustedCertificatesPath        = "/api/content/trusted-certificates"
	VCenterOVFLibraryItem          = "/com/vmware/vcenter/ovf/library-item"
	VCenterVMTXLibraryItem         = "/vcenter/vm-template/library-items"
	SessionCookieName              = "vmware-api-session-id"
	UseHeaderAuthn                 = "vmware-use-header-authn"
	DebugEcho                      = "/vc-sim/debug/echo"
)

// AssociatedObject is the same structure as types.ManagedObjectReference,
// just with a different field name (ID instead of Value).
// In the API we use mo.Reference, this type is only used for wire transfer.
type AssociatedObject struct {
	Type  string `json:"type"`
	Value string `json:"id"`
}

// Reference implements mo.Reference
func (o AssociatedObject) Reference() types.ManagedObjectReference {
	return types.ManagedObjectReference{
		Type:  o.Type,
		Value: o.Value,
	}
}

// Association for tag-association requests.
type Association struct {
	ObjectID *AssociatedObject `json:"object_id,omitempty"`
}

// NewAssociation returns an Association, converting ref to an AssociatedObject.
func NewAssociation(ref mo.Reference) Association {
	return Association{
		ObjectID: &AssociatedObject{
			Type:  ref.Reference().Type,
			Value: ref.Reference().Value,
		},
	}
}

type SubscriptionDestination struct {
	ID string `json:"subscription"`
}

type SubscriptionDestinationSpec struct {
	Subscriptions []SubscriptionDestination `json:"subscriptions,omitempty"`
}

type SubscriptionItemDestinationSpec struct {
	Force         bool                      `json:"force_sync_content"`
	Subscriptions []SubscriptionDestination `json:"subscriptions,omitempty"`
}
