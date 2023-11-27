package generate_owners

import (
	"os"

	"github.com/openshift/origin/pkg/certs"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/metadata"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type GenerateOwnersOptions struct {
	RawTLSInfoDir       string
	TLSOwnershipInfoDir string
	ViolationDir        string
	Verify              bool

	genericclioptions.IOStreams
}

func (o *GenerateOwnersOptions) Run() error {
	result, err := certs.GetPKIInfoFromRawData(o.RawTLSInfoDir)
	if err != nil {
		return err
	}
	violations, err := metadata.GenerateViolationList(result, metadata.All...)
	if err != nil {
		return err
	}

	if o.Verify {
		if diff := violations.DiffWithExistingJSON(o.ViolationDir); diff != nil {
			return diff
		}

		if diff := violations.DiffWithExistingMarkdown(o.TLSOwnershipInfoDir); diff != nil {
			return diff
		}
	} else {
		// write the json out
		if err := os.MkdirAll(o.TLSOwnershipInfoDir, 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(o.ViolationDir, 0755); err != nil {
			return err
		}

		err = violations.WriteJSONFiles(o.ViolationDir)
		if err != nil {
			return err
		}
		err = violations.WriteMarkdownFiles(o.TLSOwnershipInfoDir)
		if err != nil {
			return err
		}
	}
	return nil
}
