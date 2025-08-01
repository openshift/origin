// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package releasesjson

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/hashicorp/hc-install/internal/httpclient"
)

type ChecksumDownloader struct {
	ProductVersion   *ProductVersion
	Logger           *log.Logger
	ArmoredPublicKey string

	BaseURL string
}

type ChecksumFileMap map[string]HashSum

type HashSum []byte

func (hs HashSum) Size() int {
	return len(hs)
}

func (hs HashSum) String() string {
	return hex.EncodeToString(hs)
}

func HashSumFromHexDigest(hexDigest string) (HashSum, error) {
	sumBytes, err := hex.DecodeString(hexDigest)
	if err != nil {
		return nil, err
	}
	return HashSum(sumBytes), nil
}

func (cd *ChecksumDownloader) DownloadAndVerifyChecksums(ctx context.Context) (ChecksumFileMap, error) {
	sigFilename, err := cd.findSigFilename(cd.ProductVersion)
	if err != nil {
		return nil, err
	}

	client := httpclient.NewHTTPClient(cd.Logger)
	sigURL := fmt.Sprintf("%s/%s/%s/%s", cd.BaseURL,
		url.PathEscape(cd.ProductVersion.Name),
		url.PathEscape(cd.ProductVersion.Version.String()),
		url.PathEscape(sigFilename))
	cd.Logger.Printf("downloading signature from %s", sigURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sigURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %q: %w", sigURL, err)
	}
	sigResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if sigResp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download signature from %q: %s", sigURL, sigResp.Status)
	}

	defer sigResp.Body.Close()

	shasumsURL := fmt.Sprintf("%s/%s/%s/%s", cd.BaseURL,
		url.PathEscape(cd.ProductVersion.Name),
		url.PathEscape(cd.ProductVersion.Version.String()),
		url.PathEscape(cd.ProductVersion.SHASUMS))
	cd.Logger.Printf("downloading checksums from %s", shasumsURL)

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, shasumsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %q: %w", shasumsURL, err)
	}
	sumsResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if sumsResp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download checksums from %q: %s", shasumsURL, sumsResp.Status)
	}

	defer sumsResp.Body.Close()

	var shaSums strings.Builder
	sumsReader := io.TeeReader(sumsResp.Body, &shaSums)

	err = cd.verifySumsSignature(sumsReader, sigResp.Body)
	if err != nil {
		return nil, err
	}

	return fileMapFromChecksums(shaSums)
}

func fileMapFromChecksums(checksums strings.Builder) (ChecksumFileMap, error) {
	csMap := make(ChecksumFileMap, 0)

	lines := strings.Split(checksums.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected checksum line format: %q", line)
		}

		h, err := HashSumFromHexDigest(parts[0])
		if err != nil {
			return nil, err
		}

		if h.Size() != sha256.Size {
			return nil, fmt.Errorf("unexpected sha256 format (len: %d, expected: %d)",
				h.Size(), sha256.Size)
		}

		csMap[parts[1]] = h
	}
	return csMap, nil
}

func (cd *ChecksumDownloader) verifySumsSignature(checksums, signature io.Reader) error {
	el, err := cd.keyEntityList()
	if err != nil {
		return err
	}

	_, err = openpgp.CheckDetachedSignature(el, checksums, signature, nil)
	if err != nil {
		return fmt.Errorf("unable to verify checksums signature: %w", err)
	}

	cd.Logger.Printf("checksum signature is valid")

	return nil
}

func (cd *ChecksumDownloader) findSigFilename(pv *ProductVersion) (string, error) {
	sigFiles := pv.SHASUMSSigs
	if len(sigFiles) == 0 {
		sigFiles = []string{pv.SHASUMSSig}
	}

	keyIds, err := cd.pubKeyIds()
	if err != nil {
		return "", err
	}

	for _, filename := range sigFiles {
		for _, keyID := range keyIds {
			if strings.HasSuffix(filename, fmt.Sprintf("_SHA256SUMS.%s.sig", keyID)) {
				return filename, nil
			}
		}
		if strings.HasSuffix(filename, "_SHA256SUMS.sig") {
			return filename, nil
		}
	}

	return "", fmt.Errorf("no suitable sig file found")
}

func (cd *ChecksumDownloader) pubKeyIds() ([]string, error) {
	entityList, err := cd.keyEntityList()
	if err != nil {
		return nil, err
	}

	fingerprints := make([]string, 0)
	for _, entity := range entityList {
		fingerprints = append(fingerprints, entity.PrimaryKey.KeyIdShortString())
	}

	return fingerprints, nil
}

func (cd *ChecksumDownloader) keyEntityList() (openpgp.EntityList, error) {
	if cd.ArmoredPublicKey == "" {
		return nil, fmt.Errorf("no public key provided")
	}
	return openpgp.ReadArmoredKeyRing(strings.NewReader(cd.ArmoredPublicKey))
}
