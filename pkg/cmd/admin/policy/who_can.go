package policy

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const WhoCanRecommendedName = "who-can"

type whoCanOptions struct {
	allNamespaces    bool
	bindingNamespace string
	client           *client.Client

	verb     string
	resource string
}

// NewCmdWhoCan implements the OpenShift cli who-can command
func NewCmdWhoCan(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &whoCanOptions{}

	cmd := &cobra.Command{
		Use:   "who-can VERB RESOURCE",
		Short: "List who can perform the specified action on a resource",
		Long:  "List who can perform the specified action on a resource",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			var err error
			options.client, _, err = f.Clients()
			kcmdutil.CheckErr(err)

			options.bindingNamespace, _, err = f.DefaultNamespace()
			kcmdutil.CheckErr(err)

			err = options.run()
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().BoolVar(&options.allNamespaces, "all-namespaces", options.allNamespaces, "If present, list who can perform the specified action in all namespaces.")

	return cmd
}

func (o *whoCanOptions) complete(args []string) error {
	if len(args) != 2 {
		return errors.New("you must specify two arguments: verb and resource")
	}

	o.verb = args[0]
	o.resource = args[1]
	return nil
}

func (o *whoCanOptions) run() error {
	authorizationAttributes := authorizationapi.AuthorizationAttributes{
		Resource: o.resource,
		Verb:     o.verb,
	}

	resourceAccessReviewResponse := &authorizationapi.ResourceAccessReviewResponse{}
	var err error
	if o.allNamespaces {
		resourceAccessReviewResponse, err = o.client.ResourceAccessReviews().Create(&authorizationapi.ResourceAccessReview{Action: authorizationAttributes})
	} else {
		resourceAccessReviewResponse, err = o.client.LocalResourceAccessReviews(o.bindingNamespace).Create(&authorizationapi.LocalResourceAccessReview{Action: authorizationAttributes})
	}

	if err != nil {
		return err
	}

	if resourceAccessReviewResponse.Namespace == kapi.NamespaceAll {
		fmt.Printf("Namespace: <all>\n")
	} else {
		fmt.Printf("Namespace: %s\n", resourceAccessReviewResponse.Namespace)
	}
	fmt.Printf("Verb:      %s\n", o.verb)
	fmt.Printf("Resource:  %s\n\n", o.resource)
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
