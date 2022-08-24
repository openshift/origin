package internal

import (
	"sync"

	"github.com/onsi/ginkgo/v2/internal/interrupt_handler"
	"github.com/onsi/ginkgo/v2/internal/parallel_support"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
)

var (
	buildTreeLock sync.Mutex
)

func (suite *Suite) GetReport() types.Report {
	return suite.report
}
func (suite *Suite) WalkTests(fn func(testName string, test types.TestSpec)) {
	specs := GenerateSpecsFromTreeRoot(suite.tree)
	for _, spec := range specs {
		fn(spec.Text(), spec)
	}
}

func (suite *Suite) ClearBeforeAndAfterSuiteNodes() {
	suite.BuildTree()
	newNodes := Nodes{}
	for _, node := range suite.suiteNodes {
		if node.NodeType == types.NodeTypeBeforeSuite || node.NodeType == types.NodeTypeAfterSuite || node.NodeType == types.NodeTypeSynchronizedBeforeSuite || node.NodeType == types.NodeTypeSynchronizedAfterSuite {
			continue
		}
		newNodes = append(newNodes, node)
	}
	suite.suiteNodes = newNodes
}

func (suite *Suite) GetTree() *TreeNode {
	return suite.tree
}

func (suite *Suite) RunSpec(spec Spec, suiteLabels Labels, suitePath string, failer *Failer, reporter reporters.Reporter, writer WriterInterface, outputInterceptor OutputInterceptor, interruptHandler interrupt_handler.InterruptHandlerInterface, client parallel_support.Client, suiteConfig types.SuiteConfig) (bool, bool) {
	if suite.phase != PhaseBuildTree {
		panic("cannot run before building the tree = call suite.BuildTree() first")
	}

	suite.phase = PhaseRun
	suite.client = client
	suite.failer = failer
	suite.reporter = reporter
	suite.writer = writer
	suite.outputInterceptor = outputInterceptor
	suite.interruptHandler = interruptHandler
	suite.config = suiteConfig

	success := suite.runSpecs("", suiteLabels, suitePath, false, []Spec{spec})

	return success, false
}
