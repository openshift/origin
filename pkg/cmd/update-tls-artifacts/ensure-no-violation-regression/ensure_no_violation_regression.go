package ensure_no_violation_regression

import (
	"embed"
	"fmt"

	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatadefaults"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"

	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type EnsureNoViolationRegressionOptions struct {
	ViolationsFS embed.FS
	Requirements []tlsmetadatainterfaces.Requirement

	genericclioptions.IOStreams
}

func NewEnsureNoViolationRegressionOptions(allViolations embed.FS, streams genericclioptions.IOStreams) *EnsureNoViolationRegressionOptions {
	return &EnsureNoViolationRegressionOptions{
		ViolationsFS: allViolations,
		Requirements: tlsmetadatadefaults.GetDefaultTLSRequirements(),
		IOStreams:    streams,
	}
}

func (o *EnsureNoViolationRegressionOptions) HaveViolationsRegressed(rawData []*certgraphapi.PKIList) ([]string, bool, error) {
	regressions := []string{}
	overallNoRegressions := false
	errs := []error{}
	for _, requirement := range o.Requirements {
		result, err := requirement.InspectRequirement(rawData)
		if err != nil {
			errs = append(errs, fmt.Errorf("failure inspecting for %v: %w", requirement.GetName(), err))
			continue
		}

		descriptions, ok, err := result.HaveViolationsRegressed(o.ViolationsFS)
		regressions = append(regressions, descriptions...)
		if err != nil {
			errs = append(errs, err)
		}
		overallNoRegressions = overallNoRegressions || ok
	}

	return regressions, overallNoRegressions, utilerrors.NewAggregate(errs)
}
