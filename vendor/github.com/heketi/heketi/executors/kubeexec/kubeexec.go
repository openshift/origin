//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kubeexec

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/executors/cmdexec"
	"github.com/heketi/heketi/pkg/kubernetes"
	"github.com/heketi/heketi/pkg/logging"
	rex "github.com/heketi/heketi/pkg/remoteexec"
	"github.com/heketi/heketi/pkg/remoteexec/kube"
)

const (
	KubeGlusterFSPodLabelKey = "glusterfs-node"
)

type KubeExecutor struct {
	cmdexec.CmdExecutor

	// save kube configuration
	config    *KubeConfig
	namespace string
	kconn     *kube.KubeConn
}

var (
	logger = logging.NewLogger("[kubeexec]", logging.LEVEL_DEBUG)

	// if not specifified in the config json
	DefaultMaxConnThreshold uint64 = 64
)

func setWithEnvVariables(config *KubeConfig) {
	var env string

	// Namespace / Project
	env = os.Getenv("HEKETI_KUBE_NAMESPACE")
	if "" != env {
		config.Namespace = env
	}

	// FSTAB
	env = os.Getenv("HEKETI_FSTAB")
	if "" != env {
		config.Fstab = env
	}

	// Snapshot Limit
	env = os.Getenv("HEKETI_SNAPSHOT_LIMIT")
	if "" != env {
		i, err := strconv.Atoi(env)
		if err == nil {
			config.SnapShotLimit = i
		}
	}

	// Determine if Heketi should communicate with Gluster
	// pods deployed by a DaemonSet
	env = os.Getenv("HEKETI_KUBE_GLUSTER_DAEMONSET")
	if "" != env {
		env = strings.ToLower(env)
		if env[0] == 'y' || env[0] == '1' {
			config.GlusterDaemonSet = true
		} else if env[0] == 'n' || env[0] == '0' {
			config.GlusterDaemonSet = false
		}
	}

	// Use POD names
	env = os.Getenv("HEKETI_KUBE_USE_POD_NAMES")
	if "" != env {
		env = strings.ToLower(env)
		if env[0] == 'y' || env[0] == '1' {
			config.UsePodNames = true
		} else if env[0] == 'n' || env[0] == '0' {
			config.UsePodNames = false
		}
	}
}

func NewKubeExecutor(config *KubeConfig) (*KubeExecutor, error) {
	// Override configuration
	setWithEnvVariables(config)

	// Initialize
	k := &KubeExecutor{}
	k.config = config
	k.CmdExecutor.Init(&config.CmdConfig)
	k.RemoteExecutor = k

	if k.config.Fstab == "" {
		k.Fstab = "/etc/fstab"
	} else {
		k.Fstab = config.Fstab
	}

	var err error
	// if unset, get namespace
	k.namespace = k.config.Namespace
	if k.namespace == "" {
		k.namespace, err = kubernetes.GetNamespace()
		if err != nil {
			return nil, fmt.Errorf("Namespace must be provided in configuration: %v", err)
		}
	}

	k.BackupLVM = config.BackupLVM
	k.kconn, err = kube.NewKubeConn(logger)
	if err != nil {
		return nil, err
	}
	if config.MaxConnections == 0 {
		k.kconn.MaxConnThreshold = DefaultMaxConnThreshold
	} else if config.MaxConnections > 0 {
		k.kconn.MaxConnThreshold = uint64(config.MaxConnections)
	} else {
		k.kconn.MaxConnThreshold = 0
	}

	godbc.Ensure(k != nil)
	godbc.Ensure(k.Fstab != "")

	return k, nil
}

func (k *KubeExecutor) ExecCommands(
	host string, commands rex.Cmds,
	timeoutMinutes int) (rex.Results, error) {

	// Throttle
	k.AccessConnection(host)
	defer k.FreeConnection(host)

	// Get target pod
	var (
		pod kube.TargetPod
		err error
	)
	if k.config.UsePodNames {
		pod.Namespace = k.config.Namespace
		pod.PodName = host
	} else if k.config.GlusterDaemonSet {
		tgt := kube.TargetDaemonSet{}
		tgt.Namespace = k.config.Namespace
		tgt.Host = host
		tgt.Selector = KubeGlusterFSPodLabelKey
		pod, err = tgt.GetTargetPod(k.kconn)
	} else {
		tgt := kube.TargetLabel{}
		tgt.Namespace = k.config.Namespace
		tgt.Key = KubeGlusterFSPodLabelKey
		tgt.Value = host
		pod, err = tgt.GetTargetPod(k.kconn)
	}
	if err != nil {
		return nil, err
	}

	// Get target container
	tc, err := pod.FirstContainer(k.kconn)
	if err != nil {
		return nil, err
	}

	return kube.ExecCommands(k.kconn, tc, commands, kube.TimeoutOptions{
		TimeoutMinutes:   timeoutMinutes,
		UseTimeoutPrefix: !k.config.DisableTimeoutPrefix,
	})
}

func (k *KubeExecutor) RebalanceOnExpansion() bool {
	return k.config.RebalanceOnExpansion
}

func (k *KubeExecutor) SnapShotLimit() int {
	return k.config.SnapShotLimit
}
