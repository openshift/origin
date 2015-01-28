package policy

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
)

type whoCanOptions struct {
	clientConfig clientcmd.ClientConfig

	namespace string
	verb      string
	kind      string
}

func NewCmdWhoCan(clientConfig clientcmd.ClientConfig) *cobra.Command {
	options := &whoCanOptions{clientConfig: clientConfig}

	cmd := &cobra.Command{
		Use:   "who-can",
		Short: "who-can <verb> <resourceKind>",
		Long:  `who-can <verb> <resourceKind>`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			err := options.run()
			if err != nil {
				fmt.Printf("%v\n", err)
			}
		},
	}

	cmd.Flags().StringVar(&options.namespace, "namespace", "", "namespace to check.  This should be replaced by clientcmd.ClientConfig")

	return cmd
}

func (o *whoCanOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) != 2 {
		cmd.Help()
		return false
	}

	o.verb = args[0]
	o.kind = args[1]
	return true
}

func (o *whoCanOptions) run() error {
	clientConfig, err := o.clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	client, err := client.New(clientConfig)
	if err != nil {
		return err
	}

	resourceAccessReview := &authorizationapi.ResourceAccessReview{}
	resourceAccessReview.Spec.ResourceKind = o.kind
	resourceAccessReview.Spec.Verb = o.verb

	resourceAccessReviewResponse, err := client.ResourceAccessReviews(o.namespace).Create(resourceAccessReview)
	if err != nil {
		return err
	}

	fmt.Printf("Users: %v\n", resourceAccessReviewResponse.Status.Users)
	fmt.Printf("Groups: %v\n", resourceAccessReviewResponse.Status.Groups)

	return nil
}
