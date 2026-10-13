package rosacli

import (
	"bytes"
	"sort"

	"github.com/Masterminds/semver"
	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
)

const VersionChannelGroupStable = "stable"
const VersionChannelGroupNightly = "nightly"

type VersionService interface {
	ResourcesCleaner

	ReflectVersions(result bytes.Buffer) (*OpenShiftVersionList, error)
	ListVersions(channelGroup string, hostedCP bool, flags ...string) (bytes.Buffer, error)
	ListAndReflectVersions(channelGroup string, hostedCP bool, flags ...string) (*OpenShiftVersionList, error)
}

type versionService struct {
	ResourcesService
}

func NewVersionService(client *Client) VersionService {
	return &versionService{
		ResourcesService: ResourcesService{
			client: client,
		},
	}
}

type OpenShiftVersion struct {
	Version           string `json:"VERSION,omitempty"`
	Default           string `json:"DEFAULT,omitempty"`
	AvailableUpgrades string `json:"AVAILABLE UPGRADES,omitempty"`
}

type OpenShiftVersionList struct {
	OpenShiftVersions []*OpenShiftVersion `json:"OpenShiftVersions,omitempty"`
}

// Reflect versions
func (v *versionService) ReflectVersions(result bytes.Buffer) (versionList *OpenShiftVersionList, err error) {
	versionList = &OpenShiftVersionList{}
	theMap := v.client.Parser.TableData.Input(result).Parse().Output()
	for _, item := range theMap {
		version := &OpenShiftVersion{}
		err = MapStructure(item, version)
		if err != nil {
			return versionList, err
		}
		versionList.OpenShiftVersions = append(versionList.OpenShiftVersions, version)
	}
	return versionList, err
}

// list version `rosa list version` or `rosa list version --hosted-cp`
func (v *versionService) ListVersions(channelGroup string, hostedCP bool, flags ...string) (bytes.Buffer, error) {
	listVersion := v.client.Runner.
		Cmd("list", "versions").
		CmdFlags(flags...)

	if hostedCP {
		listVersion.AddCmdFlags("--hosted-cp")
	}

	if channelGroup != "" {
		listVersion.AddCmdFlags("--channel-group", channelGroup)
	}

	return listVersion.Run()
}

func (v *versionService) ListAndReflectVersions(channelGroup string, hostedCP bool, flags ...string) (versionList *OpenShiftVersionList, err error) {
	var output bytes.Buffer
	output, err = v.ListVersions(channelGroup, hostedCP, flags...)
	if err != nil {
		return versionList, err
	}

	versionList, err = v.ReflectVersions(output)
	return versionList, err
}

func (v *versionService) CleanResources(clusterID string) (errors []error) {
	logger.Debugf("Nothing to clean in Version Service")
	return
}

// This function will find the nearest lower OCP version which version is under `Major.{minor-sub}`.
// `strict` will find only the `Major.{minor-sub}` ones
func (vl *OpenShiftVersionList) FindNearestBackwardMinorVersion(version string, minorSub int64, strict bool) (vs *OpenShiftVersion, err error) {
	var baseVersionSemVer *semver.Version
	baseVersionSemVer, err = semver.NewVersion(version)
	if err != nil {
		return
	}
	nvl, err := vl.FilterVersionsSameMajorAndEqualOrLowerThanMinor(baseVersionSemVer.Major(), baseVersionSemVer.Minor()-minorSub, strict)
	if err != nil {
		return
	}
	if nvl, err = nvl.Sort(true); err == nil && nvl.Len() > 0 {
		vs = nvl.OpenShiftVersions[0]
	}
	return

}

// Sort sort the version list from lower to higher (or reverse)
func (vl *OpenShiftVersionList) Sort(reverse bool) (nvl *OpenShiftVersionList, err error) {
	versionListIndexMap := make(map[string]*OpenShiftVersion)
	var semVerList []*semver.Version
	var vSemVer *semver.Version
	for _, version := range vl.OpenShiftVersions {
		versionListIndexMap[version.Version] = version
		if vSemVer, err = semver.NewVersion(version.Version); err != nil {
			return
		} else {
			semVerList = append(semVerList, vSemVer)
		}
	}

	if reverse {
		sort.Sort(sort.Reverse(semver.Collection(semVerList)))
	} else {
		sort.Sort(semver.Collection(semVerList))
	}

	var sortedImageVersionList []*OpenShiftVersion
	for _, semverVersion := range semVerList {
		sortedImageVersionList = append(sortedImageVersionList, versionListIndexMap[semverVersion.Original()])
	}

	nvl = &OpenShiftVersionList{
		OpenShiftVersions: sortedImageVersionList,
	}

	return
}

// FilterVersionsByMajorMinor filter the version list for all major/minor corresponding and returns a new `OpenShiftVersionList` struct
// `strict` will find only the `Major.minor` ones
func (vl *OpenShiftVersionList) FilterVersionsSameMajorAndEqualOrLowerThanMinor(major int64, minor int64, strict bool) (nvl *OpenShiftVersionList, err error) {
	var filteredVersions []*OpenShiftVersion
	var semverVersion *semver.Version
	for _, version := range vl.OpenShiftVersions {
		if semverVersion, err = semver.NewVersion(version.Version); err != nil {
			return
		} else if semverVersion.Major() == major &&
			((strict && semverVersion.Minor() == minor) || (!strict && semverVersion.Minor() <= minor)) {
			filteredVersions = append(filteredVersions, version)
		}
	}

	nvl = &OpenShiftVersionList{
		OpenShiftVersions: filteredVersions,
	}

	return
}

// FilterVersionsByMajorMinor filter the version list for all lower versions than the given one
func (vl *OpenShiftVersionList) FilterVersionsLowerThan(version string) (nvl *OpenShiftVersionList, err error) {
	var givenSemVer *semver.Version
	givenSemVer, err = semver.NewVersion(version)

	var filteredVersions []*OpenShiftVersion
	var semverVersion *semver.Version
	for _, version := range vl.OpenShiftVersions {
		if semverVersion, err = semver.NewVersion(version.Version); err != nil {
			return
		} else if semverVersion.LessThan(givenSemVer) {
			filteredVersions = append(filteredVersions, version)
		}
	}

	nvl = &OpenShiftVersionList{
		OpenShiftVersions: filteredVersions,
	}

	return
}

func (vl *OpenShiftVersionList) Len() int {
	return len(vl.OpenShiftVersions)
}
