package assets

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"
)

type Permission os.FileMode

const (
	PermissionDirectoryDefault Permission = 0755
	PermissionFileDefault      Permission = 0644
	PermissionFileRestricted   Permission = 0600
)

// Asset defines a single static asset.
type Asset struct {
	Name           string
	FilePermission Permission
	Data           []byte
}

// Assets is a list of assets.
type Assets []Asset

// WriteFiles writes the assets to specified path.
func (as Assets) WriteFiles(path string) error {
	if err := os.MkdirAll(path, os.FileMode(PermissionDirectoryDefault)); err != nil {
		return err
	}
	for _, asset := range as {
		if _, err := os.Stat(path); os.IsExist(err) {
			fmt.Printf("WARNING: File %s already exists, content will be replaced\n", path)
		}
		if err := asset.WriteFile(path); err != nil {
			return err
		}
	}
	return nil
}

// WriteFile writes a single asset into specified path.
func (a Asset) WriteFile(path string) error {
	f := filepath.Join(path, a.Name)
	perms := PermissionFileDefault
	if err := os.MkdirAll(filepath.Dir(f), os.FileMode(PermissionDirectoryDefault)); err != nil {
		return err
	}
	if a.FilePermission != 0 {
		perms = a.FilePermission
	}
	fmt.Printf("Writing asset: %s\n", f)
	return ioutil.WriteFile(f, a.Data, os.FileMode(perms))
}

// MustCreateAssetFromTemplate process the given template using and return an asset.
func MustCreateAssetFromTemplate(name string, template []byte, config interface{}) Asset {
	asset, err := assetFromTemplate(name, template, config)
	if err != nil {
		panic(err)
	}
	return asset
}

func assetFromTemplate(name string, tb []byte, data interface{}) (Asset, error) {
	tmpl, err := template.New(name).Parse(string(tb))
	if err != nil {
		return Asset{}, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return Asset{}, err
	}
	return Asset{Name: name, Data: buf.Bytes()}, nil
}
