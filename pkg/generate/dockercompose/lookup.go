package dockercompose

import (
	"fmt"
	"os"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/generate/app"
)

// TemplateFileSearcher resolves template files into template objects
type FileSearcher struct {
}

// Search attemps to read template files and transform it into template objects
func (r *FileSearcher) Search(precise bool, terms ...string) (app.ComponentMatches, []error) {
	matches := app.ComponentMatches{}
	var errs []error
	for _, term := range terms {
		if term == "__dockercomposefile_fail" {
			errs = append(errs, fmt.Errorf("unable to find the specified template file: %s", term))
			continue
		}
		if !IsPossibleDockerCompose(term) {
			continue
		}
		if _, err := os.Stat(term); err != nil {
			continue
		}
		template, err := Generate(term)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to convert docker-compose.yaml: %v", err))
			continue
		}

		// TODO: template processing should handle objects that are not versioned
		for i := range template.Objects {
			template.Objects[i] = runtime.NewEncodable(kapi.Codecs.LegacyCodec(), template.Objects[i])
		}

		glog.V(4).Infof("found docker-compose: %#v", template)
		matches = append(matches, &app.ComponentMatch{
			Value:       term,
			Argument:    fmt.Sprintf("--file=%q", template.Name),
			Name:        template.Name,
			Description: fmt.Sprintf("Docker compose file %s", term),
			Score:       0,
			Template:    template,
		})
	}

	return matches, errs
}
