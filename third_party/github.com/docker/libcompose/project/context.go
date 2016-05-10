package project

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/golang/glog"
)

var projectRegexp = regexp.MustCompile("[^a-zA-Z0-9_.-]")

// Context holds context meta information about a libcompose project, like
// the project name, the compose file, etc.
type Context struct {
	Timeout             uint
	Log                 bool
	Volume              bool
	ForceRecreate       bool
	NoRecreate          bool
	NoCache             bool
	NoBuild             bool
	Signal              string
	ComposeFiles        []string
	ComposeBytes        [][]byte
	ProjectName         string
	isOpen              bool
	ServiceFactory      ServiceFactory
	EnvironmentLookup   EnvironmentLookup
	ResourceLookup      ResourceLookup
	IgnoreMissingConfig bool
	Project             *Project
}

func (c *Context) readComposeFiles() error {
	if c.ComposeBytes != nil {
		return nil
	}

	log.V(4).Infof("Opening compose files: %s", strings.Join(c.ComposeFiles, ","))

	// Handle STDIN (`-f -`)
	if len(c.ComposeFiles) == 1 && c.ComposeFiles[0] == "-" {
		composeBytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Errorf("Failed to read compose file from stdin: %v", err)
			return err
		}
		c.ComposeBytes = [][]byte{composeBytes}
		return nil
	}

	for _, composeFile := range c.ComposeFiles {
		composeBytes, err := ioutil.ReadFile(composeFile)
		if err != nil && !os.IsNotExist(err) {
			log.Errorf("Failed to open the compose file: %s", composeFile)
			return err
		}
		if err != nil && !c.IgnoreMissingConfig {
			log.Errorf("Failed to find the compose file: %s", composeFile)
			return err
		}
		c.ComposeBytes = append(c.ComposeBytes, composeBytes)
	}

	return nil
}

func (c *Context) determineProject() error {
	name, err := c.lookupProjectName()
	if err != nil {
		return err
	}

	c.ProjectName = projectRegexp.ReplaceAllString(strings.ToLower(name), "-")

	if c.ProjectName == "" {
		return fmt.Errorf("Falied to determine project name")
	}

	if strings.ContainsAny(c.ProjectName[0:1], "_.-") {
		c.ProjectName = "x" + c.ProjectName
	}

	return nil
}

func (c *Context) lookupProjectName() (string, error) {
	if c.ProjectName != "" {
		return c.ProjectName, nil
	}

	if envProject := os.Getenv("COMPOSE_PROJECT_NAME"); envProject != "" {
		return envProject, nil
	}

	file := "."
	if len(c.ComposeFiles) > 0 {
		file = c.ComposeFiles[0]
	}

	f, err := filepath.Abs(file)
	if err != nil {
		log.Errorf("Failed to get absolute directory for: %s", file)
		return "", err
	}

	f = toUnixPath(f)

	parent := path.Base(path.Dir(f))
	if parent != "" && parent != "." {
		return parent, nil
	} else if wd, err := os.Getwd(); err != nil {
		return "", err
	} else {
		return path.Base(toUnixPath(wd)), nil
	}
}

func toUnixPath(p string) string {
	return strings.Replace(p, "\\", "/", -1)
}

func (c *Context) open() error {
	if c.isOpen {
		return nil
	}

	if err := c.readComposeFiles(); err != nil {
		return err
	}

	if err := c.determineProject(); err != nil {
		return err
	}

	c.isOpen = true
	return nil
}
