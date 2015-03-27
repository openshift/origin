//Copyright 2013 Vastech SA (PTY) LTD
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package gographviz

import (
	"fmt"
	"strconv"
)

func ExampleRead() {
	g, err := Read([]byte(`digraph G {Hello->World}`))
	if err != nil {
		panic(err)
	}
	s := g.String()
	fmt.Println(s)
	// Output: digraph G {
	//	Hello->World;
	//	Hello;
	//	World;
	//
	//}
}

func ExampleNewGraph() {
	g := NewGraph()
	g.SetName("G")
	g.SetDir(true)
	g.AddNode("G", "Hello", nil)
	g.AddNode("G", "World", nil)
	g.AddEdge("Hello", "World", true, nil)
	s := g.String()
	fmt.Println(s)
	// Output: digraph G {
	//	Hello->World;
	//	Hello;
	//	World;
	//
	//}
}

type MyOwnGraphStructure struct {
	weights map[int]map[int]int
	max     int
}

func NewMyOwnGraphStructure() *MyOwnGraphStructure {
	return &MyOwnGraphStructure{
		make(map[int]map[int]int),
		0,
	}
}

func (this *MyOwnGraphStructure) SetStrict(strict bool) {}
func (this *MyOwnGraphStructure) SetDir(directed bool)  {}
func (this *MyOwnGraphStructure) SetName(name string)   {}
func (this *MyOwnGraphStructure) AddPortEdge(src, srcPort, dst, dstPort string, directed bool, attrs map[string]string) {
	srci, err := strconv.Atoi(src)
	if err != nil {
		return
	}
	dsti, err := strconv.Atoi(dst)
	if err != nil {
		return
	}
	ai, err := strconv.Atoi(attrs["label"])
	if err != nil {
		return
	}
	if _, ok := this.weights[srci]; !ok {
		this.weights[srci] = make(map[int]int)
	}
	this.weights[srci][dsti] = ai
	if srci > this.max {
		this.max = srci
	}
	if dsti > this.max {
		this.max = dsti
	}

}
func (this *MyOwnGraphStructure) AddEdge(src, dst string, directed bool, attrs map[string]string) {
	this.AddPortEdge(src, "", dst, "", directed, attrs)
}
func (this *MyOwnGraphStructure) AddNode(parentGraph string, name string, attrs map[string]string) {}
func (this *MyOwnGraphStructure) AddAttr(parentGraph string, field, value string)                  {}
func (this *MyOwnGraphStructure) AddSubGraph(parentGraph string, name string, attrs map[string]string) {
}
func (this *MyOwnGraphStructure) String() string { return "" }

//An Example of how to parse into your own simpler graph structure and output it back to graphviz.
//This example reads in only numbers and outputs a matrix graph.
func ExampleMyOwnGraphStructure() {
	name := "matrix"
	parsed, err := Parse([]byte(`
		digraph G {
			1 -> 2 [ label = 5 ];
			4 -> 2 [ label = 1 ];
			4 -> 1 [ label = 2 ];
			1 -> 1 [ label = 0 ];
		}

	`))
	if err != nil {
		panic(err)
	}
	mine := NewMyOwnGraphStructure()
	Analyse(parsed, mine)
	output := NewGraph()
	output.SetName(name)
	output.SetDir(true)
	for i := 1; i <= mine.max; i++ {
		output.AddNode(name, fmt.Sprintf("%v", i), nil)
		if _, ok := mine.weights[i]; !ok {
			mine.weights[i] = make(map[int]int)
		}
	}
	for i := 1; i <= mine.max; i++ {
		for j := 1; j <= mine.max; j++ {
			output.AddEdge(fmt.Sprintf("%v", i), fmt.Sprintf("%v", j), true, map[string]string{"label": fmt.Sprintf("%v", mine.weights[i][j])})
		}
	}
	s := output.String()
	fmt.Println(s)
	// Output: digraph matrix {
	//	1->1[ label=0 ];
	//	1->2[ label=5 ];
	//	1->3[ label=0 ];
	//	1->4[ label=0 ];
	//	2->1[ label=0 ];
	//	2->2[ label=0 ];
	//	2->3[ label=0 ];
	//	2->4[ label=0 ];
	//	3->1[ label=0 ];
	//	3->2[ label=0 ];
	//	3->3[ label=0 ];
	//	3->4[ label=0 ];
	//	4->1[ label=2 ];
	//	4->2[ label=1 ];
	//	4->3[ label=0 ];
	//	4->4[ label=0 ];
	//	1;
	//	2;
	//	3;
	//	4;
	//
	//}
}
