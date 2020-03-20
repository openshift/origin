// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package layout

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/plot/cmpimg"
)

// orderedGraph wraps a graph.Graph ensuring consistent ordering of nodes
// in graph queries. Removal of this causes to tests to fail due to changes
// in node iteration order, but the produced graph layouts are still good.
type orderedGraph struct {
	graph.Graph
}

func (g orderedGraph) Nodes() graph.Nodes {
	n := graph.NodesOf(g.Graph.Nodes())
	sort.Sort(ordered.ByID(n))
	return iterator.NewOrderedNodes(n)
}

func (g orderedGraph) From(id int64) graph.Nodes {
	n := graph.NodesOf(g.Graph.From(id))
	sort.Sort(ordered.ByID(n))
	return iterator.NewOrderedNodes(n)
}

func goldenPath(path string) string {
	ext := filepath.Ext(path)
	noext := strings.TrimSuffix(path, ext)
	return noext + "_golden" + ext
}

func checkRenderedLayout(t *testing.T, path string) (ok bool) {
	if *cmpimg.GenerateTestData {
		// Recreate Golden image and exit.
		golden := goldenPath(path)
		_ = os.Remove(golden)
		if err := os.Rename(path, golden); err != nil {
			t.Fatal(err)
		}
		return true
	}

	// Read the images we've just generated and check them against the
	// Golden Images.
	got, err := ioutil.ReadFile(path)
	if err != nil {
		t.Errorf("failed to read %s: %v", path, err)
		return true
	}
	golden := goldenPath(path)
	want, err := ioutil.ReadFile(golden)
	if err != nil {
		t.Errorf("failed to read golden file %s: %v", golden, err)
		return true
	}
	typ := filepath.Ext(path)[1:] // remove the dot in ".png"
	ok, err = cmpimg.Equal(typ, got, want)
	if err != nil {
		t.Errorf("failed to compare image for %s: %v", path, err)
		return true
	}
	if !ok {
		t.Errorf("image mismatch for %s\n", path)
		v1, _, err := image.Decode(bytes.NewReader(got))
		if err != nil {
			t.Errorf("failed to decode %s: %v", path, err)
			return false
		}
		v2, _, err := image.Decode(bytes.NewReader(want))
		if err != nil {
			t.Errorf("failed to decode %s: %v", golden, err)
			return false
		}

		dst := image.NewRGBA64(v1.Bounds().Union(v2.Bounds()))
		rect := cmpimg.Diff(dst, v1, v2)
		t.Logf("image bounds union:%+v diff bounds intersection:%+v", dst.Bounds(), rect)

		var buf bytes.Buffer
		err = png.Encode(&buf, dst)
		if err != nil {
			t.Errorf("failed to encode difference png: %v", err)
			return false
		}
		t.Log("IMAGE:" + base64.StdEncoding.EncodeToString(buf.Bytes()))
	}
	return ok
}
