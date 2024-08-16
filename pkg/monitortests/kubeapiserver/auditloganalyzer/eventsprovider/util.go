package eventsprovider

import (
	"bytes"
	"context"

	"github.com/openshift/origin/pkg/monitortestlibrary/nodeaccess"
	"k8s.io/client-go/kubernetes"
)

func getAuditLogFilenames(ctx context.Context, client kubernetes.Interface, nodeName, apiserverName string) ([]string, error) {
	allBytes, err := nodeaccess.GetNodeLogFile(ctx, client, nodeName, apiserverName)
	if err != nil {
		return nil, err
	}

	filenames, err := nodeaccess.GetDirectoryListing(bytes.NewBuffer(allBytes))
	if err != nil {
		return nil, err
	}

	return filenames, nil
}
