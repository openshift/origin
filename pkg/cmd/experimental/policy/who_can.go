package policy

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type whoCanOptions struct {
	bindingNamespace string
	client           client.Interface

	verb     string
	resource string
}

func NewCmdWhoCan(f *clientcmd.Factory) *cobra.Command {
	options := &whoCanOptions{}

	cmd := &cobra.Command{
		Use:   "who-can",
		Short: "who-can <verb> <resource>",
		Long:  `who-can <verb> <resource>`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			var err error
			if options.client, _, err = f.Clients(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if options.bindingNamespace, err = f.DefaultNamespace(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if err := options.run(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	return cmd
}

func (o *whoCanOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) != 2 {
		cmd.Help()
		return false
	}

	o.verb = args[0]
	o.resource = args[1]
	return true
}

func (o *whoCanOptions) run() error {
	resourceAccessReview := &authorizationapi.ResourceAccessReview{}
	resourceAccessReview.Resource = o.resource
	resourceAccessReview.Verb = o.verb

	resourceAccessReviewResponse, err := o.client.ResourceAccessReviews(o.bindingNamespace).Create(resourceAccessReview)
	if err != nil {
		return err
	}

	if len(resourceAccessReviewResponse.Users) == 0 {
		fmt.Printf("Users:  none\n\n")
	} else {
		fmt.Printf("Users:  %s\n\n", strings.Join(resourceAccessReviewResponse.Users.List(), "\n        "))
	}

	if len(resourceAccessReviewResponse.Groups) == 0 {
		fmt.Printf("Groups: none\n\n")
	} else {
		fmt.Printf("Groups: %s\n\n", strings.Join(resourceAccessReviewResponse.Groups.List(), "\n        "))
	}

	return nil
}
