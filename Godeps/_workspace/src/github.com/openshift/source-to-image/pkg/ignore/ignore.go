package ignore

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
)

type DockerIgnorer struct{}

func (b *DockerIgnorer) Ignore(config *api.Config) error {
	/*
		 so, to duplicate the .dockerignore capabilities (https://docs.docker.com/reference/builder/#dockerignore-file)
		 we have a flow that follows:
		0) First note, .dockerignore rules are NOT recursive (unlike .gitignore) .. you have to list subdir explicitly
		1) Read in the exclusion patterns
		2) Skip over comments (noted by #)
		3) note overrides (via exclamation sign i.e. !) and reinstate files (don't remove) as needed
		4) leverage Glob matching to build list, as .dockerignore is documented as following filepath.Match / filepath.Glob
		5) del files
		 1 to 4 is in getListOfFilesToIgnore
	*/
	filesToDel, lerr := getListOfFilesToIgnore(config)
	if lerr != nil {
		return lerr
	}

	if filesToDel == nil {
		return nil
	}

	// delete compiled list of files
	for _, fileToDel := range filesToDel {
		glog.V(5).Infof("attempting to remove file %s \n", fileToDel)
		rerr := os.RemoveAll(fileToDel)
		if rerr != nil {
			glog.Errorf("error removing file %s because of %v \n", fileToDel, rerr)
			return rerr
		}
	}

	return nil
}

func getListOfFilesToIgnore(config *api.Config) (map[string]string, error) {
	path := filepath.Join(config.WorkingSourceDir, api.IgnoreFile)
	file, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			glog.Errorf("Ignore processing, problem opening %s because of %v\n", path, err)
			return nil, err
		}
		glog.V(4).Info(".s2iignore file does not exist")
		return nil, nil
	}
	defer file.Close()

	filesToDel := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		filespec := strings.Trim(scanner.Text(), " ")

		if strings.HasPrefix(filespec, "#") {
			continue
		}

		glog.V(4).Infof(".s2iignore lists a file spec of %s \n", filespec)

		if strings.HasPrefix(filespec, "!") {
			//remove any existing files to del that the override covers
			// and patterns later on that undo this take precedence

			// first, remove ! ... note, replace ! with */ did not have
			// expected effect with filepath.Match
			filespec = strings.Replace(filespec, "!", "", 1)

			// iterate through and determine ones to leave in
			dontDel := make([]string, 0)
			for candidate := range filesToDel {
				compare := filepath.Join(config.WorkingSourceDir, filespec)
				glog.V(5).Infof("For %s  and %s see if it matches the spec  %s which means that we leave in\n", filespec, candidate, compare)
				leaveIn, _ := filepath.Match(compare, candidate)
				if leaveIn {
					glog.V(5).Infof("Not removing %s \n", candidate)
					dontDel = append(dontDel, candidate)
				} else {
					glog.V(5).Infof("No match for %s and %s \n", filespec, candidate)
				}
			}

			// now remove any matches from files to delete list
			for _, leaveIn := range dontDel {
				delete(filesToDel, leaveIn)
			}
			continue
		}

		globspec := filepath.Join(config.WorkingSourceDir, filespec)
		glog.V(4).Infof("using globspec %s \n", globspec)
		list, gerr := filepath.Glob(globspec)
		if gerr != nil {
			glog.V(4).Infof("Glob failed with %v \n", gerr)
		} else {
			for _, globresult := range list {
				glog.V(5).Infof("Glob result %s \n", globresult)
				filesToDel[globresult] = globresult

			}
		}

	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		glog.Errorf("Problem processing .s2iignore %v \n", err)
		return nil, err
	}

	return filesToDel, nil
}
