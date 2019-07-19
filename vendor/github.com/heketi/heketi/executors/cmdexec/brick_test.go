//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package cmdexec

import (
	"strings"
	"testing"

	"github.com/heketi/heketi/executors"
	conv "github.com/heketi/heketi/pkg/conversions"
	"github.com/heketi/heketi/pkg/paths"
	rex "github.com/heketi/heketi/pkg/remoteexec"
	"github.com/heketi/tests"
)

func doTestSshExecBrickCreate(t *testing.T, f *CommandFaker, s *FakeExecutor) {
	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
		Path:             paths.BrickPath("xvgid", "id"),
		TpName:           "tp_id",
		LvName:           "brick_id",
	}

	// Mock ssh function
	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) (rex.Results, error) {

		tests.Assert(t, host == "myhost:100", host)
		tests.Assert(t, len(commands) == 6)

		for i, cmd := range commands {
			cmd = strings.Trim(cmd, " ")
			switch i {
			case 0:
				tests.Assert(t,
					cmd == "mkdir -p /var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 1:
				tests.Assert(t,
					cmd == "lvcreate -qq --autobackup="+conv.BoolToYN(s.BackupLVM)+" --poolmetadatasize 5K "+
						"--chunksize 256K --size 100K --thin vg_xvgid/tp_id --virtualsize 10K --name brick_id", cmd)

			case 2:
				tests.Assert(t,
					cmd == "mkfs.xfs -i size=512 "+
						"-n size=8192 /dev/mapper/vg_xvgid-brick_id", cmd)

			case 3:
				tests.Assert(t,
					cmd == "awk \"BEGIN {print \\\"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id "+
						"xfs rw,inode64,noatime,nouuid 1 2\\\" "+
						">> \\\"/my/fstab\\\"}\"", cmd)

			case 4:
				tests.Assert(t,
					cmd == "mount -o rw,inode64,noatime,nouuid "+
						"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 5:
				tests.Assert(t,
					cmd == "mkdir "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)
			}
		}

		return nil, nil
	}

	// Create Brick
	_, err := s.BrickCreate("myhost", b)
	tests.Assert(t, err == nil, err)

}

func TestSshExecBrickCreate(t *testing.T) {
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.portStr = "100"

	doTestSshExecBrickCreate(t, f, s)
}

func TestSshExecBrickCreateWithGid(t *testing.T) {
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.portStr = "100"

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
		Gid:              1234,
		Path:             paths.BrickPath("xvgid", "id"),
		TpName:           "tp_id",
		LvName:           "brick_id",
	}

	// Mock ssh function
	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) (rex.Results, error) {

		tests.Assert(t, host == "myhost:100", host)
		tests.Assert(t, len(commands) == 8)

		for i, cmd := range commands {
			cmd = strings.Trim(cmd, " ")
			switch i {
			case 0:
				tests.Assert(t,
					cmd == "mkdir -p /var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 1:
				tests.Assert(t,
					cmd == "lvcreate -qq --autobackup="+conv.BoolToYN(s.BackupLVM)+" --poolmetadatasize 5K "+
						"--chunksize 256K --size 100K --thin vg_xvgid/tp_id --virtualsize 10K --name brick_id", cmd)

			case 2:
				tests.Assert(t,
					cmd == "mkfs.xfs -i size=512 "+
						"-n size=8192 /dev/mapper/vg_xvgid-brick_id", cmd)

			case 3:
				tests.Assert(t,
					cmd == "awk \"BEGIN {print \\\"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id "+
						"xfs rw,inode64,noatime,nouuid 1 2\\\" "+
						">> \\\"/my/fstab\\\"}\"", cmd)

			case 4:
				tests.Assert(t,
					cmd == "mount -o rw,inode64,noatime,nouuid "+
						"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 5:
				tests.Assert(t,
					cmd == "mkdir "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)

			case 6:
				tests.Assert(t,
					cmd == "chown :1234 "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)

			case 7:
				tests.Assert(t,
					cmd == "chmod 2775 "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)
			}
		}

		return nil, nil
	}

	// Create Brick
	_, err = s.BrickCreate("myhost", b)
	tests.Assert(t, err == nil, err)

}

func TestSshExecBrickCreateSudo(t *testing.T) {
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.useSudo = true
	s.portStr = "100"

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
		Path:             paths.BrickPath("xvgid", "id"),
		TpName:           "tp_id",
		LvName:           "brick_id",
	}

	// Mock ssh function
	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) (rex.Results, error) {

		tests.Assert(t, host == "myhost:100", host)
		tests.Assert(t, len(commands) == 6)
		tests.Assert(t, useSudo == true)

		for i, cmd := range commands {
			cmd = strings.Trim(cmd, " ")
			switch i {
			case 0:
				tests.Assert(t,
					cmd == "mkdir -p /var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 1:
				tests.Assert(t,
					cmd == "lvcreate -qq --autobackup="+conv.BoolToYN(s.BackupLVM)+" --poolmetadatasize 5K "+
						"--chunksize 256K --size 100K --thin vg_xvgid/tp_id --virtualsize 10K --name brick_id", cmd)

			case 2:
				tests.Assert(t,
					cmd == "mkfs.xfs -i size=512 "+
						"-n size=8192 /dev/mapper/vg_xvgid-brick_id", cmd)

			case 3:
				tests.Assert(t,
					cmd == "awk \"BEGIN {print \\\"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id "+
						"xfs rw,inode64,noatime,nouuid 1 2\\\" "+
						">> \\\"/my/fstab\\\"}\"", cmd)

			case 4:
				tests.Assert(t,
					cmd == "mount -o rw,inode64,noatime,nouuid "+
						"/dev/mapper/vg_xvgid-brick_id "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case 5:
				tests.Assert(t,
					cmd == "mkdir "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id/brick", cmd)
			}
		}

		return nil, nil
	}

	// Create Brick
	_, err = s.BrickCreate("myhost", b)
	tests.Assert(t, err == nil, err)

}

func TestSshExecBrickCreateBackupLVM(t *testing.T) {
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.portStr = "100"
	s.BackupLVM = true

	doTestSshExecBrickCreate(t, f, s)
}

func TestSshExecBrickDestroy(t *testing.T) {
	f := NewCommandFaker()
	s, err := NewFakeExecutor(f)
	tests.Assert(t, err == nil)
	tests.Assert(t, s != nil)
	s.portStr = "100"

	// Create a Brick
	b := &executors.BrickRequest{
		VgId:             "xvgid",
		Name:             "id",
		TpSize:           100,
		Size:             10,
		PoolMetadataSize: 5,
		Path:             strings.TrimSuffix(paths.BrickPath("xvgid", "id"), "/brick"),
		TpName:           "tp_id",
		LvName:           "brick_id",
	}

	// Mock ssh function
	f.FakeConnectAndExec = func(host string,
		commands []string,
		timeoutMinutes int,
		useSudo bool) (rex.Results, error) {

		tests.Assert(t, host == "myhost:100", host)

		for _, cmd := range commands {
			cmd = strings.Trim(cmd, " ")
			switch {
			case strings.HasPrefix(cmd, "mount"):
				tests.Assert(t,
					cmd == "mount | grep -w "+b.Path+" | cut -d\" \" -f1", cmd)
				// return the device that was mounted
				output := fakeResults("/dev/vg_xvgid/brick_id", "")
				return output[0:1], nil

			case strings.Contains(cmd, "lvs") && strings.Contains(cmd, "vg_name"):
				tests.Assert(t,
					cmd == "lvs --noheadings --separator=/ "+
						"-ovg_name,pool_lv /dev/vg_xvgid/brick_id", cmd)
				// return the device that was mounted
				output := fakeResults("vg_xvgid/tp_id", "")
				return output[0:1], nil

			case strings.Contains(cmd, "lvs") && strings.Contains(cmd, "thin_count"):
				tests.Assert(t,
					cmd == "lvs --noheadings --options=thin_count vg_xvgid/tp_id", cmd)
				// return the number of thin-p users
				output := fakeResults("0", "")
				return output[0:1], nil

			case strings.Contains(cmd, "umount"):
				tests.Assert(t,
					cmd == "umount "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case strings.Contains(cmd, "lvremove"):
				tests.Assert(t,
					cmd == "lvremove --autobackup="+conv.BoolToYN(s.BackupLVM)+" -f vg_xvgid/tp_id" ||
						cmd == "lvremove --autobackup="+conv.BoolToYN(s.BackupLVM)+" -f vg_xvgid/brick_id", cmd)

			case strings.Contains(cmd, "rmdir"):
				tests.Assert(t,
					cmd == "rmdir "+
						"/var/lib/heketi/mounts/vg_xvgid/brick_id", cmd)

			case strings.Contains(cmd, "sed"):
				tests.Assert(t,
					cmd == "sed -i.save "+
						"\"/brick_id/d\" /my/fstab", cmd)
			}
		}

		return nil, nil
	}

	// Create Brick
	_, err = s.BrickDestroy("myhost", b)
	tests.Assert(t, err == nil, err)
}

func fakeResults(f ...string) rex.Results {
	results := make(rex.Results, len(f))
	for i, s := range f {
		results[i].Output = s
		results[i].Completed = true
	}
	return results
}
