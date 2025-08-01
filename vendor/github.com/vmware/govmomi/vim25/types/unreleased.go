// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package types

import "reflect"

type ArrayOfPlaceVmsXClusterResultPlacementFaults struct {
	PlaceVmsXClusterResultPlacementFaults []PlaceVmsXClusterResultPlacementFaults `xml:"PlaceVmsXClusterResultPlacementFaults,omitempty"`
}

func init() {
	t["ArrayOfPlaceVmsXClusterResultPlacementFaults"] = reflect.TypeOf((*ArrayOfPlaceVmsXClusterResultPlacementFaults)(nil)).Elem()
}

type ArrayOfPlaceVmsXClusterResultPlacementInfo struct {
	PlaceVmsXClusterResultPlacementInfo []PlaceVmsXClusterResultPlacementInfo `xml:"PlaceVmsXClusterResultPlacementInfo,omitempty"`
}

func init() {
	t["ArrayOfPlaceVmsXClusterResultPlacementInfo"] = reflect.TypeOf((*ArrayOfPlaceVmsXClusterResultPlacementInfo)(nil)).Elem()
}

type ArrayOfPlaceVmsXClusterSpecVmPlacementSpec struct {
	PlaceVmsXClusterSpecVmPlacementSpec []PlaceVmsXClusterSpecVmPlacementSpec `xml:"PlaceVmsXClusterSpecVmPlacementSpec,omitempty"`
}

func init() {
	t["ArrayOfPlaceVmsXClusterSpecVmPlacementSpec"] = reflect.TypeOf((*ArrayOfPlaceVmsXClusterSpecVmPlacementSpec)(nil)).Elem()
}

type PlaceVmsXCluster PlaceVmsXClusterRequestType

func init() {
	t["PlaceVmsXCluster"] = reflect.TypeOf((*PlaceVmsXCluster)(nil)).Elem()
}

type PlaceVmsXClusterRequestType struct {
	This          ManagedObjectReference `xml:"_this"`
	PlacementSpec PlaceVmsXClusterSpec   `xml:"placementSpec"`
}

func init() {
	t["PlaceVmsXClusterRequestType"] = reflect.TypeOf((*PlaceVmsXClusterRequestType)(nil)).Elem()
}

type PlaceVmsXClusterResponse struct {
	Returnval PlaceVmsXClusterResult `xml:"returnval"`
}

type PlaceVmsXClusterResult struct {
	DynamicData

	PlacementInfos []PlaceVmsXClusterResultPlacementInfo   `xml:"placementInfos,omitempty"`
	Faults         []PlaceVmsXClusterResultPlacementFaults `xml:"faults,omitempty"`
}

func init() {
	t["PlaceVmsXClusterResult"] = reflect.TypeOf((*PlaceVmsXClusterResult)(nil)).Elem()
}

type PlaceVmsXClusterResultPlacementFaults struct {
	DynamicData

	ResourcePool ManagedObjectReference  `xml:"resourcePool"`
	VmName       string                  `xml:"vmName"`
	Faults       []LocalizedMethodFault  `xml:"faults,omitempty"`
	Vm           *ManagedObjectReference `xml:"vm,omitempty"`
}

func init() {
	t["PlaceVmsXClusterResultPlacementFaults"] = reflect.TypeOf((*PlaceVmsXClusterResultPlacementFaults)(nil)).Elem()
}

type PlaceVmsXClusterResultPlacementInfo struct {
	DynamicData

	VmName         string                  `xml:"vmName"`
	Recommendation ClusterRecommendation   `xml:"recommendation"`
	Vm             *ManagedObjectReference `xml:"vm,omitempty"`
}

func init() {
	t["PlaceVmsXClusterResultPlacementInfo"] = reflect.TypeOf((*PlaceVmsXClusterResultPlacementInfo)(nil)).Elem()
}

type PlaceVmsXClusterSpec struct {
	DynamicData

	ResourcePools           []ManagedObjectReference              `xml:"resourcePools,omitempty"`
	PlacementType           string                                `xml:"placementType,omitempty"`
	VmPlacementSpecs        []PlaceVmsXClusterSpecVmPlacementSpec `xml:"vmPlacementSpecs,omitempty"`
	HostRecommRequired      *bool                                 `xml:"hostRecommRequired"`
	DatastoreRecommRequired *bool                                 `xml:"datastoreRecommRequired"`
}

func init() {
	t["PlaceVmsXClusterSpec"] = reflect.TypeOf((*PlaceVmsXClusterSpec)(nil)).Elem()
}

type PlaceVmsXClusterSpecVmPlacementSpec struct {
	DynamicData

	Vm           *ManagedObjectReference     `xml:"vm,omitempty"`
	ConfigSpec   VirtualMachineConfigSpec    `xml:"configSpec"`
	RelocateSpec *VirtualMachineRelocateSpec `xml:"relocateSpec,omitempty"`
}

func init() {
	t["PlaceVmsXClusterSpecVmPlacementSpec"] = reflect.TypeOf((*PlaceVmsXClusterSpecVmPlacementSpec)(nil)).Elem()
}

const RecommendationReasonCodeXClusterPlacement = RecommendationReasonCode("xClusterPlacement")

type ClusterClusterReconfigurePlacementAction struct {
	ClusterAction
	TargetHost *ManagedObjectReference   `xml:"targetHost,omitempty"`
	Pool       ManagedObjectReference    `xml:"pool"`
	ConfigSpec *VirtualMachineConfigSpec `xml:"configSpec,omitempty"`
}

func init() {
	t["ClusterClusterReconfigurePlacementAction"] = reflect.TypeOf((*ClusterClusterReconfigurePlacementAction)(nil)).Elem()
}

type ClusterClusterRelocatePlacementAction struct {
	ClusterAction
	TargetHost   *ManagedObjectReference     `xml:"targetHost,omitempty"`
	Pool         ManagedObjectReference      `xml:"pool"`
	RelocateSpec *VirtualMachineRelocateSpec `xml:"relocateSpec,omitempty"`
}

func init() {
	t["ClusterClusterRelocatePlacementAction"] = reflect.TypeOf((*ClusterClusterRelocatePlacementAction)(nil)).Elem()
}

func init() {
	Add("PodVMOverheadInfo", reflect.TypeOf((*PodVMOverheadInfo)(nil)).Elem())
}

type PodVMOverheadInfo struct {
	CrxPageSharingSupported         bool  `xml:"crxPageSharingSupported"`
	PodVMOverheadWithoutPageSharing int32 `xml:"podVMOverheadWithoutPageSharing"`
	PodVMOverheadWithPageSharing    int32 `xml:"podVMOverheadWithPageSharing"`
}
