package apprclient

import (
	"io/ioutil"

	"k8s.io/klog"
)

// NewDownloader is a constructor for the Downloader interface
func NewDownloader(client Client) Downloader {
	return &downloader{
		client:  client,
		decoder: NewManifestDecoder(),
	}
}

// Downloader is an interface that is implemented by structs that
// implement the DownloadManifests method.
type Downloader interface {
	// DownloadManifests downloads the manifests in a namespace into a local directory
	DownloadManifests(directory, namespace string) error
	// DownloadManifests downloads manifests and returns a reference to the tmp directory that contains the manifests
	DownloadManifestsTmp(namespace string) (directory string, err error)
}

type downloader struct {
	client  Client
	decoder ManifestDecoder
}

func (d *downloader) DownloadManifests(directory, namespace string) error {
	klog.V(4).Infof("Downloading manifests at namespace %s to %s", namespace, directory)


	packages, err := d.client.ListPackages(namespace)
	if err != nil {
		return err
	}

	for _, pkg := range packages {
		klog.V(4).Infof("Downloading %s", pkg)
		manifest, err := d.client.RetrieveOne(namespace+"/"+pkg.Name, pkg.Release)
		if err != nil {
			return err
		}

		_, err = d.decoder.Decode([]*OperatorMetadata{manifest}, directory)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *downloader) DownloadManifestsTmp(namespace string) (directory string, err error) {
	directory, err = ioutil.TempDir("", "catalog-")
	if err != nil {
		return "", err
	}
	err = d.DownloadManifests(directory, namespace)
	return directory, err
}
