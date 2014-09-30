package base

import (
	"fmt"
	"os"
	"reflect"

	p "github.com/openshift/origin/pkg/cmd/util/printer"
	"github.com/spf13/cobra"
)

// Base commands

func CreateCmdRoot(resource string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   resource,
		Short: fmt.Sprintf("Command '%s' (main)", resource),
		Long:  fmt.Sprintf("Command '%s' (main)", resource),
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}
	return cmd
}

func CreateCmdList(resource string, name string, listFunc func() (interface{}, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("List one or more %s", resource),
		Long:  fmt.Sprintf("List one or more %s.", resource),
		Run: func(cmd *cobra.Command, args []string) {
			format := getFlagAsString(cmd, "format")
			items, err := listFunc()
			print(format, items, err)
		},
	}
	cmd.Flags().StringP("format", "f", "terminal", "Output format: terminal|raw|json|yaml")
	return cmd
}

func CreateCmdShow(resource string, name string, showFunc func(id string) (interface{}, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " <id>",
		Short: fmt.Sprintf("Display information about a %s", resource),
		Long:  fmt.Sprintf("Display information about a %s.", resource),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				usageError(cmd, "Need to supply an ID")
			}
			id := args[0]
			format := getFlagAsString(cmd, "format")
			item, err := showFunc(id)
			print(format, item, err)
		},
	}
	cmd.Flags().StringP("format", "f", "terminal", "Output format: terminal|raw|json|yaml")
	return cmd
}

func CreateCmdCreate(resource string, name string, createFunc func(payload interface{}) (string, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Create a %s", resource),
		Long:  fmt.Sprintf("Create a %s.", resource),
		Run: func(cmd *cobra.Command, args []string) {
			id, _ := createFunc(nil) // TODO
			fmt.Printf("Created %s %s", resource, id)
		},
	}
	return cmd
}

func CreateCmdUpdate(resource string, name string, updateFunc func(id string, payload interface{}) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Update a %s", resource),
		Long:  fmt.Sprintf("Update a %s.", resource),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				usageError(cmd, "Need to supply an ID")
			}
			id := args[0]
			_ = updateFunc(id, nil) // TODO
			fmt.Printf("Updated %s %s", resource, id)
		},
	}
	return cmd
}

func CreateCmdRemove(resource string, name string, removeFunc func(id string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name + " <id>",
		Short: fmt.Sprintf("Remove a %s", resource),
		Long:  fmt.Sprintf("Remove a %s.", resource),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				usageError(cmd, "Need to supply an ID")
			}
			id := args[0]
			_ = removeFunc(id)
			fmt.Printf("Removed %s %s", resource, id)
		},
	}
	return cmd
}

func usageError(cmd *cobra.Command, format string, args ...interface{}) {
	cmd.Help()
	os.Exit(1)
}

func print(format string, payload interface{}, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	printer := p.TerminalPrinter{} // TODO: use printer according to the provided format

	if reflect.TypeOf(payload).Kind() == reflect.Slice {
		items := reflect.ValueOf(payload)

		if items.Len() == 0 {
			printer.Errorln("Nothing found")

		} else {
			for i := 0; i < items.Len(); i++ {
				fmt.Println(items.Index(i))
			}
		}

	} else {
		fmt.Println(payload)
	}
}

func getFlagAsString(cmd *cobra.Command, flag string) string {
	return cmd.Flags().Lookup(flag).Value.String()
}
