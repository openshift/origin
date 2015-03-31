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

package parser

import (
	"fmt"
	"github.com/awalterschulze/gographviz/ast"
	"io/ioutil"
	"testing"
)

func check(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func assert(t *testing.T, msg string, v1 interface{}, v2 interface{}) {
	if v1 != v2 {
		t.Fatalf("%v\n%v\n!=\n%v", msg, v1, v2)
	}
}

func parseTest(t *testing.T, filename string) {
	localFilename := "../testdata/" + filename
	s, err := ioutil.ReadFile(localFilename)
	check(t, err)
	t.Logf("Input String = %v", string(s))
	g, err := ParseFile(localFilename)
	t.Logf("First Parse = %v", g)
	check(t, err)
	gstr := g.String()
	g2, err := ParseString(gstr)
	t.Logf("Second Parse = %v", g2)
	check(t, err)
	gstr2 := g2.String()
	assert(t, "output strings", gstr, gstr2)
}

func parseStringTest(t *testing.T, s string) {
	t.Logf("Input String = %v", s)
	g, err := ParseString(s)
	t.Logf("First Parse = %v", g)
	check(t, err)
	gstr := g.String()
	g2, err := ParseString(gstr)
	t.Logf("Second Parse = %v", g2)
	check(t, err)
	gstr2 := g2.String()
	assert(t, "output strings", gstr, gstr2)
}

func TestHelloWorldString(t *testing.T) {
	input := `digraph G {Hello->World}`
	g, err := ParseString(input)
	check(t, err)
	fmt.Printf("%#v", g)
}

func TestHelloWorldFile(t *testing.T) {
	g, err := ParseFile("../testdata/helloworld.gv.txt")
	check(t, err)
	fmt.Printf("%#v", g)
}

func TestAttr(t *testing.T) {
	parseStringTest(t,
		"digraph finite_state { rankdir = LR }")
}

func TestString(t *testing.T) {
	parseStringTest(t,
		`digraph finite_state { rankdir = "LR" }`)
}

func TestAttrList(t *testing.T) {
	parseStringTest(t, `
digraph { node [ shape = doublecircle ] }`)
}

func TestStringLit(t *testing.T) {
	parseStringTest(t, `digraph finite_state_machine {
	size= "8" ; }`)
}

func TestHashComments(t *testing.T) {
	parseStringTest(t, `## bla \n
  digraph G {Hello->World}`)
}

func TestIntLit(t *testing.T) {
	parseStringTest(t, `graph G {
	1 -- 30 [f=1];}`)
}

func TestFloat1(t *testing.T) {
	parseStringTest(t, `digraph { bla = 2.0 }`)
}

func TestFloat2(t *testing.T) {
	parseStringTest(t, `digraph { bla = .1 }`)
}

func TestNegative(t *testing.T) {
	parseStringTest(t, `digraph { -2 -> -1 }`)
}

func TestUnderscore(t *testing.T) {
	parseStringTest(t, `digraph { a_b = 1 }`)
}

func TestNonAscii(t *testing.T) {
	parseStringTest(t, `digraph {	label=T�th }`)
}

func TestPorts(t *testing.T) {
	parseStringTest(t, `digraph { "node6":f0 -> "node9":f1 }`)
}

func TestHtml(t *testing.T) {
	parseStringTest(t, `digraph { a = <<table></table>> }`)
}

func TestIdWithKeyword(t *testing.T) {
	parseStringTest(t, `digraph { edgeURL = "a" }`)
}

func TestSubGraph(t *testing.T) {
	parseStringTest(t, `digraph { subgraph { a -> b } }`)
}

func TestImplicitSubGraph(t *testing.T) {
	parseStringTest(t, `digraph { { a -> b } }`)
}

func TestEdges(t *testing.T) {
	parseStringTest(t, `digraph { a0 -> a1 -> a2 -> a3 }`)
}

func TestNodes(t *testing.T) {
	parseStringTest(t, `digraph { a0 a1 }`)
}

func TestTwoAttributes(t *testing.T) {
	g, err := ParseString(`digraph { a0 [shape = circle bla = bla]}`)
	check(t, err)
	t.Logf("Parsed String = %v", g)
	for _, stmt := range g.StmtList {
		node := stmt.(*ast.NodeStmt)
		if len(node.Attrs[0]) != 2 {
			t.Fatalf("Not enough attributes, expected two, but found %v in %v", len(node.Attrs), node)
		}
	}
}

func TestEasyFsm(t *testing.T) {
	parseStringTest(t, `digraph finite_state_machine {
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
	parseStringTest(t, `digraph g { edge [ ] }`)
}

func TestHelloWorld(t *testing.T) {
	parseTest(t, "helloworld.gv.txt")
}

func TestCluster(t *testing.T) {
	parseTest(t, "cluster.gv.txt")
}

func TestPsg(t *testing.T) {
	parseTest(t, "psg.gv.txt")
}

func TestTransparency(t *testing.T) {
	parseTest(t, "transparency.gv.txt")
}

func TestCrazy(t *testing.T) {
	parseTest(t, "crazy.gv.txt")
}

func TestKennedyanc(t *testing.T) {
	parseTest(t, "kennedyanc.gv.txt")
}

func TestRoot(t *testing.T) {
	parseTest(t, "root.gv.txt")
}

func TestTwpoi(t *testing.T) {
	parseTest(t, "twopi.gv.txt")
}

func TestDataStruct(t *testing.T) {
	parseTest(t, "datastruct.gv.txt")
}

func TestLionShare(t *testing.T) {
	parseTest(t, "lion_share.gv.txt")
}

func TestSdh(t *testing.T) {
	parseTest(t, "sdh.gv.txt")
}

func TestUnix(t *testing.T) {
	parseTest(t, "unix.gv.txt")
}

func TestEr(t *testing.T) {
	parseTest(t, "er.gv.txt")
}

func TestNerworkMapTwopi(t *testing.T) {
	parseTest(t, "networkmap_twopi.gv.txt")
}

func TestSibling(t *testing.T) {
	parseTest(t, "siblings.gv.txt")
}

func TestWorld(t *testing.T) {
	parseTest(t, "world.gv.txt")
}

func TestFdpclust(t *testing.T) {
	parseTest(t, "fdpclust.gv.txt")
}

func TestPhilo(t *testing.T) {
	parseTest(t, "philo.gv.txt")
}

func TestSoftmaint(t *testing.T) {
	parseTest(t, "softmaint.gv.txt")
}

func TestFsm(t *testing.T) {
	parseTest(t, "fsm.gv.txt")
}

func TestProcess(t *testing.T) {
	parseTest(t, "process.gv.txt")
}

func TestSwitchGv(t *testing.T) {
	parseTest(t, "switch.gv.txt")
}

func TestGd19942007(t *testing.T) {
	parseTest(t, "gd_1994_2007.gv.txt")
}

func TestProfile(t *testing.T) {
	parseTest(t, "profile.gv.txt")
}

func TestTrafficLights(t *testing.T) {
	parseTest(t, "traffic_lights.gv.txt")
}
