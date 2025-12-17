package cluster

import (
	"fmt"
	"sync"

	g "github.com/onsi/ginkgo/v2"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	defaultThreadCount int = 10
	channelSize        int = 1024
)

var _ = g.Describe("[sig-scalability][Feature:Performance][Serial][Slow] Load cluster", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLIWithoutNamespace("cl")

	g.It("concurrently with templates", g.Label("Size:L"), func() {
		var namespaces []string

		project := ConfigContext.ClusterLoader.Projects
		if project == nil {
			e2e.Failf("Invalid config file.\nFile: %v", project)
		}

		numWorkers := ConfigContext.ClusterLoader.Threads
		if numWorkers == 0 {
			numWorkers = defaultThreadCount
		}

		work := make(chan ProjectMeta, channelSize)
		ns := make(chan string)
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			go clWorker(work, ns, &wg, oc)
		}

		for _, p := range project {
			for j := 0; j < p.Number; j++ {
				wg.Add(1)
				unit := ProjectMeta{j, p}
				work <- unit
				namespace := <-ns
				namespaces = append(namespaces, namespace)
			}
		}

		wg.Wait()
		e2e.Logf("Worker creations completed, closing channels.")
		close(work)
		close(ns)

		if err := postCreateWait(oc, namespaces); err != nil {
			e2e.Failf("Error in postCreateWait: %v", err)
		}
	})
})

func clWorker(in <-chan ProjectMeta, out chan<- string, wg *sync.WaitGroup, oc *exutil.CLI) {
	for {
		p, ok := <-in
		if !ok {
			e2e.Logf("Channel closed")
			return
		}
		var allArgs []string
		if p.NodeSelector != "" {
			allArgs = append(allArgs, "--node-selector")
			allArgs = append(allArgs, p.NodeSelector)
		}
		nsName := fmt.Sprintf("%s%d", p.Basename, p.Counter)
		allArgs = append(allArgs, nsName)

		// Check to see if the project exists
		projectExists, _ := ProjectExists(oc, nsName)
		if !projectExists {
			e2e.Logf("Project %s does not exist.", nsName)
		}

		// Based on configuration handle project existance
		switch p.IfExists {
		case IF_EXISTS_REUSE:
			e2e.Logf("Configuration requested reuse of project %v", nsName)
		case IF_EXISTS_DELETE:
			e2e.Logf("Configuration requested deletion of project %v", nsName)
			if projectExists {
				err := DeleteProject(oc, nsName, checkDeleteProjectInterval, checkDeleteProjectTimeout)
				e2e.Logf("Error deleting project: %v", err)
			}
		default:
			e2e.Failf("Unsupported ifexists value '%v' for project %v", p.IfExists, p)
		}

		if p.IfExists == IF_EXISTS_REUSE && projectExists {
			// do nothing
		} else {
			// Create namespaces as defined in Cluster Loader config
			err := oc.Run("adm", "new-project").Args(allArgs...).Execute()
			if err != nil {
				e2e.Logf("Error creating project: %v", err)

			} else {
				out <- nsName
				e2e.Logf("Created new namespace: %v", nsName)
			}
		}

		// Create templates as defined
		for _, template := range p.Templates {
			err := CreateSimpleTemplates(oc, nsName, template)
			if err != nil {
				e2e.Logf("Error creating template: %v", err)
			}
		}

		wg.Done()
	}
}
