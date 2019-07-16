package create

import (
	"os"
	"text/template"

	"github.com/openshift/source-to-image/pkg/create/templates"
	utillog "github.com/openshift/source-to-image/pkg/util/log"
)

var log = utillog.StderrLog

// Bootstrap defines parameters for the template processing
type Bootstrap struct {
	DestinationDir string
	ImageName      string
}

// New returns a new bootstrap for given image name and destination directory
func New(name, dst string) *Bootstrap {
	return &Bootstrap{ImageName: name, DestinationDir: dst}
}

// AddSTIScripts creates the STI scripts directory structure and process
// templates for STI scripts
func (b *Bootstrap) AddSTIScripts() {
	os.MkdirAll(b.DestinationDir+"/"+"s2i/bin", 0700)
	b.process(templates.AssembleScript, "s2i/bin/assemble", 0755)
	b.process(templates.RunScript, "s2i/bin/run", 0755)
	b.process(templates.UsageScript, "s2i/bin/usage", 0755)
	b.process(templates.SaveArtifactsScript, "s2i/bin/save-artifacts", 0755)
}

// AddDockerfile creates an example Dockerfile
func (b *Bootstrap) AddDockerfile() {
	b.process(templates.Dockerfile, "Dockerfile", 0600)
}

// AddReadme creates a README.md
func (b *Bootstrap) AddReadme() {
	b.process(templates.Readme, "README.md", 0600)
}

// AddTests creates an example test for the STI image and the Makefile
func (b *Bootstrap) AddTests() {
	os.MkdirAll(b.DestinationDir+"/"+"test/test-app", 0700)
	b.process(templates.Index, "test/test-app/index.html", 0600)
	b.process(templates.TestRunScript, "test/run", 0700)
	b.process(templates.Makefile, "Makefile", 0600)
}

func (b *Bootstrap) process(t string, dst string, perm os.FileMode) {
	tpl := template.Must(template.New("").Parse(t))
	if _, err := os.Stat(b.DestinationDir + "/" + dst); err == nil {
		log.Errorf("File already exists: %s, skipping", dst)
		return
	}
	f, err := os.Create(b.DestinationDir + "/" + dst)
	if err != nil {
		log.Errorf("Unable to create %s file, skipping: %v", dst, err)
		return
	}
	if err := os.Chmod(b.DestinationDir+"/"+dst, perm); err != nil {
		log.Errorf("Unable to chmod %s file to %v, skipping: %v", dst, perm, err)
		return
	}
	defer f.Close()
	if err := tpl.Execute(f, b); err != nil {
		log.Errorf("Error processing %s template: %v", dst, err)
	}
}
