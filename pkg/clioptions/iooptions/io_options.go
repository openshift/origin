package iooptions

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type StreamSetter interface {
	SetIOStreams(streams genericclioptions.IOStreams)
}

type CloseFunc func() error

type OutputFlags struct {
	OutFile string
}

func NewOutputOptions() *OutputFlags {
	return &OutputFlags{}
}

func (o *OutputFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.OutFile, "output-file", "o", o.OutFile, "Write all test output to this file.")
}

func doNothing() error {
	return nil
}

func (o *OutputFlags) ConfigureIOStreams(streams genericclioptions.IOStreams, streamSetter StreamSetter) (CloseFunc, error) {
	if len(o.OutFile) == 0 {
		streamSetter.SetIOStreams(streams)
		return doNothing, nil
	}

	dir := path.Dir(o.OutFile)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return doNothing, fmt.Errorf("failed to create parentdir %q: %w", dir, err)
	}

	f, err := os.OpenFile(o.OutFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return doNothing, err
	}

	wrappedStreams := genericclioptions.IOStreams{
		In: streams.In,
	}
	wrappedStreams.Out = io.MultiWriter(streams.Out, f)
	wrappedStreams.ErrOut = io.MultiWriter(streams.ErrOut, f)

	streamSetter.SetIOStreams(wrappedStreams)
	return f.Close, nil
}
