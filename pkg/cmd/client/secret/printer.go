package secret

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubecfg"
	"github.com/openshift/origin/pkg/secret/api"
)

var secretColumns = []string{"Name", "Type", "Size"}

// RegisterPrintHandlers registers HumanReadablePrinter handlers
// for secret and secretConfig resources.
func RegisterPrintHandlers(printer *kubecfg.HumanReadablePrinter) {
	printer.Handler(secretColumns, printSecret)
	printer.Handler(secretColumns, printSecretList)
}

func printSecret(secret *api.Secret, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%d\n", secret.Name, secret.Type, len(secret.Data))
	return err
}

func printSecretList(secretList *api.SecretList, w io.Writer) error {
	for _, secret := range secretList.Items {
		if err := printSecret(&secret, w); err != nil {
			return err
		}
	}
	return nil
}
