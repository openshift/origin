package policy

import (
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const WhatCanIDoRecommendedName = "what-can-i-do"

type whatCanIDoOptions struct {
	namespace string
	client    client.SelfSubjectRulesReviewsNamespacer

	out io.Writer
}

// NewCmdWhatCanIDo implements the OpenShift cli who-can command
func NewCmdWhatCanIDo(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &whatCanIDoOptions{out: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "List what I can do in this namespace",
		Long:  "List what I can do in this namespace",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			kcmdutil.CheckErr(options.run())
		},
	}

	return cmd
}

const (
	tabwriterMinWidth = 10
	tabwriterWidth    = 4
	tabwriterPadding  = 3
	tabwriterPadChar  = ' '
	tabwriterFlags    = 0
)

func (o *whatCanIDoOptions) complete(f *clientcmd.Factory, args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}

	var err error
	o.client, _, err = f.Clients()
	if err != nil {
		return err
	}

	o.namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	return nil
}

func (o *whatCanIDoOptions) run() error {
	whatCanIDo, err := o.client.SelfSubjectRulesReviews(o.namespace).Create(&authorizationapi.SelfSubjectRulesReview{})
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(o.out, tabwriterMinWidth, tabwriterWidth, tabwriterPadding, tabwriterPadChar, tabwriterFlags)
	fmt.Fprint(writer, describe.PolicyRuleHeadings+"\n")
	for _, rule := range whatCanIDo.Status.Rules {
		describe.DescribePolicyRule(writer, rule, "")

	}
	writer.Flush()

	return nil
}
