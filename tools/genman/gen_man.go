package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/openshift/origin/pkg/cmd/openshift"
	"github.com/openshift/origin/pkg/oc/cli"
	mangen "github.com/openshift/origin/tools/genman/md2man"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/cmd/genutils"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Root command not specified (oc | openshift).\n")
		os.Exit(1)
	}

	if strings.HasSuffix(os.Args[2], "oc") {
		genCmdMan("oc", cli.NewCommandCLI("oc", "oc", &bytes.Buffer{}, os.Stdout, ioutil.Discard))
	} else if strings.HasSuffix(os.Args[2], "openshift") {
		genCmdMan("openshift", openshift.NewCommandOpenShift("openshift"))
	} else {
		fmt.Fprintf(os.Stderr, "Root command not specified (oc | openshift).")
		os.Exit(1)
	}
}

func genCmdMan(cmdName string, cmd *cobra.Command) {
	// use os.Args instead of "flags" because "flags" will mess up the man pages!
	path := "docs/man/" + cmdName
	if len(os.Args) == 3 {
		path = os.Args[1]
	} else if len(os.Args) > 3 {
		fmt.Fprintf(os.Stderr, "usage: %s [output directory]\n", os.Args[0])
		os.Exit(1)
	}

	outDir, err := genutils.OutDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get output directory: %v\n", err)
		os.Exit(1)
	}

	// Set environment variables used by openshift so the output is consistent,
	// regardless of where we run.
	os.Setenv("HOME", "/home/username")
	// TODO os.Stdin should really be something like ioutil.Discard, but a Reader
	genMarkdown(cmd, "", outDir)
	for _, c := range cmd.Commands() {
		genMarkdown(c, cmdName, outDir)
	}
}

func preamble(out *bytes.Buffer, cmdName, name, short, long string) {
	out.WriteString(`% ` + strings.ToUpper(cmdName) + `(1) Openshift CLI User Manuals
% Openshift
% June 2016
# NAME
`)

	fmt.Fprintf(out, "%s \\- %s\n\n", name, short)
	fmt.Fprintf(out, "# SYNOPSIS\n")
	fmt.Fprintf(out, "**%s** [OPTIONS]\n\n", name)
	fmt.Fprintf(out, "# DESCRIPTION\n")
	fmt.Fprintf(out, "%s\n\n", long)
}

func printFlags(out *bytes.Buffer, flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		format := "**--%s**=%s\n\t%s\n\n"
		if flag.Value.Type() == "string" {
			// put quotes on the value
			format = "**--%s**=%q\n\t%s\n\n"
		}

		defValue := flag.DefValue
		if flag.Value.Type() == "duration" {
			defValue = "0"
		}

		if len(flag.Annotations["manpage-def-value"]) > 0 {
			defValue = flag.Annotations["manpage-def-value"][0]
		}

		// Todo, when we mark a shorthand is deprecated, but specify an empty message.
		// The flag.ShorthandDeprecated is empty as the shorthand is deprecated.
		// Using len(flag.ShorthandDeprecated) > 0 can't handle this, others are ok.
		if !(len(flag.ShorthandDeprecated) > 0) && len(flag.Shorthand) > 0 {
			format = "**-%s**, " + format
			fmt.Fprintf(out, format, flag.Shorthand, flag.Name, defValue, flag.Usage)
		} else {
			fmt.Fprintf(out, format, flag.Name, defValue, flag.Usage)
		}
	})
}

func printOptions(out *bytes.Buffer, command *cobra.Command) {
	flags := command.NonInheritedFlags()
	if flags.HasFlags() {
		fmt.Fprintf(out, "# OPTIONS\n")
		printFlags(out, flags)
		fmt.Fprintf(out, "\n")
	}
	flags = command.InheritedFlags()
	if flags.HasFlags() {
		fmt.Fprintf(out, "# OPTIONS INHERITED FROM PARENT COMMANDS\n")
		printFlags(out, flags)
		fmt.Fprintf(out, "\n")
	}
}

func genMarkdown(command *cobra.Command, parent, docsDir string) {
	dparent := strings.Replace(parent, " ", "-", -1)
	name := command.Name()
	dname := name
	cmdName := name
	if len(parent) > 0 {
		dname = dparent + "-" + name
		name = parent + " " + name
		cmdName = parent
	}

	out := new(bytes.Buffer)
	short := command.Short
	long := command.Long
	if len(long) == 0 {
		long = short
	}

	preamble(out, cmdName, name, short, long)
	printOptions(out, command)

	if len(command.Example) > 0 {
		fmt.Fprintf(out, "# EXAMPLE\n")
		fmt.Fprintf(out, "```\n%s\n```\n", command.Example)
	}

	if len(command.Commands()) > 0 || len(parent) > 0 {
		fmt.Fprintf(out, "# SEE ALSO\n")
		if len(parent) > 0 {
			fmt.Fprintf(out, "**%s(1)**, ", dparent)
		}
		for _, c := range command.Commands() {
			fmt.Fprintf(out, "**%s-%s(1)**, ", dname, c.Name())
			genMarkdown(c, name, docsDir)
		}
		fmt.Fprintf(out, "\n")
	}

	out.WriteString(`
# HISTORY
June 2016, Ported from the Kubernetes man-doc generator
`)

	final := mangen.Render(out.Bytes())

	filename := docsDir + dname + ".1"
	outFile, err := os.Create(filename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer outFile.Close()
	_, err = outFile.Write(final)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
