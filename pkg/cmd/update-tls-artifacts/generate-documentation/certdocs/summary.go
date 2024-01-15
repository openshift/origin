package certdocs

import (
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetMarkdownForSummary(pkiLists []*certgraphapi.PKIList) string {
	nameToMarkdown := map[string]string{}

	ret := "# Certificates in this OpenShift Cluster\n\n" // TODO find cluster ID

	for _, pkiList := range pkiLists {
		currDoc := ""
		currDoc += fmt.Sprintf("## [%s](%s)\n", pkiList.LogicalName, strings.ReplaceAll(pkiList.LogicalName, " ", "%20")+"/README.md")
		currDoc += pkiList.Description + "\n\n"
		nameToMarkdown[pkiList.LogicalName] = currDoc
	}

	for _, name := range sets.StringKeySet(nameToMarkdown).List() {
		ret += nameToMarkdown[name] + "\n\n"
	}

	return ret
}
