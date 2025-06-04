package kubevirt

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"

	consolev1client "github.com/openshift/client-go/console/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

type Client struct {
	oc      *exutil.CLI
	virtCtl string
}

func NewClient(oc *exutil.CLI, tmpDir string) (*Client, error) {
	virtCtl, err := ensureVirtctl(oc, tmpDir)
	if err != nil {
		return nil, err
	}
	return &Client{
		oc:      oc,
		virtCtl: virtCtl,
	}, nil
}

func (c *Client) Apply(resource string) error {
	_, err := e2ekubectl.RunKubectlInput(c.oc.Namespace(), resource, "apply", "-f", "-")
	return err
}

func (c *Client) virtctl(args []string) (string, error) {
	output, err := exec.Command(c.virtCtl, args...).CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (c *Client) CreateVM(vmTemplate string, params CreationTemplateParams) error {
	vmResource, err := renderVMTemplate(vmTemplate, params)
	if err != nil {
		return err
	}
	return c.Apply(vmResource)
}

func (c *Client) CreateVMIM(vmiName string) error {
	vmim, err := renderVMTemplate(vmimTemplate, CreationTemplateParams{
		VMNamespace: c.oc.Namespace(),
		VMName:      vmiName,
	})
	if err != nil {
		return err
	}
	return c.Apply(vmim)
}

func (c *Client) Restart(vmName string) error {
	_, err := c.virtctl([]string{"restart", "-n", c.oc.Namespace(), vmName})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Console(vmName, command string) (string, error) {
	return RunCommand(c.virtCtl, c.oc.Namespace(), vmName, command, 5*time.Second)
}

func (c *Client) Login(vmName, hostname string) error {
	return LoginToFedoraWithHostname(c.virtCtl, c.oc.Namespace(), vmName, "fedora", "fedora", hostname)
}
func (c *Client) GetJSONPath(resource, name, jsonPath string) (string, error) {
	output, err := c.oc.Run("get").Args(resource, name, "-o", fmt.Sprintf(`jsonpath=%q`, jsonPath)).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimPrefix(output, `"`), `"`), nil
}
func ensureVirtctl(oc *exutil.CLI, dir string) (string, error) {
	url, err := discoverVirtctlURL(oc)
	if err != nil {
		return "", err
	}
	filepath := filepath.Join(dir, "virtctl")
	_, err = os.Stat(filepath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := downloadFile(url, filepath); err != nil {
				return "", err
			}
			if err := os.Chmod(filepath, 0755); err != nil {
				log.Fatal(err)
			}
			return filepath, nil
		}
		return "", err
	}
	return filepath, err
}

func discoverVirtctlURL(oc *exutil.CLI) (string, error) {
	consoleClient, err := consolev1client.NewForConfig(oc.AsAdmin().UserConfig())
	if err != nil {
		return "", err
	}
	virtctlCliDownload, err := consoleClient.ConsoleV1().ConsoleCLIDownloads().Get(context.Background(), "virtctl-clidownloads-kubevirt-hyperconverged", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, virtctlLink := range virtctlCliDownload.Spec.Links {
		if strings.Contains(virtctlLink.Text, "x86_64") {
			return virtctlLink.Href, nil
		}
	}
	return "", fmt.Errorf("missing virtctl for x86_64 arch")
}

func downloadFile(url string, filepath string) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(gzipReader)
	for true {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeReg {
			out, err := os.Create(filepath)
			if err != nil {
				return err
			}
			defer out.Close()
			if _, err := io.Copy(out, tarReader); err != nil {
				return err
			}
		}
	}
	return nil
}
