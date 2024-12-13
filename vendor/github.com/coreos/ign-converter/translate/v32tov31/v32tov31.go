// Copyright 2020 Red Hat, Inc.
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

package v32tov31

import (
	"fmt"

	"github.com/coreos/ignition/v2/config/translate"
	"github.com/coreos/ignition/v2/config/v3_1/types"
	old_types "github.com/coreos/ignition/v2/config/v3_2/types"
	"github.com/coreos/ignition/v2/config/validate"
)

// Copy of github.com/coreos/ignition/v2/config/v3_2/translate/translate.go
// with the types & old_types imports reversed (the referenced file translates
// from 3.1 -> 3.2 but as a result only touches fields that are understood by
// the 3.1 spec).
func translateIgnition(old old_types.Ignition) (ret types.Ignition) {
	// use a new translator so we don't recurse infinitely
	translate.NewTranslator().Translate(&old, &ret)
	ret.Version = types.MaxVersion.String()
	return
}

func translateStorage(old old_types.Storage) (ret types.Storage) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translatePartition)
	tr.Translate(&old.Directories, &ret.Directories)
	tr.Translate(&old.Disks, &ret.Disks)
	tr.Translate(&old.Files, &ret.Files)
	tr.Translate(&old.Filesystems, &ret.Filesystems)
	tr.Translate(&old.Links, &ret.Links)
	tr.Translate(&old.Raid, &ret.Raid)
	return
}

func translatePasswdUser(old old_types.PasswdUser) (ret types.PasswdUser) {
	tr := translate.NewTranslator()
	tr.Translate(&old.Gecos, &ret.Gecos)
	tr.Translate(&old.Groups, &ret.Groups)
	tr.Translate(&old.HomeDir, &ret.HomeDir)
	tr.Translate(&old.Name, &ret.Name)
	tr.Translate(&old.NoCreateHome, &ret.NoCreateHome)
	tr.Translate(&old.NoLogInit, &ret.NoLogInit)
	tr.Translate(&old.NoUserGroup, &ret.NoUserGroup)
	tr.Translate(&old.PasswordHash, &ret.PasswordHash)
	tr.Translate(&old.PrimaryGroup, &ret.PrimaryGroup)
	tr.Translate(&old.SSHAuthorizedKeys, &ret.SSHAuthorizedKeys)
	tr.Translate(&old.Shell, &ret.Shell)
	tr.Translate(&old.System, &ret.System)
	tr.Translate(&old.UID, &ret.UID)
	return
}

func translatePasswdGroup(old old_types.PasswdGroup) (ret types.PasswdGroup) {
	tr := translate.NewTranslator()
	tr.Translate(&old.Gid, &ret.Gid)
	tr.Translate(&old.Name, &ret.Name)
	tr.Translate(&old.PasswordHash, &ret.PasswordHash)
	tr.Translate(&old.System, &ret.System)
	return
}

func translatePartition(old old_types.Partition) (ret types.Partition) {
	tr := translate.NewTranslator()
	tr.Translate(&old.GUID, &ret.GUID)
	tr.Translate(&old.Label, &ret.Label)
	tr.Translate(&old.Number, &ret.Number)
	tr.Translate(&old.ShouldExist, &ret.ShouldExist)
	tr.Translate(&old.SizeMiB, &ret.SizeMiB)
	tr.Translate(&old.StartMiB, &ret.StartMiB)
	tr.Translate(&old.TypeGUID, &ret.TypeGUID)
	tr.Translate(&old.WipePartitionEntry, &ret.WipePartitionEntry)
	return
}

func translateConfig(old old_types.Config) (ret types.Config) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateIgnition)
	tr.AddCustomTranslator(translateStorage)
	tr.AddCustomTranslator(translatePasswdUser)
	tr.AddCustomTranslator(translatePasswdGroup)
	tr.Translate(&old, &ret)
	return
}

// end copied Ignition v3_2/translate block

// Translate translates Ignition spec config v3.2 to spec v3.1
func Translate(cfg old_types.Config) (types.Config, error) {
	rpt := validate.ValidateWithContext(cfg, nil)
	if rpt.IsFatal() {
		return types.Config{}, fmt.Errorf("Invalid input config:\n%s", rpt.String())
	}

	if len(cfg.Storage.Luks) > 0 {
		return types.Config{}, fmt.Errorf("LUKS is not supported on 3.1")
	}

	// ShouldExist for Users & Groups do not exist in 3.1
	for _, u := range cfg.Passwd.Users {
		if u.ShouldExist != nil && !*u.ShouldExist {
			return types.Config{}, fmt.Errorf("ShouldExist in Passwd.Users is not supported on 3.1")
		}
	}
	for _, g := range cfg.Passwd.Groups {
		if g.ShouldExist != nil && !*g.ShouldExist {
			return types.Config{}, fmt.Errorf("ShouldExist in Passwd.Groups is not supported on 3.1")
		}
	}

	// Resize is not in 2.4
	// Fail for now
	for _, d := range cfg.Storage.Disks {
		for _, p := range d.Partitions {
			if p.Resize != nil && *p.Resize {
				return types.Config{}, fmt.Errorf("Resize in Storage.Disks.Partitions is not supported on 3.1")
			}
		}
	}

	res := translateConfig(cfg)

	// Sanity check the returned config
	oldrpt := validate.ValidateWithContext(res, nil)
	if oldrpt.IsFatal() {
		return types.Config{}, fmt.Errorf("Converted spec has unexpected fatal error:\n%s", oldrpt.String())
	}
	return res, nil
}
