package main

import (
	"fmt"
	"os"

	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/archive"
	"github.com/kubernetes-incubator/cri-o/libpod/common"
	"github.com/kubernetes-incubator/cri-o/libpod/images"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	pushFlags = []cli.Flag{
		cli.StringFlag{
			Name:   "signature-policy",
			Usage:  "`pathname` of signature policy file (not usually used)",
			Hidden: true,
		},
		cli.StringFlag{
			Name:  "creds",
			Usage: "`credentials` (USERNAME:PASSWORD) to use for authenticating to a registry",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Usage: "`pathname` of a directory containing TLS certificates and keys",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when contacting registries (default: true)",
		},
		cli.BoolFlag{
			Name:  "remove-signatures",
			Usage: "discard any pre-existing signatures in the image",
		},
		cli.StringFlag{
			Name:  "sign-by",
			Usage: "add a signature at the destination using the specified key",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "don't output progress information when pushing images",
		},
	}
	pushDescription = fmt.Sprintf(`
   Pushes an image to a specified location.
   The Image "DESTINATION" uses a "transport":"details" format.
   See kpod-push(1) section "DESTINATION" for the expected format`)

	pushCommand = cli.Command{
		Name:        "push",
		Usage:       "push an image to a specified destination",
		Description: pushDescription,
		Flags:       pushFlags,
		Action:      pushCmd,
		ArgsUsage:   "IMAGE DESTINATION",
	}
)

func pushCmd(c *cli.Context) error {
	var registryCreds *types.DockerAuthConfig

	args := c.Args()
	if len(args) < 2 {
		return errors.New("kpod push requires exactly 2 arguments")
	}
	srcName := c.Args().Get(0)
	destName := c.Args().Get(1)

	signaturePolicy := c.String("signature-policy")
	registryCredsString := c.String("creds")
	certPath := c.String("cert-dir")
	skipVerify := !c.BoolT("tls-verify")
	removeSignatures := c.Bool("remove-signatures")
	signBy := c.String("sign-by")

	if registryCredsString != "" {
		creds, err := common.ParseRegistryCreds(registryCredsString)
		if err != nil {
			if err == common.ErrNoPassword {
				fmt.Print("Password: ")
				password, err := terminal.ReadPassword(0)
				if err != nil {
					return errors.Wrapf(err, "could not read password from terminal")
				}
				creds.Password = string(password)
			} else {
				return err
			}
		}
		registryCreds = creds
	}

	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get config")
	}
	store, err := getStore(config)
	if err != nil {
		return err
	}

	options := images.CopyOptions{
		Compression:         archive.Uncompressed,
		SignaturePolicyPath: signaturePolicy,
		Store:               store,
		DockerRegistryOptions: common.DockerRegistryOptions{
			DockerRegistryCreds:         registryCreds,
			DockerCertPath:              certPath,
			DockerInsecureSkipTLSVerify: skipVerify,
		},
		SigningOptions: common.SigningOptions{
			RemoveSignatures: removeSignatures,
			SignBy:           signBy,
		},
	}
	if !c.Bool("quiet") {
		options.ReportWriter = os.Stderr
	}
	return images.PushImage(srcName, destName, options)
}
