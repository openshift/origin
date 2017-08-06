package push

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	"github.com/openshift/origin/pkg/image/apis/image/v1"
	clientset "github.com/openshift/origin/pkg/image/generated/clientset/typed/image/v1"
)

var (
	pushBinaryLong = templates.LongDesc(`
		Create a new image by pushing an archive

		This command assists users in creating new images that can be used as part of builds
		or as deployed applications. It accepts a zip or tar.gz file that will become a new 
		image in an image stream. You may also specify image metadata like the entrypoint, 
		environment variables, or labels to create a runnable image.

		Experimental: This command is under active development and may change without notice.`)

	pushBinaryExample = templates.Examples(`
		# Upload myblob.zip as a new image in the app:binary image stream tag
	  $ %[1]s binary --to=app:binary myblob.zip

	  # Force upload in case another user is also uploading at the same time.
	  $ %[1]s binary --to=app:binary myblob.zip --force`)
)

type PushBinaryOptions struct {
	Out, ErrOut io.Writer
	In          io.Reader
	Output      string
	PrintObject func([]*resource.Info) error
	Mapper      *resource.Mapper

	Path      string
	BaseImage string
	To        string
	Namespace string
	Force     bool

	Client clientset.ImageStreamsGetter
}

// NewCmdPushBinary creates a new binary image in an image stream
func NewCmdPushBinary(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &PushBinaryOptions{
		Out:    out,
		ErrOut: errout,
		In:     in,
	}
	cmd := &cobra.Command{
		Use:     "binary --to=IMAGESTREAMTAG [ARCHIVE|-]",
		Short:   "Upload a zip or tar.gz file as an image",
		Long:    pushBinaryLong,
		Example: fmt.Sprintf(pushBinaryExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	cmd.MarkFlagRequired("filename")

	cmd.Flags().StringVar(&options.BaseImage, "from", options.BaseImage, "An optional image stream tag or image to use as the base.")
	cmd.Flags().StringVar(&options.To, "to", options.To, "An image stream tag to push to. Can be fully specified or relative to the current project.")
	cmd.Flags().BoolVar(&options.Force, "force", options.Force, "If true, cancel any other pending uploads to this tag.")

	kcmdutil.AddPrinterFlags(cmd)

	return cmd
}

func (o *PushBinaryOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("you must specify exactly one argument - a URL, a path to a zip or tar.gz file, or '-' to use STDIN")
	}
	o.Path = args[0]

	ns, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = ns

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.Client, err = clientset.NewForConfig(config)

	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.PrintObject = func(infos []*resource.Info) error {
		return f.PrintResourceInfos(cmd, false, infos, o.Out)
	}
	o.Mapper = clientcmd.ResourceMapper(f)

	return err
}

func (o *PushBinaryOptions) Validate() error {
	if len(o.To) == 0 {
		return fmt.Errorf("--to=IMAGESTREAMTAG is a required argument")
	}

	to, err := imageapi.ParseDockerImageReference(o.To)
	if err != nil {
		return fmt.Errorf("--to is not a valid reference: %v", err)
	}
	if len(to.Registry) > 0 || len(to.ID) > 0 || len(to.Tag) == 0 {
		return fmt.Errorf("--to must be NAMESPACE/NAME:TAG or NAME:TAG")
	}

	if len(o.BaseImage) > 0 {
		ref, err := imageapi.ParseDockerImageReference(o.BaseImage)
		if err != nil {
			return fmt.Errorf("--from requires a valid image stream tag or image reference: %v", err)
		}
		if len(ref.Registry) > 0 || (len(ref.ID) == 0 && len(ref.Tag) == 0) {
			return fmt.Errorf("--from must be [NAMESPACE/]NAME:TAG or [NAMESPACE/]NAME@ID")
		}
	}
	return nil
}

func (o *PushBinaryOptions) Run() error {
	localPath, r, size, err := contentsForPathOrURL(o.Path, o.In)
	if err != nil {
		return err
	}
	r = convertStreamToTarGz(localPath, r, size)

	to, err := imageapi.ParseDockerImageReference(o.To)
	if err != nil {
		return fmt.Errorf("--to is not a valid reference: %v", err)
	}
	if len(to.Namespace) == 0 {
		to.Namespace = o.Namespace
	}

	imageMetadata := &docker10.DockerImage{
		Config: &docker10.DockerConfig{
			Labels: map[string]string{
				"io.openshift.image.binary": "true",
			},
		},
	}
	clone := &v1.ImageStreamTagInstantiate{
		ObjectMeta: metav1.ObjectMeta{Name: to.NameString(), Namespace: to.Namespace},
		Image: &v1.ImageInstantiateMetadata{
			DockerImageMetadata: runtime.RawExtension{
				Object: imageMetadata,
			},
		},
	}
	if len(o.BaseImage) > 0 {
		ref, err := imageapi.ParseDockerImageReference(o.BaseImage)
		if err != nil {
			return fmt.Errorf("--from requires a valid image stream tag or image reference: %v", err)
		}
		clone.From = &kapiv1.ObjectReference{Name: ref.NameString(), Namespace: ref.Namespace}
		if len(ref.ID) > 0 {
			clone.From.Kind = "ImageStreamImage"
		} else {
			clone.From.Kind = "ImageStreamTag"
		}
		// TODO: clear image metadata
		clone.Image = nil
	}

	result, err := o.Client.ImageStreams(to.Namespace).Instantiate(clone, r)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			glog.V(2).Infof("Unable to push: %v", err)
			return fmt.Errorf("a push has already been requested for this tag, use --force to push anyway")
		}
		return fmt.Errorf("failed to push binary: %v", err)
	}
	obj, err := kapi.Scheme.ConvertToVersion(result, imageapi.SchemeGroupVersion)
	if err != nil {
		return fmt.Errorf("unable to convert: %v", err)
	}
	info, err := o.Mapper.InfoForObject(obj, nil)
	if err != nil {
		return err
	}
	return o.PrintObject([]*resource.Info{info})
}

func contentsForPathOrURL(s string, in io.Reader, subpaths ...string) (string, io.Reader, int64, error) {
	switch {
	case s == "-":
		return "", in, -1, nil
	case strings.Index(s, "http://") == 0 || strings.Index(s, "https://") == 0:
		_, err := url.Parse(s)
		if err != nil {
			return "", nil, 0, fmt.Errorf("the URL passed to filename %q is not valid: %v", s, err)
		}
		res, err := http.Get(s)
		if err != nil {
			return "", nil, 0, err
		}
		return "", res.Body, -1, nil
	default:
		stat, err := os.Stat(s)
		if err != nil {
			return "", nil, 0, err
		}
		if !stat.IsDir() {
			contents, err := os.Open(s)
			return s, contents, stat.Size(), err
		}
		for _, sub := range subpaths {
			path := filepath.Join(s, sub)
			stat, err := os.Stat(path)
			if err != nil {
				continue
			}
			if stat.IsDir() {
				continue
			}
			contents, err := os.Open(path)
			return path, contents, stat.Size(), err
		}
		return s, nil, 0, os.ErrNotExist
	}
}

func convertStreamToTarGz(path string, r io.Reader, size int64) io.Reader {
	br := bufio.NewReaderSize(r, 2048)
	buf, err := br.Peek(2048)
	if err != nil && err != io.EOF {
		glog.V(4).Infof("Got error peeking: %v", err)
		return errReader{err: err}
	}
	switch contentType := http.DetectContentType(buf); contentType {
	case "application/x-gzip":
		gr, err := gzip.NewReader(bytes.NewBuffer(buf))
		if err != nil {
			glog.V(4).Infof("Not a tar.gz file: %v", err)
			return errReader{err: fmt.Errorf("%s does not appear to be a valid tar.gz file: %v", path, err)}
		}
		tr := tar.NewReader(gr)
		_, err = tr.Next()
		if err != nil && err != io.EOF {
			glog.V(4).Infof("Not a tar.gz file: %v", err)
			return errReader{err: fmt.Errorf("%s does not appear to be a valid tar.gz file: %v", path, err)}
		}
		glog.V(4).Infof("Detected input stream as tar.gz")
		return br
	case "application/zip":
		if size == -1 {
			glog.V(4).Infof("Not a valid zip file: %v", err)
			return errReader{err: fmt.Errorf("zip files are only supported from the filesystem", path)}
		}
		rAt, ok := r.(io.ReaderAt)
		if !ok {
			glog.V(4).Infof("Not a valid zip file: %v", err)
			return errReader{err: fmt.Errorf("zip files are only supported from the filesystem", path)}
		}
		zr, err := zip.NewReader(rAt, size)
		if err != nil {
			glog.V(4).Infof("Not a valid zip file: %v", err)
			return errReader{err: err}
		}
		pr, pw := io.Pipe()
		gw := gzip.NewWriter(pw)
		tw := tar.NewWriter(gw)
		go func() {
			if err := zipToTarGz(zr, tw); err != nil {
				glog.V(4).Infof("Failed to write to tar.gz: %v", err)
				pw.CloseWithError(err)
				return
			}
			if err := tw.Close(); err != nil {
				glog.V(4).Infof("Failed to close tar.gz: %v", err)
				pw.CloseWithError(err)
				return
			}
			gw.Close()
			pw.Close()
		}()
		glog.V(4).Infof("Detected input stream as zip")
		return pr
	default:
		tr := tar.NewReader(br)
		if _, err := tr.Next(); err == nil {
			glog.V(4).Infof("Detected input stream as tar")
			pr, pw := io.Pipe()
			gw := gzip.NewWriter(pw)
			go func() {
				if _, err := io.Copy(gw, br); err != nil {
					pw.CloseWithError(err)
				}
				gw.Close()
				pw.Close()
			}()
			return pr
		}

		glog.V(4).Infof("Unrecognized content type for contents: %s", contentType)
		return errReader{err: fmt.Errorf("%s does not appear to be a zip or tar.gz file (%s)", path, contentType)}
	}
}

// zipToTarGz copies the files in a zip.Reader into a tar, or returns an error.
func zipToTarGz(zr *zip.Reader, tw *tar.Writer) error {
	for _, file := range zr.File {
		glog.V(5).Infof("Writing zip file %s", file.Name)
		fi := file.FileInfo()
		h, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		h.Name = file.Name
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		fr, err := file.Open()
		if err != nil {
			return err
		}
		if _, err := io.Copy(tw, fr); err != nil {
			return err
		}
	}
	return nil
}

// errReader only returns the provided error.
type errReader struct {
	err error
}

func (r errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
