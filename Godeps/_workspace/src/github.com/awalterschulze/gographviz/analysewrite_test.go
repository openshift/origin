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
	"github.com/awalterschulze/gographviz/parser"
	"io/ioutil"
	"os"
	"testing"
)

func (this *Nodes) String() string {
	s := "Nodes:"
	for i := range this.Nodes {
		s += fmt.Sprintf("Node{%v}", this.Nodes[i])
	}
	return s + "\n"
}

func (this *Edges) String() string {
	s := "Edges:"
	for i := range this.Edges {
		s += fmt.Sprintf("Edge{%v}", this.Edges[i])
	}
	return s + "\n"
}

func check(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func assert(t *testing.T, msg string, v1 interface{}, v2 interface{}) {
	if v1 != v2 {
		t.Fatalf("%v %v != %v", msg, v1, v2)
	}
}

func anal(t *testing.T, input string) Interface {
	fmt.Printf("Input: %v\n", input)
	g, err := parser.ParseString(input)
	check(t, err)
	fmt.Printf("Parsed: %v\n", g)
	ag := NewGraph()
	Analyse(g, ag)
	fmt.Printf("Analysed: %v\n", ag)
	agstr := ag.String()
	fmt.Printf("Written: %v\n", agstr)
	g2, err := parser.ParseString(agstr)
	check(t, err)
	fmt.Printf("Parsed %v\n", g2)
	ag2 := NewEscape()
	Analyse(g2, ag2)
	fmt.Printf("Analysed %v\n", ag2)
	ag2str := ag2.String()
	fmt.Printf("Written: %v\n", ag2str)
	assert(t, "analysed", agstr, ag2str)
	return ag2
}

func analfile(t *testing.T, filename string) Interface {
	f, err := os.Open(filename)
	check(t, err)
	all, err := ioutil.ReadAll(f)
	check(t, err)
	return anal(t, string(all))
}

func analtest(t *testing.T, testname string) Interface {
	return analfile(t, "./testdata/"+testname)
}

func TestHelloWorldString(t *testing.T) {
	input := `digraph G {Hello->World}`
	anal(t, input)
}

func TestHelloWorldFile(t *testing.T) {
	analfile(t, "./testdata/helloworld.gv.txt")
}

func TestAttr(t *testing.T) {
	anal(t,
		"digraph finite_state { rankdir = LR }")
}

func TestString(t *testing.T) {
	anal(t,
		`digraph finite_state { rankdir = "LR" }`)
}

func TestAttrList(t *testing.T) {
	anal(t, `
digraph { node [ shape = doublecircle ] }`)
}

func TestStringLit(t *testing.T) {
	anal(t, `digraph finite_state_machine {
	size= "8" ; }`)
}

func TestHashComments(t *testing.T) {
	anal(t, `## bla \n
  digraph G {Hello->World}`)
}

func TestIntLit(t *testing.T) {
	anal(t, `graph G {
	1 -- 30 [f=1];}`)
}

func TestFloat1(t *testing.T) {
	anal(t, `digraph { bla = 2.0 }`)
}

func TestFloat2(t *testing.T) {
	anal(t, `digraph { bla = .1 }`)
}

func TestNegative(t *testing.T) {
	anal(t, `digraph { -2 -> -1 }`)
}

func TestUnderscore(t *testing.T) {
	anal(t, `digraph { a_b = 1 }`)
}

func TestNonAscii(t *testing.T) {
	anal(t, `digraph {	label=T�th }`)
}

func TestPorts(t *testing.T) {
	anal(t, `digraph { "node6":f0 -> "node9":f1 }`)
}

func TestHtml(t *testing.T) {
	anal(t, `digraph { a = <<table></table>> }`)
}

func TestIdWithKeyword(t *testing.T) {
	anal(t, `digraph { edgeURL = "a" }`)
}

func TestSubGraph(t *testing.T) {
	anal(t, `digraph { subgraph { a -> b } }`)
}

func TestImplicitSubGraph(t *testing.T) {
	anal(t, `digraph { { a -> b } }`)
}

func TestEdges(t *testing.T) {
	anal(t, `digraph { a0 -> a1 -> a2 -> a3 }`)
}

func TestEasyFsm1(t *testing.T) {
	anal(t, `digraph finite_state_machine {
	rankdir=LR;
	size="8,5";
	node [shape = circle];
	LR_0 -> LR_2 [ label = "SS(B)" ];
	LR_0 -> LR_1 [ label = "SS(S)" ];
	LR_1 -> LR_3 [ label = "S($end)" ];
	LR_2 -> LR_6 [ label = "SS(b)" ];
	LR_2 -> LR_5 [ label = "SS(a)" ];
	LR_2 -> LR_4 [ label = "S(A)" ];
	LR_5 -> LR_7 [ label = "S(b)" ];
	LR_5 -> LR_5 [ label = "S(a)" ];
	LR_6 -> LR_6 [ label = "S(b)" ];
	LR_6 -> LR_5 [ label = "S(a)" ];
	LR_7 -> LR_8 [ label = "S(b)" ];
	LR_7 -> LR_5 [ label = "S(a)" ];
	LR_8 -> LR_6 [ label = "S(b)" ];
	LR_8 -> LR_5 [ label = "S(a)" ];
}`)
}

//node [shape = doublecircle]; LR_0 LR_3 LR_4 LR_8; should be applied to the nodes
func TestEasyFsm2(t *testing.T) {
	anal(t, `digraph finite_state_machine {
	rankdir=LR;
	size="8,5";
	node [shape = doublecircle]; LR_0 LR_3 LR_4 LR_8;
	node [shape = circle];
	LR_0 -> LR_2 [ label = "SS(B)" ];
	LR_0 -> LR_1 [ label = "SS(S)" ];
	LR_1 -> LR_3 [ label = "S($end)" ];
	LR_2 -> LR_6 [ label = "SS(b)" ];
	LR_2 -> LR_5 [ label = "SS(a)" ];
	LR_2 -> LR_4 [ label = "S(A)" ];
	LR_5 -> LR_7 [ label = "S(b)" ];
	LR_5 -> LR_5 [ label = "S(a)" ];
	LR_6 -> LR_6 [ label = "S(b)" ];
	LR_6 -> LR_5 [ label = "S(a)" ];
	LR_7 -> LR_8 [ label = "S(b)" ];
	LR_7 -> LR_5 [ label = "S(a)" ];
	LR_8 -> LR_6 [ label = "S(b)" ];
	LR_8 -> LR_5 [ label = "S(a)" ];
}`)
}

func TestEmptyAttrList(t *testing.T) {
	anal(t, `digraph g { edge [ ] }`)
}

func TestHelloWorld(t *testing.T) {
	analtest(t, "helloworld.gv.txt")
}

func TestCluster(t *testing.T) {
	analtest(t, "cluster.gv.txt")
}

func TestPsg(t *testing.T) {
	analtest(t, "psg.gv.txt")
}

func TestTransparency(t *testing.T) {
	analtest(t, "transparency.gv.txt")
}

func TestCrazy(t *testing.T) {
	analtest(t, "crazy.gv.txt")
}

func TestKennedyanc(t *testing.T) {
	analtest(t, "kennedyanc.gv.txt")
}

func TestRoot(t *testing.T) {
	analtest(t, "root.gv.txt")
}

func TestTwpoi(t *testing.T) {
	analtest(t, "twopi.gv.txt")
}

func TestDataStruct(t *testing.T) {
	analtest(t, "datastruct.gv.txt")
}

func TestLionShare(t *testing.T) {
	analtest(t, "lion_share.gv.txt")
}

func TestSdh(t *testing.T) {
	analtest(t, "sdh.gv.txt")
}

func TestUnix(t *testing.T) {
	analtest(t, "unix.gv.txt")
}

func TestEr(t *testing.T) {
	analtest(t, "er.gv.txt")
}

func TestNerworkMapTwopi(t *testing.T) {
	analtest(t, "networkmap_twopi.gv.txt")
}

func TestSibling(t *testing.T) {
	analtest(t, "siblings.gv.txt")
}

func TestWorld(t *testing.T) {
	analtest(t, "world.gv.txt")
}

func TestFdpclust(t *testing.T) {
	analtest(t, "fdpclust.gv.txt")
}

func TestPhilo(t *testing.T) {
	analtest(t, "philo.gv.txt")
}

func TestSoftmaint(t *testing.T) {
	analtest(t, "softmaint.gv.txt")
}

func TestFsm(t *testing.T) {
	analtest(t, "fsm.gv.txt")
}

func TestProcess(t *testing.T) {
	analtest(t, "process.gv.txt")
}

func TestSwitchGv(t *testing.T) {
	analtest(t, "switch.gv.txt")
}

func TestGd19942007(t *testing.T) {
	analtest(t, "gd_1994_2007.gv.txt")
}

func TestProfile(t *testing.T) {
	analtest(t, "profile.gv.txt")
}

func TestTrafficLights(t *testing.T) {
	analtest(t, "traffic_lights.gv.txt")
}
