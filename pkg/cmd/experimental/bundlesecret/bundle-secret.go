package bundlesecret

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/spf13/cobra"
)

func NewCmdBundleSecret(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	var sourceList util.StringList
	cmd := &cobra.Command{
		Use:   "bundle-secret NAME -f SOURCENAME",
		Short: "bundle data from a source list to load kubernetes secret",
		Long:  "bundle data from a source list to load kubernetes secret",
		Run: func(cmd *cobra.Command, args []string) {

			if len(args) == 0 {
				glog.Fatalf("Must specify NAME as an argument")
			}
			objName := args[0]

			if len(sourceList) == 0 {
				glog.Fatalf("Must specify files and/or directories to gather data from.")
			}

			secretData := make(map[string][]byte)

			for _, sourceItem := range sourceList {
				sourceItem = strings.TrimSuffix(sourceItem, "/")
				info, err := os.Stat(sourceItem)
				checkErr(err)

				if info.IsDir() {
					fileList, err := ioutil.ReadDir(sourceItem)
					checkErr(err)

					for _, item := range fileList {
						if !item.IsDir() {
							file := fmt.Sprint(sourceItem, "/", item.Name())
							err := readFile(file, secretData)
							checkErr(err)
						}
					}
				} else {
					err := readFile(sourceItem, secretData)
					checkErr(err)
				}
			}

			secretObj := kapi.Secret{
				ObjectMeta: kapi.ObjectMeta{Name: objName},
				Data:       secretData,
			}

			setDefaultPrinter(cmd)
			f.Factory.PrintObject(cmd, &secretObj, out)
		},
	}

	cmd.Flags().VarP(&sourceList, "source", "f", "List of filenames, directories to use as source of Kubernetes Secret.Data")
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func checkErr(err error) {
	if err != nil {
		glog.Fatal(err)
	}
}

func setDefaultPrinter(c *cobra.Command) {
	flag := c.Flags().Lookup("output")
	if len(flag.Value.String()) == 0 {
		flag.Value.Set("json")
	}
}

func readFile(filePath string, dataMap map[string][]byte) error {
	var err error
	fileName := path.Base(filePath)
	if !util.IsDNS1123Subdomain(fileName) {
		err := fmt.Errorf("%v is not a valid DNS Subdomain.\n", filePath)
		return err
	}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	dataMap[fileName] = []byte(data)
	return nil
}
