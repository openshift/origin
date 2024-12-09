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

package v32tov22

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	old "github.com/coreos/ignition/config/v2_2/types"
	oldValidate "github.com/coreos/ignition/config/validate"
	"github.com/coreos/ignition/v2/config/v3_2/types"
	"github.com/coreos/ignition/v2/config/validate"

	"github.com/coreos/ign-converter/util"
)

// Translate translates Ignition spec config v3.2 to v2.2
func Translate(cfg types.Config) (old.Config, error) {
	rpt := validate.ValidateWithContext(cfg, nil)
	if rpt.IsFatal() {
		return old.Config{}, fmt.Errorf("Invalid input config:\n%s", rpt.String())
	}

	// Check for potential issues in the spec 3 config
	for _, m := range cfg.Ignition.Config.Merge {
		if m.Compression != nil {
			return old.Config{}, fmt.Errorf("Compression in Ignition.Config.Merge is not supported on 2.2")
		}
		if m.HTTPHeaders != nil {
			return old.Config{}, fmt.Errorf("HTTPHeaders in Ignition.Config.Merge are not supported on 2.2")
		}
	}

	if cfg.Ignition.Config.Replace.Compression != nil {
		return old.Config{}, fmt.Errorf("Compression in Ignition.Config.Replace is not supported on 2.2")
	}

	if cfg.Ignition.Config.Replace.HTTPHeaders != nil {
		return old.Config{}, fmt.Errorf("HTTPHeaders in Ignition.Config.Replace are not supported on 2.2")
	}

	for _, ca := range cfg.Ignition.Security.TLS.CertificateAuthorities {
		if ca.Compression != nil {
			return old.Config{}, fmt.Errorf("Compression in Ignition.Security.TLS.CertificateAuthorities is not supported on 2.2")
		}
		if ca.HTTPHeaders != nil {
			return old.Config{}, fmt.Errorf("HTTPHeaders in Ignition.Security.TLS.CertificateAuthorities are not supported on 2.2")
		}
	}

	if cfg.Ignition.Proxy.HTTPProxy != nil || cfg.Ignition.Proxy.HTTPSProxy != nil || cfg.Ignition.Proxy.NoProxy != nil {
		return old.Config{}, fmt.Errorf("HTTP proxies in Ignition.Proxy are not supported on 2.2")
	}

	if len(cfg.Storage.Luks) > 0 {
		return old.Config{}, fmt.Errorf("LUKS is not supported on 2.2")
	}

	// ShouldExist for Users & Groups do not exist in 2.2
	for _, u := range cfg.Passwd.Users {
		if u.ShouldExist != nil && !*u.ShouldExist {
			return old.Config{}, fmt.Errorf("ShouldExist in Passwd.Users is not supported on 2.2")
		}
	}
	for _, g := range cfg.Passwd.Groups {
		if g.ShouldExist != nil && !*g.ShouldExist {
			return old.Config{}, fmt.Errorf("ShouldExist in Passwd.Groups is not supported on 2.2")
		}
	}

	// Size and Start are sectors not MiB in 2.2, so we don't understand them.
	// Resize is not in 2.2
	// Fail for now
	for _, d := range cfg.Storage.Disks {
		for _, p := range d.Partitions {
			if p.SizeMiB != nil || p.StartMiB != nil {
				return old.Config{}, fmt.Errorf("SizeMiB and StartMiB in Storage.Disks.Partitions is not supported on 2.2")
			}
			if p.Resize != nil && *p.Resize {
				return old.Config{}, fmt.Errorf("Resize in Storage.Disks.Partitions is not supported on 2.2")
			}
		}
	}

	for _, fs := range cfg.Storage.Filesystems {
		if fs.MountOptions != nil {
			return old.Config{}, fmt.Errorf("MountOptions in Storage.Filesystems is not supported on 2.2")
		}
	}

	for _, f := range cfg.Storage.Files {
		if f.Contents.HTTPHeaders != nil {
			return old.Config{}, fmt.Errorf("HTTPHeaders in Storage.Files.Contents are not supported on 2.2")
		}
		for _, a := range f.Append {
			if a.HTTPHeaders != nil {
				return old.Config{}, fmt.Errorf("HTTPHeaders in Storage.Files.Append.* are not supported on 2.2")
			}
		}
	}

	// fsMap is a mapping of filesystems populated via the v3 config, to be
	// used for v2 files sections. The naming of each section will be uniquely
	// named by the path
	fsList := generateFsList(cfg.Storage.Filesystems)

	res := old.Config{
		// Ignition section
		Ignition: old.Ignition{
			Version: "2.2.0",
			Config: old.IgnitionConfig{
				Replace: translateCfgRef(cfg.Ignition.Config.Replace),
				Append:  translateCfgRefs(cfg.Ignition.Config.Merge),
			},
			Security: old.Security{
				TLS: old.TLS{
					CertificateAuthorities: translateCAs(cfg.Ignition.Security.TLS.CertificateAuthorities),
				},
			},
			Timeouts: old.Timeouts{
				HTTPResponseHeaders: cfg.Ignition.Timeouts.HTTPResponseHeaders,
				HTTPTotal:           cfg.Ignition.Timeouts.HTTPTotal,
			},
		},
		// Passwd section
		Passwd: old.Passwd{
			Users:  translateUsers(cfg.Passwd.Users),
			Groups: translateGroups(cfg.Passwd.Groups),
		},
		Systemd: old.Systemd{
			Units: translateUnits(cfg.Systemd.Units),
		},
		Storage: old.Storage{
			Disks:       translateDisks(cfg.Storage.Disks),
			Raid:        translateRaid(cfg.Storage.Raid),
			Filesystems: translateFilesystems(cfg.Storage.Filesystems),
			Files:       translateFiles(cfg.Storage.Files, fsList),
			Directories: translateDirectories(cfg.Storage.Directories, fsList),
			Links:       translateLinks(cfg.Storage.Links, fsList),
		},
	}

	// Sanity check the returned config
	oldrpt := oldValidate.ValidateWithoutSource(reflect.ValueOf(res))
	if oldrpt.IsFatal() {
		return old.Config{}, fmt.Errorf("Converted spec has unexpected fatal error:\n%s", oldrpt.String())
	}
	return res, nil
}

func generateFsList(fss []types.Filesystem) (ret []string) {
	for _, f := range fss {
		if f.Path == nil {
			// Spec 3 has defined the filesystem but has no path, which means we will not be writing files/dirs to it
			continue
		}
		ret = append(ret, *f.Path)
	}
	return
}

func translateCfgRef(ref types.Resource) (ret *old.ConfigReference) {
	if ref.Source == nil {
		return
	}
	ret = &old.ConfigReference{}
	ret.Source = util.StrV(ref.Source)
	ret.Verification.Hash = ref.Verification.Hash
	return
}

func translateCfgRefs(refs []types.Resource) (ret []old.ConfigReference) {
	for _, ref := range refs {
		ret = append(ret, *translateCfgRef(ref))
	}
	return
}

func translateCAs(refs []types.Resource) (ret []old.CaReference) {
	for _, ref := range refs {
		ret = append(ret, old.CaReference{
			Source: *ref.Source,
			Verification: old.Verification{
				Hash: ref.Verification.Hash,
			},
		})
	}
	return
}

func translateUsers(users []types.PasswdUser) (ret []old.PasswdUser) {
	for _, u := range users {
		ret = append(ret, old.PasswdUser{
			Name:              u.Name,
			PasswordHash:      u.PasswordHash,
			SSHAuthorizedKeys: translateUserSSH(u.SSHAuthorizedKeys),
			UID:               u.UID,
			Gecos:             util.StrV(u.Gecos),
			HomeDir:           util.StrV(u.HomeDir),
			NoCreateHome:      util.BoolV(u.NoCreateHome),
			PrimaryGroup:      util.StrV(u.PrimaryGroup),
			Groups:            translateUserGroups(u.Groups),
			NoUserGroup:       util.BoolV(u.NoUserGroup),
			NoLogInit:         util.BoolV(u.NoLogInit),
			Shell:             util.StrV(u.Shell),
			System:            util.BoolV(u.System),
		})
	}
	return
}

func translateUserSSH(in []types.SSHAuthorizedKey) (ret []old.SSHAuthorizedKey) {
	for _, k := range in {
		ret = append(ret, old.SSHAuthorizedKey(k))
	}
	return
}

func translateUserGroups(in []types.Group) (ret []old.Group) {
	for _, g := range in {
		ret = append(ret, old.Group(g))
	}
	return
}

func translateGroups(groups []types.PasswdGroup) (ret []old.PasswdGroup) {
	for _, g := range groups {
		ret = append(ret, old.PasswdGroup{
			Name:         g.Name,
			Gid:          g.Gid,
			PasswordHash: util.StrV(g.PasswordHash),
			System:       util.BoolV(g.System),
		})
	}
	return
}

func translateUnits(units []types.Unit) (ret []old.Unit) {
	for _, u := range units {
		ret = append(ret, old.Unit{
			Name:     u.Name,
			Enabled:  u.Enabled,
			Mask:     util.BoolV(u.Mask),
			Contents: util.StrV(u.Contents),
			Dropins:  translateDropins(u.Dropins),
		})
	}
	return
}

func translateDropins(dropins []types.Dropin) (ret []old.SystemdDropin) {
	for _, d := range dropins {
		ret = append(ret, old.SystemdDropin{
			Name:     d.Name,
			Contents: util.StrV(d.Contents),
		})
	}
	return
}

func translateDisks(disks []types.Disk) (ret []old.Disk) {
	for _, d := range disks {
		ret = append(ret, old.Disk{
			Device:     d.Device,
			WipeTable:  util.BoolV(d.WipeTable),
			Partitions: translatePartitions(d.Partitions),
		})
	}
	return
}

func translatePartitions(parts []types.Partition) (ret []old.Partition) {
	for _, p := range parts {
		ret = append(ret, old.Partition{
			Label:    util.StrV(p.Label),
			Number:   p.Number,
			TypeGUID: util.StrV(p.TypeGUID),
			GUID:     util.StrV(p.GUID),
		})
	}
	return
}

func translateRaid(raids []types.Raid) (ret []old.Raid) {
	for _, r := range raids {
		ret = append(ret, old.Raid{
			Name:    r.Name,
			Level:   r.Level,
			Devices: translateDevices(r.Devices),
			Spares:  util.IntV(r.Spares),
			Options: translateRaidOptions(r.Options),
		})
	}
	return
}

func translateDevices(devices []types.Device) (ret []old.Device) {
	for _, d := range devices {
		ret = append(ret, old.Device(d))
	}
	return
}

func translateRaidOptions(options []types.RaidOption) (ret []old.RaidOption) {
	for _, o := range options {
		ret = append(ret, old.RaidOption(o))
	}
	return
}

func translateFilesystems(fss []types.Filesystem) (ret []old.Filesystem) {
	// For filesystems that have no explicit path, we will uniquely name them with an int instead
	inc := 1
	for _, f := range fss {
		var fsname string
		if f.Path == nil {
			fsname = strconv.Itoa(inc)
			inc++
		} else {
			fsname = *f.Path
		}

		ret = append(ret, old.Filesystem{
			// To construct a mapping for files/directories, we name the filesystem by path uniquely.
			// TODO: check if its ok to leave out "Path" since we are mapping it via Name later
			Name: fsname,
			Mount: &old.Mount{
				Device:         f.Device,
				Format:         util.StrV(f.Format),
				WipeFilesystem: util.BoolV(f.WipeFilesystem),
				Label:          f.Label,
				UUID:           f.UUID,
				Options:        translateFilesystemOptions(f.Options),
			},
		})
	}
	return
}

func translateFilesystemOptions(options []types.FilesystemOption) (ret []old.MountOption) {
	for _, o := range options {
		ret = append(ret, old.MountOption(o))
	}
	return
}

func translateNode(n types.Node, fss []string) old.Node {
	fsname := ""
	path := n.Path
	for _, fs := range fss {
		if strings.HasPrefix(n.Path, fs) && len(fs) > len(fsname) {
			fsname = fs
			path = strings.TrimPrefix(n.Path, fsname)
		}
	}
	if len(fsname) == 0 {
		fsname = "root"
	}

	ret := old.Node{
		Filesystem: fsname,
		Path:       path,
		Overwrite:  n.Overwrite,
	}
	if n.User != (types.NodeUser{}) {
		ret.User = &old.NodeUser{
			ID:   n.User.ID,
			Name: util.StrV(n.User.Name),
		}
	}
	if n.Group != (types.NodeGroup{}) {
		ret.Group = &old.NodeGroup{
			ID:   n.Group.ID,
			Name: util.StrV(n.Group.Name),
		}
	}
	return ret
}

func translateFiles(files []types.File, fss []string) (ret []old.File) {
	for _, f := range files {
		file := old.File{
			Node: translateNode(f.Node, fss),
			FileEmbedded1: old.FileEmbedded1{
				Mode: f.Mode,
			},
		}

		// Overwrite defaults to false in spec 3 and true in spec 2;
		// we want to retain the "unset" default of spec 3 when translating down,
		// so we're defaulting to false
		if f.Node.Overwrite == nil {
			file.Node.Overwrite = util.BoolPStrict(false)
		}

		if f.FileEmbedded1.Contents.Source != nil {
			file.FileEmbedded1.Contents = old.FileContents{
				Compression: util.StrV(f.Contents.Compression),
				Source:      util.StrV(f.Contents.Source),
			}
			file.FileEmbedded1.Contents.Verification.Hash = f.FileEmbedded1.Contents.Verification.Hash
			file.FileEmbedded1.Append = false
			ret = append(ret, file)
		}
		if f.FileEmbedded1.Append != nil {
			for _, fc := range f.FileEmbedded1.Append {
				appendFile := old.File{
					Node:          file.Node,
					FileEmbedded1: file.FileEmbedded1,
				}
				appendFile.FileEmbedded1.Contents = old.FileContents{
					Compression: util.StrV(fc.Compression),
					Source:      util.StrV(fc.Source),
				}
				appendFile.FileEmbedded1.Contents.Verification.Hash = fc.Verification.Hash
				appendFile.FileEmbedded1.Append = true
				// In spec 3, we may have a file object with overwrite true, contents, and some appends.
				// When the appended files are split out to separate file objects for spec 2,
				// the append false object may still have overwrite true,
				// but the append true objects must have overwrite false in spec 2.
				appendFile.Node.Overwrite = util.BoolPStrict(false)
				ret = append(ret, appendFile)
			}
		}
	}
	return
}

func translateLinks(links []types.Link, fss []string) (ret []old.Link) {
	for _, l := range links {
		ret = append(ret, old.Link{
			Node: translateNode(l.Node, fss),
			LinkEmbedded1: old.LinkEmbedded1{
				Hard:   util.BoolV(l.Hard),
				Target: l.Target,
			},
		})
	}
	return
}

func translateDirectories(dirs []types.Directory, fss []string) (ret []old.Directory) {
	for _, d := range dirs {
		ret = append(ret, old.Directory{
			Node: translateNode(d.Node, fss),
			DirectoryEmbedded1: old.DirectoryEmbedded1{
				Mode: d.Mode,
			},
		})
	}
	return
}
