package main

import "testing"

func TestTree(*testing.T) {
	nodes := []treeNode{
		{"F", "H", []string{}},
		{"F", "I", []string{}},
		{"F", "J", []string{}},
		{"A", "B", []string{}},
		{"A", "C", []string{}},
		{"A", "K", []string{}},
		{"C", "F", []string{}},
		{"C", "G", []string{"beware", "the", "scary", "thing"}},
		{"C", "L", []string{}},
		{"B", "D", []string{}},
		{"B", "E", []string{}},
		{"B", "M", []string{}},
		{"K", "N", []string{}},
		{"W", "X", []string{}},
		{"Y", "Z", []string{}},
		{"X", "Y", []string{}},
	}
	printTree(nodes)
}
