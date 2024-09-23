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

package v2_0

import (
	"fmt"

	v1 "github.com/coreos/ignition/config/v1/types"
	"github.com/coreos/ignition/config/v2_0/types"
	"github.com/vincent-petithory/dataurl"
)

func TranslateFromV1(old v1.Config) types.Config {
	config := types.Config{
		Ignition: types.Ignition{
			Version: types.IgnitionVersion(types.MaxVersion),
		},
	}

	for _, oldDisk := range old.Storage.Disks {
		disk := types.Disk{
			Device:    types.Path(oldDisk.Device),
			WipeTable: oldDisk.WipeTable,
		}

		for _, oldPartition := range oldDisk.Partitions {
			disk.Partitions = append(disk.Partitions, types.Partition{
				Label:    types.PartitionLabel(oldPartition.Label),
				Number:   oldPartition.Number,
				Size:     types.PartitionDimension(oldPartition.Size),
				Start:    types.PartitionDimension(oldPartition.Start),
				TypeGUID: types.PartitionTypeGUID(oldPartition.TypeGUID),
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
			array.Devices = append(array.Devices, types.Path(oldDevice))
		}

		config.Storage.Arrays = append(config.Storage.Arrays, array)
	}

	for i, oldFilesystem := range old.Storage.Filesystems {
		filesystem := types.Filesystem{
			Name: fmt.Sprintf("_translate-filesystem-%d", i),
			Mount: &types.FilesystemMount{
				Device: types.Path(oldFilesystem.Device),
				Format: types.FilesystemFormat(oldFilesystem.Format),
			},
		}

		if oldFilesystem.Create != nil {
			filesystem.Mount.Create = &types.FilesystemCreate{
				Force:   oldFilesystem.Create.Force,
				Options: types.MkfsOptions(oldFilesystem.Create.Options),
			}
		}

		config.Storage.Filesystems = append(config.Storage.Filesystems, filesystem)

		for _, oldFile := range oldFilesystem.Files {
			file := types.File{
				Filesystem: filesystem.Name,
				Path:       types.Path(oldFile.Path),
				User:       types.FileUser{Id: oldFile.Uid},
				Group:      types.FileGroup{Id: oldFile.Gid},
				Mode:       types.FileMode(oldFile.Mode),
				Contents: types.FileContents{
					Source: types.Url{
						Scheme: "data",
						Opaque: "," + dataurl.EscapeString(oldFile.Contents),
					},
				},
			}

			config.Storage.Files = append(config.Storage.Files, file)
		}
	}

	for _, oldUnit := range old.Systemd.Units {
		unit := types.SystemdUnit{
			Name:     types.SystemdUnitName(oldUnit.Name),
			Enable:   oldUnit.Enable,
			Mask:     oldUnit.Mask,
			Contents: oldUnit.Contents,
		}

		for _, oldDropIn := range oldUnit.DropIns {
			unit.DropIns = append(unit.DropIns, types.SystemdUnitDropIn{
				Name:     types.SystemdUnitDropInName(oldDropIn.Name),
				Contents: oldDropIn.Contents,
			})
		}

		config.Systemd.Units = append(config.Systemd.Units, unit)
	}

	for _, oldUnit := range old.Networkd.Units {
		config.Networkd.Units = append(config.Networkd.Units, types.NetworkdUnit{
			Name:     types.NetworkdUnitName(oldUnit.Name),
			Contents: oldUnit.Contents,
		})
	}

	for _, oldUser := range old.Passwd.Users {
		user := types.User{
			Name:              oldUser.Name,
			PasswordHash:      oldUser.PasswordHash,
			SSHAuthorizedKeys: oldUser.SSHAuthorizedKeys,
		}

		if oldUser.Create != nil {
			var uid *uint
			if oldUser.Create.Uid != nil {
				tmp := uint(*oldUser.Create.Uid)
				uid = &tmp
			}

			user.Create = &types.UserCreate{
				Uid:          uid,
				GECOS:        oldUser.Create.GECOS,
				Homedir:      oldUser.Create.Homedir,
				NoCreateHome: oldUser.Create.NoCreateHome,
				PrimaryGroup: oldUser.Create.PrimaryGroup,
				Groups:       oldUser.Create.Groups,
				NoUserGroup:  oldUser.Create.NoUserGroup,
				System:       oldUser.Create.System,
				NoLogInit:    oldUser.Create.NoLogInit,
				Shell:        oldUser.Create.Shell,
			}
		}

		config.Passwd.Users = append(config.Passwd.Users, user)
	}

	for _, oldGroup := range old.Passwd.Groups {
		var gid *uint
		if oldGroup.Gid != nil {
			tmp := uint(*oldGroup.Gid)
			gid = &tmp
		}
		config.Passwd.Groups = append(config.Passwd.Groups, types.Group{
			Name:         oldGroup.Name,
			Gid:          gid,
			PasswordHash: oldGroup.PasswordHash,
			System:       oldGroup.System,
		})
	}

	return config
}
