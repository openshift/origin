package printers

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// we have this here so that we can provide the human readable printer since this bypasses the factory.

type NewHumanReadablePrinterFunc func(encoder runtime.Encoder, decoder runtime.Decoder, options PrintOptions) *HumanReadablePrinter

var NewHumanReadablePrinterFn NewHumanReadablePrinterFunc = NewHumanReadablePrinter
