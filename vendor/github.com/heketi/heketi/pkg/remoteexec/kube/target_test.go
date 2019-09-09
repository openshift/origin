//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kube

import (
	"testing"

	"github.com/heketi/tests"
)

func TestTargetPodDesc(t *testing.T) {
	tgt := TargetPod{
		target: target{
			Namespace: "ducks",
		},
		PodName: "daffy",
	}
	tests.Assert(t, tgt.String() == "pod:daffy ns:ducks", "got:", tgt.String())
}

func TestTargetLabelDesc(t *testing.T) {
	tgt := TargetLabel{
		Key:   "foo",
		Value: "bar",
	}
	tests.Assert(t, tgt.String() == "label:foo=bar", "got:", tgt.String())
}

func TestTargetDaemonSetDesc(t *testing.T) {
	tgt := TargetDaemonSet{
		Host:     "n1.example.com",
		Selector: "fish",
	}
	tests.Assert(t, tgt.String() == "host:n1.example.com selector:fish", "got:", tgt.String())
}

func TestTargetPodDescWithOrigin(t *testing.T) {
	tgt1 := TargetDaemonSet{
		Host:     "n2.example.com",
		Selector: "birds",
	}
	tgt := TargetPod{
		target: target{
			Namespace: "ducks",
		},
		PodName: "donald",
		origin:  tgt1,
	}
	tests.Assert(t,
		tgt.String() == "pod:donald ns:ducks (from host:n2.example.com selector:birds)",
		"got:", tgt.String())
}

func TestTargetContainerDescWithOrigin(t *testing.T) {
	tgt1 := TargetDaemonSet{
		Host:     "n2.example.com",
		Selector: "birds",
	}
	tgt := TargetContainer{
		TargetPod: TargetPod{
			target: target{
				Namespace: "ducks",
			},
			PodName: "donald",
			origin:  tgt1,
		},
		ContainerName: "cap",
	}
	tests.Assert(t,
		tgt.String() == "pod:donald c:cap ns:ducks (from host:n2.example.com selector:birds)",
		"got:", tgt.String())
}
