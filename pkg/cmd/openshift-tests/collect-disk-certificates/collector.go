package collectdiskcertificates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphanalysis"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/templates"
)

var inspectDirs = []string{"/etc/kubernetes", "/var/lib/ovn-ic/etc", "/var/lib/openvswitch/pki"}

type RunCollectDiskCertificatesFlags struct {
	ConfigFlags *genericclioptions.ConfigFlags
	OutputFlags *iooptions.OutputFlags

	ArtifactDir string

	genericclioptions.IOStreams
}

func NewRunCollectDiskCertificatesFlags(ioStreams genericclioptions.IOStreams) *RunCollectDiskCertificatesFlags {
	return &RunCollectDiskCertificatesFlags{
		ConfigFlags: genericclioptions.NewConfigFlags(false),
		OutputFlags: iooptions.NewOutputOptions(),
		IOStreams:   ioStreams,
	}
}

func NewRunCollectDiskCertificatesCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	f := NewRunCollectDiskCertificatesFlags(ioStreams)
	cmd := &cobra.Command{
		Use:   "collect-disk-certificates",
		Short: "Run disk certificate collector",
		Long: templates.LongDesc(`
		Run a colelctor which fetches information about certificates stored on disk
		`),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancelFn := context.WithCancel(context.Background())
			defer cancelFn()
			abortCh := make(chan os.Signal, 2)
			go func() {
				<-abortCh
				fmt.Fprintf(f.ErrOut, "Interrupted, terminating\n")
				cancelFn()

				sig := <-abortCh
				fmt.Fprintf(f.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
				switch sig {
				case syscall.SIGINT:
					os.Exit(130)
				default:
					os.Exit(0)
				}
			}()
			signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

			if err := f.Validate(); err != nil {
				return err
			}

			o, err := f.ToOptions()
			if err != nil {
				return err
			}

			return o.Run(ctx)
		},
	}

	f.AddFlags(cmd.Flags())

	return cmd
}

func (f *RunCollectDiskCertificatesFlags) AddFlags(flags *pflag.FlagSet) {
	f.ConfigFlags.AddFlags(flags)
	f.OutputFlags.BindFlags(flags)
}

func (f *RunCollectDiskCertificatesFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
}

func (f *RunCollectDiskCertificatesFlags) Validate() error {
	if len(f.OutputFlags.OutFile) == 0 {
		return fmt.Errorf("output-file must be specified")
	}

	return nil
}

func (f *RunCollectDiskCertificatesFlags) ToOptions() (*RunCollectDiskCertificatesOptions, error) {
	originalOutStream := f.IOStreams.Out
	closeFn, err := f.OutputFlags.ConfigureIOStreams(f.IOStreams, f)
	if err != nil {
		return nil, err
	}

	restConfig, err := f.ConfigFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &RunCollectDiskCertificatesOptions{
		KubeClient:       kubeClient,
		KubeClientConfig: restConfig,
		OutputFile:       f.OutputFlags.OutFile,
		CloseFn:          closeFn,
		OriginalOutFile:  originalOutStream,
		IOStreams:        f.IOStreams,
	}, nil
}

// RunCollectDiskCertificatesOptions sets options for api server disruption monitor
type RunCollectDiskCertificatesOptions struct {
	KubeClient       kubernetes.Interface
	KubeClientConfig *rest.Config
	OutputFile       string

	OriginalOutFile io.Writer
	CloseFn         iooptions.CloseFunc
	genericclioptions.IOStreams
}

func (o *RunCollectDiskCertificatesOptions) Run(ctx context.Context) error {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	fmt.Fprintf(o.Out, "Starting up.\n")

	pkiList := &certgraphapi.PKIList{}
	errs := []error{}
	for _, srcDir := range inspectDirs {
		dirPKIList, err := certgraphanalysis.GatherCertsFromDisk(ctx, o.KubeClient, srcDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %s", srcDir, err))
		}
		pkiList = certgraphanalysis.MergePKILists(ctx, pkiList, dirPKIList)
	}
	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}
	bytes, err := json.MarshalIndent(pkiList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to unmarshal pkilist: %s", err)
	}
	_, err = o.OriginalOutFile.Write(bytes)
	if err != nil {
		return fmt.Errorf("failed to write to output file: %s", err)
	}

	fmt.Fprintf(o.Out, "certificates collected")

	return nil
}
