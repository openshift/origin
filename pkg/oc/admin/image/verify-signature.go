package image

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/docker/policyconfiguration"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/signature"
	sigtypes "github.com/containers/image/types"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	verifyImageSignatureLongDesc = templates.LongDesc(`
	Verifies the image signature of an image imported to internal registry using the local public GPG key.

	This command verifies if the image identity contained in the image signature can be trusted
	by using the public GPG key to verify the signature itself and matching the provided expected identity
	with the identity (pull spec) of the given image.
	By default, this command will use the public GPG keyring located in "$GNUPGHOME/.gnupg/pubring.gpg"

	By default, this command will not save the result of the verification back to the image object, to do so
	user have to specify the "--save" flag. Note that to modify the image signature verification status,
	user have to have permissions to edit an image object (usually an "image-auditor" role).

	Note that using the "--save" flag on already verified image together with invalid GPG
	key or invalid expected identity will cause the saved verification status to be removed
	and the image will become "unverified".

	If this command is outside the cluster, users have to specify the "--registry-url" parameter
	with the public URL of image registry.

	To remove all verifications, users can use the "--remove-all" flag.
	`)

	verifyImageSignatureExample = templates.Examples(`
	# Verify the image signature and identity using the local GPG keychain
	%[1]s sha256:c841e9b64e4579bd56c794bdd7c36e1c257110fd2404bebbb8b613e4935228c4 \
			--expected-identity=registry.local:5000/foo/bar:v1

	# Verify the image signature and identity using the local GPG keychain and save the status
	%[1]s sha256:c841e9b64e4579bd56c794bdd7c36e1c257110fd2404bebbb8b613e4935228c4 \
			--expected-identity=registry.local:5000/foo/bar:v1 --save

	# Verify the image signature and identity via exposed registry route
	%[1]s sha256:c841e9b64e4579bd56c794bdd7c36e1c257110fd2404bebbb8b613e4935228c4 \
			--expected-identity=registry.local:5000/foo/bar:v1 \
			--registry-url=docker-registry.foo.com

	# Remove all signature verifications from the image
	%[1]s sha256:c841e9b64e4579bd56c794bdd7c36e1c257110fd2404bebbb8b613e4935228c4 --remove-all
	`)
)

type VerifyImageSignatureOptions struct {
	InputImage        string
	ExpectedIdentity  string
	PublicKeyFilename string
	PublicKey         []byte
	Save              bool
	RemoveAll         bool
	CurrentUser       string
	CurrentUserToken  string
	RegistryURL       string
	Insecure          bool

	ImageClient imageclient.ImageInterface

	clientConfig kclientcmd.ClientConfig

	Out    io.Writer
	ErrOut io.Writer
}

const (
	VerifyRecommendedName = "verify-image-signature"
)

func NewCmdVerifyImageSignature(name, fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	opts := &VerifyImageSignatureOptions{
		ErrOut:       errOut,
		Out:          out,
		clientConfig: f.OpenShiftClientConfig(),
		// TODO: This improves the error message users get when containers/image is not able
		// to locate the pubring.gpg file (which is default).
		// This should be improved/fixed in containers/image.
		PublicKeyFilename: filepath.Join(os.Getenv("GNUPGHOME"), "pubring.gpg"),
	}
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s IMAGE --expected-identity=EXPECTED_IDENTITY [--save]", VerifyRecommendedName),
		Short:   "Verify the image identity contained in the image signature",
		Long:    verifyImageSignatureLongDesc,
		Example: fmt.Sprintf(verifyImageSignatureExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Validate())
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Run())
		},
	}

	cmd.Flags().StringVar(&opts.ExpectedIdentity, "expected-identity", opts.ExpectedIdentity, "An expected image docker reference to verify (required).")
	cmd.Flags().BoolVar(&opts.Save, "save", opts.Save, "If true, the result of the verification will be saved to an image object.")
	cmd.Flags().BoolVar(&opts.RemoveAll, "remove-all", opts.RemoveAll, "If set, all signature verifications will be removed from the given image.")
	cmd.Flags().StringVar(&opts.PublicKeyFilename, "public-key", opts.PublicKeyFilename, fmt.Sprintf("A path to a public GPG key to be used for verification. (defaults to %q)", opts.PublicKeyFilename))
	cmd.Flags().StringVar(&opts.RegistryURL, "registry-url", opts.RegistryURL, "The address to use when contacting the registry, instead of using the internal cluster address. This is useful if you can't resolve or reach the internal registry address.")
	cmd.Flags().BoolVar(&opts.Insecure, "insecure", opts.Insecure, "If set, use the insecure protocol for registry communication.")
	return cmd
}

func (o *VerifyImageSignatureOptions) Validate() error {
	if !o.RemoveAll {
		if len(o.ExpectedIdentity) == 0 {
			return errors.New("the --expected-identity is required")
		}
		if _, err := imageapi.ParseDockerImageReference(o.ExpectedIdentity); err != nil {
			return errors.New("the --expected-identity must be valid image reference")
		}
	}
	if o.RemoveAll && len(o.ExpectedIdentity) > 0 {
		return errors.New("the --expected-identity cannot be used when removing all verifications")
	}
	return nil
}
func (o *VerifyImageSignatureOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) != 1 {
		return kcmdutil.UsageErrorf(cmd, "exactly one image must be specified")
	}
	o.InputImage = args[0]
	var err error

	if len(o.PublicKeyFilename) > 0 {
		if o.PublicKey, err = ioutil.ReadFile(o.PublicKeyFilename); err != nil {
			return fmt.Errorf("unable to read --public-key: %v", err)
		}
	}
	imageClient, err := f.OpenshiftInternalImageClient()
	if err != nil {
		return err
	}
	o.ImageClient = imageClient.Image()

	userClient, err := f.OpenshiftInternalUserClient()
	if err != nil {
		return err
	}

	// We need the current user name so we can record it into an verification condition and
	// we need a bearer token so we can fetch the manifest from the registry.
	// TODO: Add support for external registries (currently only integrated registry will
	if me, err := userClient.User().Users().Get("~", metav1.GetOptions{}); err != nil {
		return err
	} else {
		o.CurrentUser = me.Name
		if config, err := o.clientConfig.ClientConfig(); err != nil {
			return err
		} else {
			if o.CurrentUserToken = config.BearerToken; len(o.CurrentUserToken) == 0 {
				return fmt.Errorf("no token is currently in use for this session")
			}
		}
	}

	return nil
}

func (o VerifyImageSignatureOptions) Run() error {
	img, err := o.ImageClient.Images().Get(o.InputImage, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if len(img.Signatures) == 0 {
		return fmt.Errorf("%s does not have any signature", img.Name)
	}

	pr, err := signature.NewPRSignedByKeyPath(signature.SBKeyTypeGPGKeys, o.PublicKeyFilename, signature.NewPRMMatchRepoDigestOrExact())
	if err != nil {
		return fmt.Errorf("unable to prepare verification policy requirements: %v", err)
	}
	policy := signature.Policy{Default: []signature.PolicyRequirement{pr}}
	pc, err := signature.NewPolicyContext(&policy)
	if err != nil {
		return fmt.Errorf("unable to setup policy: %v", err)
	}
	defer pc.Destroy()

	if o.RemoveAll {
		img.Signatures = []imageapi.ImageSignature{}
	}

	for i, s := range img.Signatures {
		// Verify the signature against the policy
		signedBy, err := o.verifySignature(pc, img, s.Content)
		if err != nil {
			fmt.Fprintf(o.ErrOut, "error verifying signature %s for image %s (verification status will be removed): %v\n", img.Signatures[i].Name, o.InputImage, err)
			img.Signatures[i] = imageapi.ImageSignature{}
			continue
		}
		fmt.Fprintf(o.Out, "image %q identity is now confirmed (signed by GPG key %q)\n", o.InputImage, signedBy)

		now := metav1.Now()
		newConditions := []imageapi.SignatureCondition{
			{
				Type:               imageapi.SignatureTrusted,
				Status:             kapi.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
				Reason:             "manually verified",
				Message:            fmt.Sprintf("verified by user %q", o.CurrentUser),
			},
			// TODO: This should be not needed (need to relax validation).
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

	if o.Save || o.RemoveAll {
		_, err := o.ImageClient.Images().Update(img)
		return err
	}
	return nil
}

// getImageManifest fetches the manifest for provided image from the integrated registry.
func (o *VerifyImageSignatureOptions) getImageManifest(img *imageapi.Image) ([]byte, error) {
	parsed, err := imageapi.ParseDockerImageReference(img.DockerImageReference)
	if err != nil {
		return nil, err
	}
	registryURL := parsed.RegistryURL()
	if len(o.RegistryURL) > 0 {
		registryURL = &url.URL{Host: o.RegistryURL, Scheme: "https"}
		if o.Insecure {
			registryURL.Scheme = ""
		}
	}
	return getImageManifestByIDFromRegistry(registryURL, parsed.RepositoryName(), img.Name, o.CurrentUser, o.CurrentUserToken, o.Insecure)
}

// verifySignature takes policy, image and the image signature blob and verifies that the
// signature was signed by a trusted key, the expected identity matches the one in the
// signature message and the manifest matches as well.
// In case the image identity is confirmed, this function returns the matching GPG key in
// short form, otherwise it returns rejection reason.
func (o *VerifyImageSignatureOptions) verifySignature(pc *signature.PolicyContext, img *imageapi.Image, sigBlob []byte) (string, error) {
	manifest, err := o.getImageManifest(img)
	if err != nil {
		return "", fmt.Errorf("failed to get image %q manifest: %v", img.Name, err)
	}
	allowed, err := pc.IsRunningImageAllowed(newUnparsedImage(o.ExpectedIdentity, sigBlob, manifest))
	if !allowed && err == nil {
		return "", errors.New("signature rejected but no error set")
	}
	if err != nil {
		return "", fmt.Errorf("signature rejected: %v", err)
	}
	if untrustedInfo, err := signature.GetUntrustedSignatureInformationWithoutVerifying(sigBlob); err != nil {
		// Tis is treated as an unverified signature. It really shouldn’t happen anyway.
		return "", fmt.Errorf("error getting signing key identity: %v", err)
	} else {
		return untrustedInfo.UntrustedShortKeyIdentifier, nil
	}
}

// dummyDockerTransport is containers/image/docker.Transport, except that it only provides identity information.
var dummyDockerTransport = dockerTransport{}

type dockerTransport struct{}

func (t dockerTransport) Name() string {
	return "docker"
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func (t dockerTransport) ParseReference(reference string) (sigtypes.ImageReference, error) {
	return parseDockerReference(reference)
}

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (t dockerTransport) ValidatePolicyConfigurationScope(scope string) error {
	// FIXME? We could be verifying the various character set and length restrictions
	// from docker/distribution/reference.regexp.go, but other than that there
	// are few semantically invalid strings.
	return nil
}

// dummyDockerReference is containers/image/docker.Reference, except that only provides identity information.
type dummyDockerReference struct{ ref reference.Named }

// parseDockerReference converts a string, which should not start with the ImageTransport.Name prefix, into an Docker ImageReference.
func parseDockerReference(refString string) (sigtypes.ImageReference, error) {
	if !strings.HasPrefix(refString, "//") {
		return nil, fmt.Errorf("docker: image reference %s does not start with //", refString)
	}
	ref, err := reference.ParseNormalizedNamed(strings.TrimPrefix(refString, "//"))
	if err != nil {
		return nil, err
	}
	ref = reference.TagNameOnly(ref)

	if reference.IsNameOnly(ref) {
		return nil, fmt.Errorf("Docker reference %s has neither a tag nor a digest", reference.FamiliarString(ref))
	}
	// A github.com/distribution/reference value can have a tag and a digest at the same time!
	// The docker/distribution API does not really support that (we can’t ask for an image with a specific
	// tag and digest), so fail.  This MAY be accepted in the future.
	// (Even if it were supported, the semantics of policy namespaces are unclear - should we drop
	// the tag or the digest first?)
	_, isTagged := ref.(reference.NamedTagged)
	_, isDigested := ref.(reference.Canonical)
	if isTagged && isDigested {
		return nil, fmt.Errorf("Docker references with both a tag and digest are currently not supported")
	}
	return dummyDockerReference{
		ref: ref,
	}, nil
}

func (ref dummyDockerReference) Transport() sigtypes.ImageTransport {
	return dummyDockerTransport
}

// StringWithinTransport returns a string representation of the reference, which MUST be such that
// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
// NOTE: The returned string is not promised to be equal to the original input to ParseReference;
// e.g. default attribute values omitted by the user may be filled in in the return value, or vice versa.
// WARNING: Do not use the return value in the UI to describe an image, it does not contain the Transport().Name() prefix.
func (ref dummyDockerReference) StringWithinTransport() string {
	return "//" + reference.FamiliarString(ref.ref)
}

// DockerReference returns a Docker reference associated with this reference
// (fully explicit, i.e. !reference.IsNameOnly, but reflecting user intent,
// not e.g. after redirect or alias processing), or nil if unknown/not applicable.
func (ref dummyDockerReference) DockerReference() reference.Named {
	return ref.ref
}

// PolicyConfigurationIdentity returns a string representation of the reference, suitable for policy lookup.
// This MUST reflect user intent, not e.g. after processing of third-party redirects or aliases;
// The value SHOULD be fully explicit about its semantics, with no hidden defaults, AND canonical
// (i.e. various references with exactly the same semantics should return the same configuration identity)
// It is fine for the return value to be equal to StringWithinTransport(), and it is desirable but
// not required/guaranteed that it will be a valid input to Transport().ParseReference().
// Returns "" if configuration identities for these references are not supported.
func (ref dummyDockerReference) PolicyConfigurationIdentity() string {
	res, err := policyconfiguration.DockerReferenceIdentity(ref.ref)
	if res == "" || err != nil { // Coverage: Should never happen, NewReference above should refuse values which could cause a failure.
		panic(fmt.Sprintf("Internal inconsistency: policyconfiguration.DockerReferenceIdentity returned %#v, %v", res, err))
	}
	return res
}

// PolicyConfigurationNamespaces returns a list of other policy configuration namespaces to search
// for if explicit configuration for PolicyConfigurationIdentity() is not set.  The list will be processed
// in order, terminating on first match, and an implicit "" is always checked at the end.
// It is STRONGLY recommended for the first element, if any, to be a prefix of PolicyConfigurationIdentity(),
// and each following element to be a prefix of the element preceding it.
func (ref dummyDockerReference) PolicyConfigurationNamespaces() []string {
	return policyconfiguration.DockerReferenceNamespaces(ref.ref)
}

func (ref dummyDockerReference) NewImage(ctx *sigtypes.SystemContext) (sigtypes.Image, error) {
	panic("Unimplemented")
}
func (ref dummyDockerReference) NewImageSource(ctx *sigtypes.SystemContext, requestedManifestMIMETypes []string) (sigtypes.ImageSource, error) {
	panic("Unimplemented")
}
func (ref dummyDockerReference) NewImageDestination(ctx *sigtypes.SystemContext) (sigtypes.ImageDestination, error) {
	panic("Unimplemented")
}
func (ref dummyDockerReference) DeleteImage(ctx *sigtypes.SystemContext) error {
	panic("Unimplemented")
}

// unparsedImage implements sigtypes.UnparsedImage, to allow evaluating the signature policy
// against an image without having to make it pullable by containers/image
type unparsedImage struct {
	ref       sigtypes.ImageReference
	manifest  []byte
	signature []byte
}

func newUnparsedImage(expectedIdentity string, signature, manifest []byte) sigtypes.UnparsedImage {
	// We check the error in Validate()
	ref, _ := parseDockerReference("//" + expectedIdentity)
	return &unparsedImage{ref: ref, manifest: manifest, signature: signature}
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (ui *unparsedImage) Reference() sigtypes.ImageReference {
	return ui.ref
}

// Close removes resources associated with an initialized UnparsedImage, if any.
func (ui *unparsedImage) Close() error {
	return nil
}

// Manifest is like ImageSource.GetManifest, but the result is cached; it is OK to call this however often you need.
func (ui *unparsedImage) Manifest() ([]byte, string, error) {
	return ui.manifest, "", nil
}

// Signatures is like ImageSource.GetSignatures, but the result is cached; it is OK to call this however often you need.
func (ui *unparsedImage) Signatures(context.Context) ([][]byte, error) {
	return [][]byte{ui.signature}, nil
}
