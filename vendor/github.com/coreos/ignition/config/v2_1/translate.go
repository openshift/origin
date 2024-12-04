// Copyright 2018 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2_1

import (
	"strings"

	"github.com/coreos/ignition/config/util"
	v2_0 "github.com/coreos/ignition/config/v2_0/types"
	"github.com/coreos/ignition/config/v2_1/types"
)

// golang--
func translateV2_0MkfsOptionsTov2_1OptionSlice(opts v2_0.MkfsOptions) []types.CreateOption {
	newOpts := make([]types.CreateOption, len(opts))
	for i, o := range opts {
		newOpts[i] = types.CreateOption(o)
	}
	return newOpts
}

// golang--
func translateStringSliceTov2_1SSHAuthorizedKeySlice(keys []string) []types.SSHAuthorizedKey {
	newKeys := make([]types.SSHAuthorizedKey, len(keys))
	for i, k := range keys {
		newKeys[i] = types.SSHAuthorizedKey(k)
	}
	return newKeys
}

// golang--
func translateStringSliceTov2_1UsercreateGroupSlice(groups []string) []types.UsercreateGroup {
	var newGroups []types.UsercreateGroup
	for _, g := range groups {
		newGroups = append(newGroups, types.UsercreateGroup(g))
	}
	return newGroups
}

func TranslateFromV2_0(old v2_0.Config) types.Config {
	translateVerification := func(old v2_0.Verification) types.Verification {
		var ver types.Verification
		if old.Hash != nil {
			// .String() here is a wrapper around MarshalJSON, which will put the hash in quotes
			h := strings.Trim(old.Hash.String(), "\"")
			ver.Hash = &h
		}
		return ver
	}
	translateConfigReference := func(old v2_0.ConfigReference) types.ConfigReference {
		return types.ConfigReference{
			Source:       old.Source.String(),
			Verification: translateVerification(old.Verification),
		}
	}

	config := types.Config{
		Ignition: types.Ignition{
			Version: types.MaxVersion.String(),
		},
	}

	if old.Ignition.Config.Replace != nil {
		ref := translateConfigReference(*old.Ignition.Config.Replace)
		config.Ignition.Config.Replace = &ref
	}

	for _, oldAppend := range old.Ignition.Config.Append {
		config.Ignition.Config.Append =
			append(config.Ignition.Config.Append, translateConfigReference(oldAppend))
	}

	for _, oldDisk := range old.Storage.Disks {
		disk := types.Disk{
			Device:    string(oldDisk.Device),
			WipeTable: oldDisk.WipeTable,
		}

		for _, oldPartition := range oldDisk.Partitions {
			disk.Partitions = append(disk.Partitions, types.Partition{
				Label:    string(oldPartition.Label),
				Number:   oldPartition.Number,
				Size:     int(oldPartition.Size),
				Start:    int(oldPartition.Start),
				TypeGUID: string(oldPartition.TypeGUID),
			})
		}

		config.Storage.Disks = append(config.Storage.Disks, disk)
	}

	for _, oldArray := range old.Storage.Arrays {
		array := types.Raid{
			Name:   oldArray.Name,
			Level:  oldArray.Level,
			Spares: oldArray.Spares,
		}

		for _, oldDevice := range oldArray.Devices {
			array.Devices = append(array.Devices, types.Device(oldDevice))
		}

		config.Storage.Raid = append(config.Storage.Raid, array)
	}

	for _, oldFilesystem := range old.Storage.Filesystems {
		filesystem := types.Filesystem{
			Name: oldFilesystem.Name,
		}

		if oldFilesystem.Mount != nil {
			filesystem.Mount = &types.Mount{
				Device: string(oldFilesystem.Mount.Device),
				Format: string(oldFilesystem.Mount.Format),
			}

			if oldFilesystem.Mount.Create != nil {
				filesystem.Mount.Create = &types.Create{
					Force:   oldFilesystem.Mount.Create.Force,
					Options: translateV2_0MkfsOptionsTov2_1OptionSlice(oldFilesystem.Mount.Create.Options),
				}
			}
		}

		if oldFilesystem.Path != nil {
			p := string(*oldFilesystem.Path)
			filesystem.Path = &p
		}

		config.Storage.Filesystems = append(config.Storage.Filesystems, filesystem)
	}

	for _, oldFile := range old.Storage.Files {
		file := types.File{
			Node: types.Node{
				Filesystem: oldFile.Filesystem,
				Path:       string(oldFile.Path),
				User:       types.NodeUser{ID: util.IntToPtr(oldFile.User.Id)},
				Group:      types.NodeGroup{ID: util.IntToPtr(oldFile.Group.Id)},
			},
			FileEmbedded1: types.FileEmbedded1{
				Mode: int(oldFile.Mode),
				Contents: types.FileContents{
					Compression:  string(oldFile.Contents.Compression),
					Source:       oldFile.Contents.Source.String(),
					Verification: translateVerification(oldFile.Contents.Verification),
				},
			},
		}

		config.Storage.Files = append(config.Storage.Files, file)
	}

	for _, oldUnit := range old.Systemd.Units {
		unit := types.Unit{
			Name:     string(oldUnit.Name),
			Enable:   oldUnit.Enable,
			Mask:     oldUnit.Mask,
			Contents: oldUnit.Contents,
		}

		for _, oldDropIn := range oldUnit.DropIns {
			unit.Dropins = append(unit.Dropins, types.Dropin{
				Name:     string(oldDropIn.Name),
				Contents: oldDropIn.Contents,
			})
		}

		config.Systemd.Units = append(config.Systemd.Units, unit)
	}

	for _, oldUnit := range old.Networkd.Units {
		config.Networkd.Units = append(config.Networkd.Units, types.Networkdunit{
			Name:     string(oldUnit.Name),
			Contents: oldUnit.Contents,
		})
	}

	for _, oldUser := range old.Passwd.Users {
		user := types.PasswdUser{
			Name:              oldUser.Name,
			PasswordHash:      util.StrToPtr(oldUser.PasswordHash),
			SSHAuthorizedKeys: translateStringSliceTov2_1SSHAuthorizedKeySlice(oldUser.SSHAuthorizedKeys),
		}

		if oldUser.Create != nil {
			var u *int
			if oldUser.Create.Uid != nil {
				tmp := int(*oldUser.Create.Uid)
				u = &tmp
			}
			user.Create = &types.Usercreate{
				UID:          u,
				Gecos:        oldUser.Create.GECOS,
				HomeDir:      oldUser.Create.Homedir,
				NoCreateHome: oldUser.Create.NoCreateHome,
				PrimaryGroup: oldUser.Create.PrimaryGroup,
				Groups:       translateStringSliceTov2_1UsercreateGroupSlice(oldUser.Create.Groups),
				NoUserGroup:  oldUser.Create.NoUserGroup,
				System:       oldUser.Create.System,
				NoLogInit:    oldUser.Create.NoLogInit,
				Shell:        oldUser.Create.Shell,
			}
		}

		config.Passwd.Users = append(config.Passwd.Users, user)
	}

	for _, oldGroup := range old.Passwd.Groups {
		var g *int
		if oldGroup.Gid != nil {
			tmp := int(*oldGroup.Gid)
			g = &tmp
		}
		config.Passwd.Groups = append(config.Passwd.Groups, types.PasswdGroup{
			Name:         oldGroup.Name,
			Gid:          g,
			PasswordHash: oldGroup.PasswordHash,
			System:       oldGroup.System,
		})
	}

	return config
}
