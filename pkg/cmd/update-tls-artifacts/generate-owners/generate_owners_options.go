package generate_owners

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/certs"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type GenerateOwnersOptions struct {
	TLSInfoDir   string
	Verify       bool
	Requirements []tlsmetadatainterfaces.Requirement

	genericclioptions.IOStreams
}

func (o *GenerateOwnersOptions) Run() error {
	rawData, err := certs.GetRawDataFromDir(filepath.Join(o.TLSInfoDir, "raw-data"))
	if err != nil {
		return fmt.Errorf("failure reading raw data: %w", err)
	}
	errs := []error{}
	for _, requirement := range o.Requirements {
		result, err := requirement.InspectRequirement(rawData)
		if err != nil {
			errs = append(errs, fmt.Errorf("failure inspecting for %v: %w", requirement.GetName(), err))
			continue
		}

		if o.Verify {
			diff, ok, err := result.DiffExistingContent(o.TLSInfoDir)
			switch {
			case err != nil:
				errs = append(errs, fmt.Errorf("failure diffing for %v: %w", requirement.GetName(), err))

			case len(diff) > 0:
				errs = append(errs, fmt.Errorf("diff didn't match for %v\n%v", requirement.GetName(), diff))

			case !ok:
				errs = append(errs, fmt.Errorf("diff didn't match for %v, but no details included", requirement.GetName()))

			}

			continue
		}

		if err := os.MkdirAll(filepath.Join(o.TLSInfoDir, requirement.GetName()), 0755); err != nil {
			errs = append(errs, fmt.Errorf("failure making directory for %v: %w", requirement.GetName(), err))
			continue
		}
		if err := os.MkdirAll(filepath.Join(o.TLSInfoDir, "violations", requirement.GetName()), 0755); err != nil {
			errs = append(errs, fmt.Errorf("failure making directory for %v: %w", requirement.GetName(), err))
			continue
		}
		if err := result.WriteResultToTLSDir(o.TLSInfoDir); err != nil {
			errs = append(errs, fmt.Errorf("failure inspecting for %v: %w", requirement.GetName(), err))
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}
