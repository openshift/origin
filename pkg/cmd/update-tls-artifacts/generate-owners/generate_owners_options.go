package generate_owners

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"

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
	rawData, err := o.getRawDataFromDir()
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

func (o *GenerateOwnersOptions) getRawDataFromDir() ([]*certgraphapi.PKIList, error) {
	ret := []*certgraphapi.PKIList{}

	rawDataDir := filepath.Join(o.TLSInfoDir, "raw-data")
	err := filepath.WalkDir(rawDataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failure walking directory %v: %w", rawDataDir, err)
		}
		if d.IsDir() {
			return nil
		}

		filename := filepath.Join(rawDataDir, d.Name())
		currBytes, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failure reading file %v: %w", filename, err)
		}
		currPKI := &certgraphapi.PKIList{}
		err = json.Unmarshal(currBytes, currPKI)
		if err != nil {
			return fmt.Errorf("failure unmarshalling JSON from file %v: %w", filename, err)
		}
		ret = append(ret, currPKI)

		return nil
	})
	if err != nil {
		return nil, err
	}

	// verification that our raw data is consistent
	if _, err := tlsmetadatainterfaces.ProcessByLocation(ret); err != nil {
		return nil, err
	}

	return ret, nil
}
