package assets

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAsset_WriteFile(t *testing.T) {
	sampleAssets := Assets{
		{
			Name: "test-default",
			Data: []byte("test"),
		},
		{
			Name:           "test-restricted",
			FilePermission: PermissionFileRestricted,
			Data:           []byte("test"),
		},
		{
			Name:           "test-default-explicit",
			FilePermission: PermissionFileDefault,
			Data:           []byte("test"),
		},
	}

	assetDir, err := ioutil.TempDir("", "asset-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(assetDir)

	if err := sampleAssets.WriteFiles(assetDir); err != nil {
		t.Fatalf("unexpected error when writing files: %v", err)
	}

	if s, err := os.Stat(filepath.Join(assetDir, sampleAssets[0].Name)); err != nil {
		t.Fatalf("expected file to exists, got: %v", err)
	} else {
		if s.Mode() != os.FileMode(PermissionFileDefault) {
			t.Errorf("expected file to have %d permissions, got %d", PermissionFileDefault, s.Mode())
		}
	}

	if s, err := os.Stat(filepath.Join(assetDir, sampleAssets[1].Name)); err != nil {
		t.Fatalf("expected file to exists, got: %v", err)
	} else {
		if s.Mode() != os.FileMode(sampleAssets[1].FilePermission) {
			t.Errorf("expected file to have %d permissions, got %d", sampleAssets[1].FilePermission, s.Mode())
		}
	}

	if s, err := os.Stat(filepath.Join(assetDir, sampleAssets[2].Name)); err != nil {
		t.Fatalf("expected file to exists, got: %v", err)
	} else {
		if s.Mode() != os.FileMode(sampleAssets[2].FilePermission) {
			t.Errorf("expected file to have %s permissions, got %s", os.FileMode(sampleAssets[2].FilePermission), s.Mode())
		}
	}
}
