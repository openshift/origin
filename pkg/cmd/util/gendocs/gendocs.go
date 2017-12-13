package gendocs

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/printers"
)

type Examples []*unstructured.Unstructured

func (x Examples) Len() int      { return len(x) }
func (x Examples) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x Examples) Less(i, j int) bool {
	xi, _ := x[i].Object["fullName"].(string)
	xj, _ := x[j].Object["fullName"].(string)
	return xi < xj
}

func GenDocs(cmd *cobra.Command, filename string) error {
	out := new(bytes.Buffer)
	templateFile, err := filepath.Abs("hack/clibyexample/template")
	if err != nil {
		return err
	}
	template, err := ioutil.ReadFile(templateFile)
	if err != nil {
		return err
	}

	examples := extractExamples(cmd)
	items := []runtime.Object{}
	for _, example := range examples {
		items = append(items, example)
	}

	printOpts := kcmdutil.ExtractCmdPrintOptions(cmd, false)
	printOpts.OutputFormatType = "template"
	printOpts.OutputFormatArgument = string(template)

	printer, err := printers.GetStandardPrinter(
		nil, nil, nil, nil, *printOpts)
	if err != nil {
		return err
	}

	err = printer.PrintObj(&kapi.List{
		ListMeta: metav1.ListMeta{},
		Items:    items,
	}, out)
	if err != nil {
		return err
	}

	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = outFile.Write(out.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func extractExamples(cmd *cobra.Command) Examples {
	objs := Examples{}
	for _, c := range cmd.Commands() {
		if len(c.Deprecated) > 0 {
			continue
		}
		objs = append(objs, extractExamples(c)...)
	}
	if cmd.HasExample() {
		o := &unstructured.Unstructured{
			Object: make(map[string]interface{}),
		}
		o.Object["name"] = cmd.Name()
		o.Object["fullName"] = cmd.CommandPath()
		o.Object["description"] = cmd.Short
		o.Object["examples"] = cmd.Example
		objs = append(objs, o)
	}
	sort.Sort(objs)
	return objs
}
