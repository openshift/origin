package sti

import (
	"os"
	"text/template"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/sti/create/templates"
)

// Bootstrap defines parameters for the template processing
type Bootstrap struct {
	DestinationDir string
	ImageName      string
}

// NewCreate returns a new bootstrap for giben image name and destination
// directory
func NewCreate(name, dst string) *Bootstrap {
	return &Bootstrap{ImageName: name, DestinationDir: dst}
}

// AddSTIScripts creates the STI scripts directory structure and process
// templates for STI scripts
func (b *Bootstrap) AddSTIScripts() {
	os.MkdirAll(b.DestinationDir+"/"+".sti/bin", 0700)
	b.process(templates.AssembleScript, ".sti/bin/assemble")
	b.process(templates.RunScript, ".sti/bin/run")
	b.process(templates.UsageScript, ".sti/bin/usage")
	b.process(templates.SaveArtifactsScript, ".sti/bin/save-artifacts")
}

// AddDockerfile creates an example Dockerfile
func (b *Bootstrap) AddDockerfile() {
	b.process(templates.Dockerfile, "Dockerfile")
}

// AddTests creates an example test for the STI image and the Makefile
func (b *Bootstrap) AddTests() {
	os.MkdirAll(b.DestinationDir+"/"+"test/test-app", 0700)
	b.process(templates.TestRunScript, "test/run")
	b.process(templates.Makefile, "Makefile")
}

func (b *Bootstrap) process(t string, dst string) {
	tpl := template.Must(template.New("").Parse(t))
	if _, err := os.Stat(b.DestinationDir + "/" + dst); err == nil {
		glog.Errorf("File already exists: %s, skipping", dst)
		return
	}
	f, err := os.Create(b.DestinationDir + "/" + dst)
	if err != nil {
		glog.Errorf("Unable to create %s file, skipping: %v", dst, err)
		return
	}
	defer f.Close()
	if err := tpl.Execute(f, b); err != nil {
		glog.Errorf("Error processing %s template: %v", dst, err)
	}
}
