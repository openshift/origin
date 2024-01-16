package generate_documentation

import (
	"context"
	"fmt"
	"path/filepath"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/origin/pkg/certs"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-documentation/certdocs"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type GenerateDocumentationOptions struct {
	TLSInfoDir string
	Verify     bool

	genericclioptions.IOStreams
}

func (o *GenerateDocumentationOptions) Run(ctx context.Context) error {
	rawData, err := certs.GetRawDataFromDir(filepath.Join(o.TLSInfoDir, "raw-data"))
	if err != nil {
		return fmt.Errorf("failure reading raw data: %w", err)
	}

	merged, errs := certs.MergeRawPKILists(ctx, rawData...)
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(o.ErrOut, "failed to merge raw PKI data: %v\n", err)
		}
		return utilerrors.NewAggregate(errs)
	}

	if err := certdocs.WriteDocs(merged, filepath.Join(o.TLSInfoDir, "human-summary")); err != nil {
		return err
	}

	return nil
}
