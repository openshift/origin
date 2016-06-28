package main

import (
	"fmt"
	"io"
	"os"
	"runtime"

	kruntime "k8s.io/kubernetes/pkg/runtime"

	flag "github.com/spf13/pflag"
)

var (
	// inputFile is the file from which source types are read
	inputFile string

	// outputFile is the file to which generated maps and functions will be written
	outputFile string

	// verify determines if a report is generated about types missing documentation
	verify bool
)

const (
	defaultInputFile  = "/dev/stdin"
	defaultOutputFile = "/dev/stdout"
	defaultVerify     = false
)

func init() {
	flag.StringVar(&inputFile, "input", defaultInputFile, "Go source code containing types to be documented")
	flag.StringVar(&outputFile, "output", defaultOutputFile, "file to which generated Go code should be written")
	flag.BoolVar(&verify, "verify", defaultVerify, "verify that types being documented are not missing any comments, write no output")
}

const (
	genSwaggerDocLong = `Generate functions that allow types to describe themselves for Swagger.

%s consumes a Go source file containing types supporting API endpoints that Swagger describes,
and generates 'SwaggerDoc()' methods for every object so that they can describe themselves when Swagger
spec is generated. Godoc for types and fields is exposed as the documentation through the 'SwaggerDoc()'
function. Comment lines beginning with '---' or 'TOOD' are ignored.
`

	genSwaggerDocUsage = `Usage:
  %s [--input=GO-FILE] [--output=GENERATED-FILE] [--verify]
`

	genSwaggerDocExamples = `Examples:
  # Generate 'SwaggerDoc' methods to file 'swagger_doc_generated.go' for objects in file 'types.go'
  %[1]s --input=types.go --output=swagger_doc_generated.go

  # Verify that types in 'types.go' are sufficiently docummented
  %[1]s --input=types.go --verify=true
`
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, genSwaggerDocLong+"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, genSwaggerDocUsage+"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, genSwaggerDocExamples+"\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		os.Exit(2)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()

	var output io.Writer
	if outputFile == defaultOutputFile {
		output = os.Stdout
	} else {
		file, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading output file: %v\n", err)
		}
		defer file.Close()
		output = file
	}

	documentationForTypes := kruntime.ParseDocumentationFrom(inputFile)

	if verify {
		if numMissingDocs, err := kruntime.VerifySwaggerDocsExist(documentationForTypes, output); err != nil {
			fmt.Fprintf(os.Stderr, "Error verifying documentation: %v\n", err)
			os.Exit(1)
		} else if numMissingDocs != 0 {
			fmt.Fprintf(os.Stderr, "Found %d types or fields missing documentation in %q.\n", numMissingDocs, inputFile)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "Found no types or fields missing documentation in %q.\n", inputFile)
		os.Exit(0)
	}

	if err := kruntime.WriteSwaggerDocFunc(documentationForTypes, output); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing generated Swagger documentation to file: %v\n", err)
		os.Exit(1)
	}
}
