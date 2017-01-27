package image

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var (
	verifyImageSignatureLongDesc = templates.LongDesc(`
	Verifies the imported image signature using the local public key.

	This command verifies if the signature attached to an image is trusted by
	using the provided public GPG key.
	Trusted image means that the image signature was signed by a valid GPG key and the image identity
	provided by the signature content matches with the image.
	By default, this command will not record a signature condition back to the Image object but only
	print the verification status to the console.

	To record a new condition, you have to pass the "--confirm" flag.
	`)

	verifyImageSignatureExample = templates.Examples(`
	# Verify the image signature using the local GNUPG keychan and record the status as a condition to image
	%[1]s sha256:c841e9b64e4579bd56c794bdd7c36e1c257110fd2404bebbb8b613e4935228c4 --expected-identity=registry.local:5000/foo/bar:v1
	`)
)

type VerifyImageSignatureOptions struct {
	InputImage        string
	ExpectedIdentity  string
	PublicKeyFilename string
	PublicKey         []byte
	Confirm           bool
	Remove            bool
	CurrentUser       string

	Client client.Interface
	Out    io.Writer
	ErrOut io.Writer
}

func NewCmdVerifyImageSignature(name, fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	opts := &VerifyImageSignatureOptions{ErrOut: errOut, Out: out}
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s IMAGE [--confirm]", name),
		Short:   "Verify that the given IMAGE signature is trusted",
		Long:    verifyImageSignatureLongDesc,
		Example: fmt.Sprintf(verifyImageSignatureExample, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Run())
		},
	}

	cmd.Flags().BoolVar(&opts.Confirm, "confirm", opts.Confirm, "If true, the result of the verification will be recorded to an image object.")
	cmd.Flags().BoolVar(&opts.Remove, "remove", opts.Remove, "If set, all signature verifications will be removed from the given image.")
	cmd.Flags().StringVar(&opts.PublicKeyFilename, "public-key", opts.PublicKeyFilename, "A path to a public GPG key to be used for verification.")
	cmd.Flags().StringVar(&opts.ExpectedIdentity, "expected-identity", opts.ExpectedIdentity, "An expected image docker reference to verify.")
	return cmd
}

func (o *VerifyImageSignatureOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) != 1 {
		return kcmdutil.UsageError(cmd, "exactly one image must be specified")
	}
	o.InputImage = args[0]
	if len(o.ExpectedIdentity) == 0 {
		return kcmdutil.UsageError(cmd, "the --expected-identity must be specified")
	}
	var err error

	// If --public-key is provided only this key will be used for verification and the
	// .gnupg/pubring.gpg will be ignored.
	if len(o.PublicKeyFilename) > 0 {
		if o.Remove {
			return kcmdutil.UsageError(cmd, "cannot use public key when removing verification status")
		}
		if o.PublicKey, err = ioutil.ReadFile(o.PublicKeyFilename); err != nil {
			return err
		}
	}
	if o.Client, _, err = f.Clients(); err != nil {
		return err
	}
	// Only make this API call when we are sure we will be writing validation.
	if o.Confirm && !o.Remove {
		if me, err := o.Client.Users().Get("~"); err != nil {
			return err
		} else {
			o.CurrentUser = me.Name
		}
	}

	return nil
}

// verifySignature verifies the image signature and returns the identity when the signature
// is valid.
// TODO: This should be calling the 'containers/image' library in future.
func (o *VerifyImageSignatureOptions) verifySignature(signature []byte) (string, []byte, error) {
	var (
		mechanism SigningMechanism
		err       error
	)
	// If public key is specified, use JUST that key for verification. Otherwise use all
	// keys in local GPG public keyring.
	if len(o.PublicKeyFilename) == 0 {
		mechanism, err = newGPGSigningMechanismInDirectory("")
	} else {
		mechanism, _, err = newEphemeralGPGSigningMechanism(o.PublicKey)
	}
	if err != nil {
		return "", nil, err
	}
	defer mechanism.Close()
	content, identity, err := mechanism.Verify(signature)
	if err != nil {
		return "", nil, err
	}
	return string(identity), content, nil
}

// verifySignatureContent verifies that the signature content matches the given image.
// TODO: This should be done by calling the 'containers/image' library in future.
func (o *VerifyImageSignatureOptions) verifySignatureContent(content []byte) (string, error) {
	// TODO: The types here are just to decompose the JSON. The fields should not change but
	// we need to use containers/image library here to guarantee compatibility in future.
	type criticalImage struct {
		Digest string `json:"docker-manifest-digest"`
	}
	type criticalIdentity struct {
		DockerReference string `json:"docker-reference"`
	}
	type critical struct {
		Image    criticalImage    `json:"image"`
		Identity criticalIdentity `json:"identity"`
	}
	type message struct {
		Critical critical `json:"critical"`
	}
	m := message{}
	if err := json.Unmarshal(content, &m); err != nil {
		return "", err
	}
	if o.InputImage != m.Critical.Image.Digest {
		return "", fmt.Errorf("signature is valid for digest %q not for %q", m.Critical.Image.Digest, o.InputImage)
	}
	return m.Critical.Identity.DockerReference, nil
}

// verifyImageIdentity verifies the source of the image specified in the signature is
// valid.
func (o *VerifyImageSignatureOptions) verifyImageIdentity(reference string) error {
	if reference != o.ExpectedIdentity {
		return fmt.Errorf("signature identity %q does not match expected image identity %q", reference, o.ExpectedIdentity)
	}
	return nil
}

// clearSignatureVerificationStatus removes the current image signature from the Image object by
// erasing all signature fields that were previously set (when image signature was
// previously verified).
func (o *VerifyImageSignatureOptions) clearSignatureVerificationStatus(s *imageapi.ImageSignature) {
	s.Conditions = []imageapi.SignatureCondition{}
	s.IssuedBy = nil
}

func (o *VerifyImageSignatureOptions) Run() error {
	img, err := o.Client.Images().Get(o.InputImage)
	if err != nil {
		return err
	}
	if len(img.Signatures) == 0 {
		return fmt.Errorf("%s does not have any signature", img.Name)
	}

	for i, s := range img.Signatures {
		// If --remove is specified, just erase the existing signature verification for all
		// signatures.
		// TODO: This should probably need to handle removal of a single signature.
		if o.Remove {
			o.clearSignatureVerificationStatus(&img.Signatures[i])
			continue
		}

		// Verify the signature against the public key
		signedBy, content, signatureErr := o.verifySignature(s.Content)
		if signatureErr != nil {
			fmt.Fprintf(o.ErrOut, "%s signature cannot be verified: %v\n", o.InputImage, signatureErr)
		}

		// Verify the signed message content matches with the provided image id
		identity, signatureContentErr := o.verifySignatureContent(content)
		if signatureContentErr != nil {
			fmt.Fprintf(o.ErrOut, "%s signature content cannot be verified: %v\n", o.InputImage, signatureContentErr)
		}

		identityError := o.verifyImageIdentity(identity)
		if identityError != nil {
			fmt.Fprintf(o.ErrOut, "%s identity cannot be verified: %v\n", o.InputImage, identityError)
		}

		if signatureErr != nil || signatureContentErr != nil || identityError != nil {
			o.clearSignatureVerificationStatus(&img.Signatures[i])
			continue
		}

		fmt.Fprintf(o.Out, "%s signature is verified (signed by key: %q)\n", o.InputImage, signedBy)

		now := unversioned.Now()
		newConditions := []imageapi.SignatureCondition{
			{
				Type:               imageapi.SignatureTrusted,
				Status:             kapi.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
				Reason:             "verified manually",
				Message:            fmt.Sprintf("verified by user %s", o.CurrentUser),
			},
			// FIXME: This condition is required to be set for validation.
			{
				Type:               imageapi.SignatureForImage,
				Status:             kapi.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
			},
		}
		img.Signatures[i].Conditions = newConditions
		img.Signatures[i].IssuedBy = &imageapi.SignatureIssuer{}
		// TODO: This should not be just a key id but a human-readable identity.
		img.Signatures[i].IssuedBy.CommonName = signedBy
	}

	if o.Confirm {
		_, err := o.Client.Images().Update(img)
		return err
	}
	fmt.Fprintf(o.Out, "(add --confirm to record signature verification status to server)\n")
	return nil
}
