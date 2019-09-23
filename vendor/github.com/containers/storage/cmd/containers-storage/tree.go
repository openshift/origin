package main

import (
	"fmt"
	"strings"
)

const treeIndentStep = 2
const treeStemWidth = treeIndentStep - 1
const treeVertical = '\u2502'
const treeThisAndMore = "\u251c"
const treeJustThis = "\u2514"
const treeStem = "\u2500"

type treeNode struct {
	left, right string
	notes       []string
}

func selectRoot(nodes []treeNode) string {
	children := make(map[string][]string)
	areChildren := make(map[string]bool)
	for _, node := range nodes {
		areChildren[node.right] = true
		if childlist, ok := children[node.left]; ok {
			children[node.left] = append(childlist, node.right)
		} else {
			children[node.left] = []string{node.right}
		}
	}
	favorite := ""
	for left, right := range children {
		if areChildren[left] {
			continue
		}
		if favorite == "" {
			favorite = left
		} else if len(right) < len(children[favorite]) {
			favorite = left
		}
	}
	return favorite
}

func printSubTree(root string, nodes []treeNode, indent int, continued []int) []treeNode {
	leftovers := []treeNode{}
	children := []treeNode{}
	for _, node := range nodes {
		if node.left != root {
			leftovers = append(leftovers, node)
			continue
		}
		children = append(children, node)
	}
	for n, child := range children {
		istring := []rune(strings.Repeat(" ", indent))
		for _, column := range continued {
			istring[column] = treeVertical
		}
		subc := continued[:]
		header := treeJustThis
		noteHeader := " "
		if n < len(children)-1 {
			subc = append(subc, indent)
			header = treeThisAndMore
			noteHeader = string(treeVertical)
		}
		fmt.Printf("%s%s%s%s\n", string(istring), header, strings.Repeat(treeStem, treeStemWidth), child.right)
		for _, note := range child.notes {
			fmt.Printf("%s%s%s%s\n", string(istring), noteHeader, strings.Repeat(" ", treeStemWidth), note)
		}
		leftovers = printSubTree(child.right, leftovers, indent+treeIndentStep, subc)
	}
	return leftovers
}

func printTree(nodes []treeNode) {
	for len(nodes) > 0 {
		root := selectRoot(nodes)
		fmt.Printf("%s\n", root)
		oldLength := len(nodes)
		nodes = printSubTree(root, nodes, 0, []int{})
		newLength := len(nodes)
		if oldLength == newLength {
			break
		}
	}
}
