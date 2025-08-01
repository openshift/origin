// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package releasesjson

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/hc-install/internal/httpclient"
)

type Downloader struct {
	Logger           *log.Logger
	VerifyChecksum   bool
	ArmoredPublicKey string
	BaseURL          string
}

type UnpackedProduct struct {
	PathsToRemove []string
}

func (d *Downloader) DownloadAndUnpack(ctx context.Context, pv *ProductVersion, binDir string, licenseDir string) (up *UnpackedProduct, err error) {
	if len(pv.Builds) == 0 {
		return nil, fmt.Errorf("no builds found for %s %s", pv.Name, pv.Version)
	}

	pb, ok := pv.Builds.FilterBuild(runtime.GOOS, runtime.GOARCH, "zip")
	if !ok {
		return nil, fmt.Errorf("no ZIP archive found for %s %s %s/%s",
			pv.Name, pv.Version, runtime.GOOS, runtime.GOARCH)
	}

	var verifiedChecksum HashSum
	if d.VerifyChecksum {
		v := &ChecksumDownloader{
			BaseURL:          d.BaseURL,
			ProductVersion:   pv,
			Logger:           d.Logger,
			ArmoredPublicKey: d.ArmoredPublicKey,
		}
		verifiedChecksums, err := v.DownloadAndVerifyChecksums(ctx)
		if err != nil {
			return nil, err
		}
		var ok bool
		verifiedChecksum, ok = verifiedChecksums[pb.Filename]
		if !ok {
			return nil, fmt.Errorf("no checksum found for %q", pb.Filename)
		}
	}

	client := httpclient.NewHTTPClient(d.Logger)

	archiveURL, err := determineArchiveURL(pb.URL, d.BaseURL)
	if err != nil {
		return nil, err
	}

	d.Logger.Printf("downloading archive from %s", archiveURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %q: %w", archiveURL, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download ZIP archive from %q: %s", archiveURL, resp.Status)
	}

	defer resp.Body.Close()

	pkgReader := resp.Body

	contentType := resp.Header.Get("content-type")
	if !contentTypeIsZip(contentType) {
		return nil, fmt.Errorf("unexpected content-type: %s (expected any of %q)",
			contentType, zipMimeTypes)
	}

	expectedSize := resp.ContentLength

	pkgFile, err := os.CreateTemp("", pb.Filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		pkgFile.Close()
		filePath := pkgFile.Name()
		err = os.Remove(filePath)
		if err != nil {
			d.Logger.Printf("failed to delete unpacked archive at %s: %s", filePath, err)
			return
		}
		d.Logger.Printf("deleted unpacked archive at %s", filePath)
	}()

	up = &UnpackedProduct{}

	d.Logger.Printf("copying %q (%d bytes) to %s", pb.Filename, expectedSize, pkgFile.Name())

	var bytesCopied int64
	if d.VerifyChecksum {
		d.Logger.Printf("verifying checksum of %q", pb.Filename)
		h := sha256.New()
		r := io.TeeReader(resp.Body, pkgFile)

		bytesCopied, err = io.Copy(h, r)
		if err != nil {
			return nil, err
		}

		calculatedSum := h.Sum(nil)
		if !bytes.Equal(calculatedSum, verifiedChecksum) {
			return up, fmt.Errorf(
				"checksum mismatch (expected: %x, got: %x)",
				verifiedChecksum, calculatedSum,
			)
		}
	} else {
		bytesCopied, err = io.Copy(pkgFile, pkgReader)
		if err != nil {
			return up, err
		}
	}

	d.Logger.Printf("copied %d bytes to %s", bytesCopied, pkgFile.Name())

	if expectedSize != 0 && bytesCopied != int64(expectedSize) {
		return up, fmt.Errorf(
			"unexpected size (downloaded: %d, expected: %d)",
			bytesCopied, expectedSize,
		)
	}

	r, err := zip.OpenReader(pkgFile.Name())
	if err != nil {
		return up, err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.Contains(f.Name, "..") {
			// While we generally trust the source ZIP file
			// we still reject path traversal attempts as a precaution.
			continue
		}
		srcFile, err := f.Open()
		if err != nil {
			return up, err
		}

		// Determine the appropriate destination file path
		dstDir := binDir
		// for license files, use binDir if licenseDir is not set
		if isLicenseFile(f.Name) && licenseDir != "" {
			dstDir = licenseDir
		}

		d.Logger.Printf("unpacking %s to %s", f.Name, dstDir)
		dstPath := filepath.Join(dstDir, f.Name)

		if isLicenseFile(f.Name) {
			up.PathsToRemove = append(up.PathsToRemove, dstPath)
		}

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return up, err
		}

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return up, err
		}
		srcFile.Close()
		dstFile.Close()
	}

	return up, nil
}

// The production release site uses consistent single mime type
// but mime types are platform-dependent
// and we may use different OS under test
var zipMimeTypes = []string{
	"application/x-zip-compressed", // Windows
	"application/zip",              // Unix
}

func contentTypeIsZip(contentType string) bool {
	for _, mt := range zipMimeTypes {
		if mt == contentType {
			return true
		}
	}
	return false
}

// Product archives may have a few license files
// which may be extracted to a separate directory
// and may need to be tracked for later cleanup.
var licenseFiles = []string{
	"EULA.txt",
	"TermsOfEvaluation.txt",
	"LICENSE.txt",
}

func isLicenseFile(filename string) bool {
	for _, lf := range licenseFiles {
		if lf == filename {
			return true
		}
	}
	return false
}

// determineArchiveURL determines the archive URL based on the base URL provided.
func determineArchiveURL(archiveURL, baseURL string) (string, error) {
	// If custom URL is set, use that instead of the one from the JSON.
	// Also ensures that absolute download links from mocked responses
	// are still pointing to the mock server if one is set.
	if baseURL == "" {
		return archiveURL, nil
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(archiveURL)
	if err != nil {
		return "", err
	}

	// Use base URL path and append the path from the archive URL.
	newArchiveURL := base.JoinPath(u.Path)

	return newArchiveURL.String(), nil
}
