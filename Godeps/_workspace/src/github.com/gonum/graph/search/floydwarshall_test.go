// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package search_test

import (
	"github.com/gonum/graph/concrete"
	"github.com/gonum/graph/search"
	"math"
	"testing"
)

func TestFWOneEdge(t *testing.T) {
	dg := concrete.NewDenseGraph(2, true)
	aPaths, sPath := search.FloydWarshall(dg, nil)

	path, cost, err := sPath(concrete.Node(0), concrete.Node(1))
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(cost-1) > 1e-6 {
		t.Errorf("FW got wrong cost %f", cost)
	}

	if len(path) != 2 || path[0].ID() != 0 && path[1].ID() != 1 {
		t.Errorf("Wrong path in FW %v", path)
	}

	paths, cost, err := aPaths(concrete.Node(0), concrete.Node(1))
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(cost-1) > 1e-6 {
		t.Errorf("FW got wrong cost %f", cost)
	}

	if len(paths) != 1 {
		t.Errorf("Didn't get right paths in FW %v", paths)
	}

	path = paths[0]
	if len(path) != 2 || path[0].ID() != 0 && path[1].ID() != 1 {
		t.Errorf("Wrong path in FW allpaths %v", path)
	}
}

func TestFWTwoPaths(t *testing.T) {
	dg := concrete.NewDenseGraph(5, false)
	// Adds two paths from 0->2 of equal length
	dg.SetEdgeCost(concrete.Edge{concrete.Node(0), concrete.Node(2)}, 2, true)
	dg.SetEdgeCost(concrete.Edge{concrete.Node(0), concrete.Node(1)}, 1, true)
	dg.SetEdgeCost(concrete.Edge{concrete.Node(1), concrete.Node(2)}, 1, true)

	aPaths, sPath := search.FloydWarshall(dg, nil)
	path, cost, err := sPath(concrete.Node(0), concrete.Node(2))
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(cost-2) > .00001 {
		t.Errorf("Path has incorrect cost, %f", cost)
	}

	if len(path) == 2 && path[0].ID() == 0 && path[1].ID() == 2 {
		t.Logf("Got correct path: %v", path)
	} else if len(path) == 3 && path[0].ID() == 0 && path[1].ID() == 1 && path[2].ID() == 2 {
		t.Logf("Got correct path %v", path)
	} else {
		t.Errorf("Got wrong path %v", path)
	}

	paths, cost, err := aPaths(concrete.Node(0), concrete.Node(2))

	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(cost-2) > .00001 {
		t.Errorf("All paths function gets incorrect cost, %f", cost)
	}

	if len(paths) != 2 {
		t.Fatalf("Didn't get all shortest paths %v", paths)
	}

	for _, path := range paths {
		if len(path) == 2 && path[0].ID() == 0 && path[1].ID() == 2 {
			t.Logf("Got correct path for all paths: %v", path)
		} else if len(path) == 3 && path[0].ID() == 0 && path[1].ID() == 1 && path[2].ID() == 2 {
			t.Logf("Got correct path for all paths %v", path)
		} else {
			t.Errorf("Got wrong path for all paths %v", path)
		}
	}
}

// Tests with multiple right paths, but also one dead-end path
// and one path that reaches the goal, but not optimally
func TestFWConfoundingPath(t *testing.T) {
	dg := concrete.NewDenseGraph(6, false)

	// Add a path from 0->5 of cost 4
	dg.SetEdgeCost(concrete.Edge{concrete.Node(0), concrete.Node(1)}, 1, true)
	dg.SetEdgeCost(concrete.Edge{concrete.Node(1), concrete.Node(2)}, 1, true)
	dg.SetEdgeCost(concrete.Edge{concrete.Node(2), concrete.Node(3)}, 1, true)
	dg.SetEdgeCost(concrete.Edge{concrete.Node(3), concrete.Node(5)}, 1, true)

	// Add direct edge to goal of cost 4
	dg.SetEdgeCost(concrete.Edge{concrete.Node(0), concrete.Node(5)}, 4, true)

	// Add edge to a node that's still optimal
	dg.SetEdgeCost(concrete.Edge{concrete.Node(0), concrete.Node(2)}, 2, true)

	// Add edge to 3 that's overpriced
	dg.SetEdgeCost(concrete.Edge{concrete.Node(0), concrete.Node(3)}, 4, true)

	// Add very cheap edge to 4 which is a dead end
	dg.SetEdgeCost(concrete.Edge{concrete.Node(0), concrete.Node(4)}, 0.25, true)

	aPaths, sPath := search.FloydWarshall(dg, nil)

	path, cost, err := sPath(concrete.Node(0), concrete.Node(5))
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(cost-4) > 1e-6 {
		t.Errorf("Incorrect cost %f", cost)
	}

	if len(path) == 5 && path[0].ID() == 0 && path[1].ID() == 1 && path[2].ID() == 2 && path[3].ID() == 3 && path[4].ID() == 5 {
		t.Logf("Correct path found for single path %v", path)
	} else if len(path) == 2 && path[0].ID() == 0 && path[1].ID() == 5 {
		t.Logf("Correct path found for single path %v", path)
	} else if len(path) == 4 && path[0].ID() == 0 && path[1].ID() == 2 && path[2].ID() == 3 && path[3].ID() == 5 {
		t.Logf("Correct path found for single path %v", path)
	} else {
		t.Errorf("Wrong path found for single path %v", path)
	}

	paths, cost, err := aPaths(concrete.Node(0), concrete.Node(5))
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(cost-4) > 1e-6 {
		t.Errorf("Incorrect cost %f", cost)
	}

	if len(paths) != 3 {
		t.Errorf("Wrong paths gotten for all paths %v", paths)
	}

	for _, path := range paths {
		if len(path) == 5 && path[0].ID() == 0 && path[1].ID() == 1 && path[2].ID() == 2 && path[3].ID() == 3 && path[4].ID() == 5 {
			t.Logf("Correct path found for multi path %v", path)
		} else if len(path) == 2 && path[0].ID() == 0 && path[1].ID() == 5 {
			t.Logf("Correct path found for multi path %v", path)
		} else if len(path) == 4 && path[0].ID() == 0 && path[1].ID() == 2 && path[2].ID() == 3 && path[3].ID() == 5 {
			t.Logf("Correct path found for multi path %v", path)
		} else {
			t.Errorf("Wrong path found for multi path %v", path)
		}
	}

	path, _, err = sPath(concrete.Node(4), concrete.Node(5))
	if err != nil {
		t.Log("Success!", err)
	} else {
		t.Errorf("Path was found by FW single path where one shouldn't be %v", path)
	}

	paths, _, err = aPaths(concrete.Node(4), concrete.Node(5))
	if err != nil {
		t.Log("Success!", err)
	} else {
		t.Errorf("Path was found by FW multi-path where one shouldn't be %v", paths)
	}
}
