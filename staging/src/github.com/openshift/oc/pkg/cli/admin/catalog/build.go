package catalog

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/oc/pkg/cli/admin/catalog/apprclient"
)

func NewBuildImageOptions(streams genericclioptions.IOStreams) *BuildImageOptions {
	return &BuildImageOptions{
		IOStreams: streams,
		AppRegistryEndpoint: "https://quay.io/cnr",
	}
}

func NewBuildImage(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewBuildImageOptions(streams)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Builds a registry container image from a collection operator manifests.",
		Long: templates.LongDesc(`
			Builds a registry container image from a collection operator manifests.

			Extracts the contents of a collection of operator manifests to disk, and builds them into
			an operator registry image.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()

	flags.StringVarP(&o.Tag, "tag", "t", "", "Tag to apply to built catalog image.")
	flags.StringVar(&o.AuthToken, "auth-token", "", "Auth token for communicating to appregistry.")
	flags.StringVar(&o.AppRegistryEndpoint, "app-registry", o.AppRegistryEndpoint, "Endpoint for pulling from an appregistry instance.")
	flags.StringVarP(&o.AppRegistryNamespace, "namespace", "n", "", "Namespace to pull from an appregistry instance")

	return cmd
}

type BuildImageOptions struct {
	genericclioptions.IOStreams

	Tag                  string
	AuthToken            string
	AppRegistryEndpoint  string
	AppRegistryNamespace string
}

func (o *BuildImageOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {

	if o.Tag == "" {
		return fmt.Errorf("you must specify a tag for the resulting image. example: quay.io/myorg/myimage:1.0.3")
	}
	if o.AppRegistryEndpoint == "" {
		return fmt.Errorf("app-registry must be a valid app-registry endpoint")
	}
	if o.AppRegistryNamespace == "" {
		return fmt.Errorf("namespace must be specified")
	}

	return nil
}

func (o *BuildImageOptions) Run() error {
	opts := apprclient.Options{Source: o.AppRegistryEndpoint}
	if o.AuthToken != "" {
		opts.AuthToken = o.AuthToken
	}
	client, err := apprclient.New(opts)
	if err != nil {
		return fmt.Errorf("couldn't connect to appregistry, %s", err.Error())
	}

	downloader := apprclient.NewDownloader(client)
	dir, err := downloader.DownloadManifestsTmp(o.AppRegistryNamespace)
	if err != nil {
		return err
	}
	fmt.Printf("downloaded to %s\n", dir)

	archivePath, err := BuildManifestLayer(dir)
	if err != nil {
		klog.V(4).Info(err)
		return err
	}

	fmt.Printf("archive: %s\n", archivePath)

	return nil
}

func BuildManifestLayer(directory string) (string, error) {
	archiveDir, err := ioutil.TempDir("", "archive-")
	if err != nil {
		return "", err
	}

	archive, err := os.Create(path.Join(archiveDir, "layer.tar.gz"))
	if err != nil {
		return "", err
	}
	defer func(){
		if err := archive.Close(); err != nil {
			klog.Warningf("error closing file: %s", err.Error())
		}
	}()

	gzipWriter := gzip.NewWriter(archive)
	defer func(){
		if err := gzipWriter.Close(); err != nil {
			klog.Warningf("error closing writer: %s", err.Error())
		}
	}()
	writer := tar.NewWriter(gzipWriter)
	defer func(){
		if err := writer.Close(); err != nil {
			klog.Warningf("error closing writer: %s", err.Error())
		}
	}()

	if err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func(){
			if err := file.Close(); err != nil {
				klog.Warningf("error closing file: %s", err.Error())
			}
		}()

		header := new(tar.Header)
		header.Name = "/manifests"+strings.TrimPrefix(file.Name(), directory)
		header.Size = info.Size()
		header.Mode = int64(info.Mode())
		header.ModTime = info.ModTime()
		err = writer.WriteHeader(header)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, file)
		return err
	}); err != nil {
		return "", err
	}

	return archive.Name(), nil
}
