// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package topo_test

// batageljZaversnikGraph is the example graph from
// figure 1 of http://arxiv.org/abs/cs/0310049v1
var batageljZaversnikGraph = []set{
	0: nil,

	1: linksTo(2, 3),
	2: linksTo(4),
	3: linksTo(4),
	4: linksTo(5),
	5: nil,

	6:  linksTo(7, 8, 14),
	7:  linksTo(8, 11, 12, 14),
	8:  linksTo(14),
	9:  linksTo(11),
	10: linksTo(11),
	11: linksTo(12),
	12: linksTo(18),
	13: linksTo(14, 15),
	14: linksTo(15, 17),
	15: linksTo(16, 17),
	16: nil,
	17: linksTo(18, 19, 20),
	18: linksTo(19, 20),
	19: linksTo(20),
	20: nil,
}

// set is an integer set.
type set map[int]struct{}

func linksTo(i ...int) set {
	if len(i) == 0 {
		return nil
	}
	s := make(set)
	for _, v := range i {
		s[v] = struct{}{}
	}
	return s
}
