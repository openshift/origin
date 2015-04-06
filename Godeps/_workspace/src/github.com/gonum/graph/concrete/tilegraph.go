// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concrete

import (
	"errors"
	"strings"

	"github.com/gonum/graph"
)

type TileGraph struct {
	tiles            []bool
	numRows, numCols int
}

func NewTileGraph(dimX, dimY int, isPassable bool) *TileGraph {
	tiles := make([]bool, dimX*dimY)
	if isPassable {
		for i := range tiles {
			tiles[i] = true
		}
	}

	return &TileGraph{
		tiles:   tiles,
		numRows: dimX,
		numCols: dimY,
	}
}

func GenerateTileGraph(template string) (*TileGraph, error) {
	rows := strings.Split(strings.TrimSpace(template), "\n")

	tiles := make([]bool, 0)

	colCheck := -1
	for _, colString := range rows {
		colCount := 0
		cols := strings.NewReader(colString)
		for cols.Len() != 0 {
			colCount += 1
			ch, _, err := cols.ReadRune()
			if err != nil {
				return nil, errors.New("Error while reading rune from input string")
			}

			switch ch {
			case '\u2580':
				tiles = append(tiles, false)
			case ' ':
				tiles = append(tiles, true)
			default:
				return nil, errors.New("Unrecognized character while reading input string")
			}
		}

		if colCheck == -1 {
			colCheck = colCount
		} else if colCheck != colCount {
			return nil, errors.New("Jagged rows, cannot generate graph.")
		}
	}

	return &TileGraph{
		tiles:   tiles,
		numRows: len(rows),
		numCols: colCheck,
	}, nil
}

func (g *TileGraph) SetPassability(row, col int, passability bool) {
	loc := row*g.numCols + col
	if loc >= len(g.tiles) || row < 0 || col < 0 {
		return
	}

	g.tiles[loc] = passability
}

func (g *TileGraph) String() string {
	var outString string
	for r := 0; r < g.numRows; r++ {
		for c := 0; c < g.numCols; c++ {
			if g.tiles[r*g.numCols+c] == false {
				outString += "\u2580" // Black square
			} else {
				outString += " " // Space
			}
		}

		outString += "\n"
	}

	return outString[:len(outString)-1] // Kill final newline
}

func (g *TileGraph) PathString(path []graph.Node) string {
	if path == nil || len(path) == 0 {
		return g.String()
	}

	var outString string
	for r := 0; r < g.numRows; r++ {
		for c := 0; c < g.numCols; c++ {
			if id := r*g.numCols + c; g.tiles[id] == false {
				outString += "\u2580" // Black square
			} else if id == path[0].ID() {
				outString += "s"
			} else if id == path[len(path)-1].ID() {
				outString += "g"
			} else {
				toAppend := " "
				for _, num := range path[1 : len(path)-1] {
					if id == num.ID() {
						toAppend = "♥"
					}
				}
				outString += toAppend
			}
		}

		outString += "\n"
	}

	return outString[:len(outString)-1]
}

func (g *TileGraph) Dimensions() (rows, cols int) {
	return g.numRows, g.numCols
}

func (g *TileGraph) IDToCoords(id int) (row, col int) {
	col = (id % g.numCols)
	row = (id - col) / g.numCols

	return row, col
}

func (g *TileGraph) CoordsToID(row, col int) int {
	if row < 0 || row >= g.numRows || col < 0 || col >= g.numCols {
		return -1
	}

	return row*g.numCols + col
}

func (g *TileGraph) CoordsToNode(row, col int) graph.Node {
	id := g.CoordsToID(row, col)
	if id == -1 {
		return nil
	}
	return Node(id)
}

func (g *TileGraph) Neighbors(n graph.Node) []graph.Node {
	id := n.ID()
	if !g.NodeExists(n) {
		return nil
	}

	row, col := g.IDToCoords(id)

	neighbors := []graph.Node{g.CoordsToNode(row-1, col), g.CoordsToNode(row+1, col), g.CoordsToNode(row, col-1), g.CoordsToNode(row, col+1)}
	realNeighbors := make([]graph.Node, 0, 4) // Will overallocate sometimes, but not by much. Not a big deal
	for _, neigh := range neighbors {
		if neigh != nil && g.tiles[neigh.ID()] == true {
			realNeighbors = append(realNeighbors, neigh)
		}
	}

	return realNeighbors
}

func (g *TileGraph) EdgeBetween(n, neigh graph.Node) graph.Edge {
	if !g.NodeExists(n) || !g.NodeExists(neigh) {
		return nil
	}

	r1, c1 := g.IDToCoords(n.ID())
	r2, c2 := g.IDToCoords(neigh.ID())
	if (c1 == c2 && (r2 == r1+1 || r2 == r1-1)) || (r1 == r2 && (c2 == c1+1 || c2 == c1-1)) {
		return Edge{n, neigh}
	}

	return nil
}

func (g *TileGraph) NodeExists(n graph.Node) bool {
	id := n.ID()
	return id >= 0 && id < len(g.tiles) && g.tiles[id] == true
}

func (g *TileGraph) Degree(n graph.Node) int {
	return len(g.Neighbors(n)) * 2
}

func (g *TileGraph) EdgeList() []graph.Edge {
	edges := make([]graph.Edge, 0)
	for id, passable := range g.tiles {
		if !passable {
			continue
		}

		for _, succ := range g.Neighbors(Node(id)) {
			edges = append(edges, Edge{Node(id), succ})
		}
	}

	return edges
}

func (g *TileGraph) NodeList() []graph.Node {
	nodes := make([]graph.Node, 0)
	for id, passable := range g.tiles {
		if !passable {
			continue
		}

		nodes = append(nodes, Node(id))
	}

	return nodes
}

func (g *TileGraph) Cost(e graph.Edge) float64 {
	if edge := g.EdgeBetween(e.Head(), e.Tail()); edge != nil {
		return 1
	}

	return inf
}
