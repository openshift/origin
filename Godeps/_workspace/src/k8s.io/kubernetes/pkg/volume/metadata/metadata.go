/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metadata

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fieldpath"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"
	utilErrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume"
	"github.com/golang/glog"
)

// ProbeVolumePlugins is the entry point for plugin detection in a package.
func ProbeVolumePlugins() []volume.VolumePlugin {
	return []volume.VolumePlugin{&metadataPlugin{}}
}

const (
	metadataPluginName = "kubernetes.io/metadata"
)

// metadataPlugin implements the VolumePlugin interface.
type metadataPlugin struct {
	host volume.VolumeHost
}

func (plugin *metadataPlugin) Init(host volume.VolumeHost) {
	plugin.host = host
}

func (plugin *metadataPlugin) Name() string {
	return metadataPluginName
}

func (plugin *metadataPlugin) CanSupport(spec *volume.Spec) bool {
	return spec.VolumeSource.Metadata != nil
}

func (plugin *metadataPlugin) NewBuilder(spec *volume.Spec, pod *api.Pod, opts volume.VolumeOptions, mounter mount.Interface) (volume.Builder, error) {
	return plugin.newBuilderInternal(spec, pod, opts, mounter)
}

func (plugin *metadataPlugin) newBuilderInternal(spec *volume.Spec, pod *api.Pod, opts volume.VolumeOptions, mounter mount.Interface) (volume.Builder, error) {
	v := &metadataVolume{volName: spec.Name,
		pod:     pod,
		plugin:  plugin,
		opts:    &opts,
		mounter: mounter}
	v.fieldReferenceFileNames = make(map[string]string)
	for _, fileInfo := range spec.VolumeSource.Metadata.Items {
		v.fieldReferenceFileNames[fileInfo.FieldRef.FieldPath] = fileInfo.Name
	}
	return v, nil
}

func (plugin *metadataPlugin) NewCleaner(volName string, podUID types.UID, mounter mount.Interface) (volume.Cleaner, error) {
	return plugin.newCleanerInternal(volName, podUID, mounter)
}

func (plugin *metadataPlugin) newCleanerInternal(volName string, podUID types.UID, mounter mount.Interface) (volume.Cleaner, error) {
	return &metadataVolume{volName: volName,
		pod:     &api.Pod{ObjectMeta: api.ObjectMeta{UID: podUID}},
		plugin:  plugin,
		mounter: mounter}, nil
}

// metadataVolume handles retrieving metadata from the API server
// and placing them into the volume on the host.
type metadataVolume struct {
	volName                 string
	fieldReferenceFileNames map[string]string
	pod                     *api.Pod
	plugin                  *metadataPlugin
	opts                    *volume.VolumeOptions
	mounter                 mount.Interface
}

// This is the spec for the volume that this plugin wraps.
var wrappedVolumeSpec = &volume.Spec{
	Name:         "not-used",
	VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: api.StorageMediumMemory}},
}

// IsReadOnly exposes if the volume is read only.
// TODO: is this ok to always return true?
func (m *metadataVolume) IsReadOnly() bool {
	return true
}

// SetUp puts in place the volume plugin.
// This function is not idempotent by design. We want the data to be refreshed periodically.
// The internal sync interval of kubelet will drive the refresh of data.
func (m *metadataVolume) SetUp() error {
	return m.SetUpAt(m.GetPath())
}

func (m *metadataVolume) SetUpAt(dir string) error {
	glog.V(3).Infof("Setting up a metadata volume %v for pod %v at %v", m.volName, m.pod.UID, dir)
	// Wrap EmptyDir,let it do the setup.
	wrapped, err := m.plugin.host.NewWrapperBuilder(wrappedVolumeSpec, m.pod, *m.opts, m.mounter)
	if err != nil {
		return err
	}
	if err := wrapped.SetUpAt(dir); err != nil {
		return err
	}

	data := make(map[string]string)
	if err := m.collectData(&data); err != nil {
		return err
	}

	// files are fully regenerated only if requested metadata changed
	if m.isDataChanged(&data) {
		err = m.writeData(&data)
	}
	return err
}

// collectData collects requested metadata in data map.
// Map's key is the requested name of file to dump
// Map's value is the (sorted) content of the field to be dumped in the file.
func (m *metadataVolume) collectData(data *map[string]string) error {
	errlist := []error{}
	for fieldReference, fileName := range m.fieldReferenceFileNames {
		if values, err := fieldpath.ExtractFieldPathAsString(m.pod, fieldReference); err != nil {
			glog.Error(err)
			errlist = append(errlist, err)
		} else {
			(*data)[fileName] = sortLines(values)
		}
	}
	return utilErrors.NewAggregate(errlist)
}

// isDataChanged iterate over all the entries to check wether at least one
// file needs to be updated.
func (m *metadataVolume) isDataChanged(data *map[string]string) bool {
	for fileName, values := range *data {
		if isFileToGenerate(path.Join(m.GetPath(), fileName), values) {
			return true
		}
	}
	return false
}

// isFileToGenerate compares actual file with the new values. If
// different (or the file does not exist) return true
func isFileToGenerate(fileName, values string) bool {
	if _, err := os.Lstat(fileName); os.IsNotExist(err) {
		return true
	}
	return readFile(fileName) != values
}

const currentDir = ".current"
const currentTmpDir = ".current_tmp"

// writeData writes requested metadata in specified files.
//
// The file visible in this volume are symlinks to files in the '.current'
// directory. Actual files are stored in an hidden timestamped directory which is
// symlinked to by '.current'. The timestamped directory and '.current' symlink
// are created in the plugin root dir.  This scheme allows the files to be
// atomically updated by changing the target of the '.current' symlink.  When new
// data is available:
//
// 1.  A new timestamped dir is created by writeDataInTimestampDir
// 2.  Symlinks for new files are created if needed
// 3.  The previous timestamped directory is detected reading the '.current' symlink
// 4.  In case no symlink exists then it's created and func returns (first execution)
// 5.  In case symlink exists a new temporary symlink is created .current_tmp
// 6.  .current_tmp is renamed to .current
// 7.  The previous timestamped directory is removed
func (m *metadataVolume) writeData(data *map[string]string) error {
	var timestampDir string
	var err error
	timestampDir, err = m.writeDataInTimestampDir(data)
	if err != nil {
		glog.Error(err)
		return err
	}
	// update symbolic links for relative paths
	if err = m.updateSymlinksToCurrentDir(); err != nil {
		os.RemoveAll(timestampDir)
		glog.Error(err)
		return err
	}

	_, timestampDirBaseName := filepath.Split(timestampDir)
	var oldTimestampDirectory string
	oldTimestampDirectory, err = os.Readlink(path.Join(m.GetPath(), currentDir))
	if err != nil {
		if os.IsNotExist(err) { // no link to currentDir so creates it
			err = os.Symlink(timestampDirBaseName, path.Join(m.GetPath(), currentDir))
		}
		return err // in any cases return
	}

	// nominal case: link to currentDir exists create the link to currentTmpDir
	if err = os.Symlink(timestampDirBaseName, path.Join(m.GetPath(), currentTmpDir)); err != nil {
		os.RemoveAll(timestampDir)
		glog.Error(err)
		return err
	}

	// Rename the symbolic link currentTmpDir to currentDir
	if err = os.Rename(path.Join(m.GetPath(), currentTmpDir), path.Join(m.GetPath(), currentDir)); err != nil {
		// in case of error remove latest data and currentTmpDir
		os.Remove(path.Join(m.GetPath(), currentTmpDir))
		os.RemoveAll(timestampDir)
		glog.Error(err)
		return err
	}
	// Remove oldTimestampDirectory
	if len(oldTimestampDirectory) > 0 {
		if e := os.RemoveAll(path.Join(m.GetPath(), oldTimestampDirectory)); e != nil {
			glog.Error(e)
			return e
		}
	}
	return nil
}

// writeDataInTimestampDir writes the latest data into a new temporary directory with a timestamp.
func (m *metadataVolume) writeDataInTimestampDir(data *map[string]string) (string, error) {
	errlist := []error{}
	timestampDir, err := ioutil.TempDir(m.GetPath(), "."+time.Now().Format("2006_01_02_15_04_05"))
	for fileName, values := range *data {
		fullPathFile := path.Join(timestampDir, fileName)
		dir, _ := filepath.Split(fullPathFile)
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return "", err
		}
		if e := ioutil.WriteFile(fullPathFile, []byte(values), 0644); e != nil {
			glog.Error(e)
			errlist = append(errlist, err)
		}
	}
	return timestampDir, utilErrors.NewAggregate(errlist)
}

// updateSymlinksToCurrentDir creates the relative symlinks for all the files configured in this volume.
// If the directory in a file path does not exist, it is created.
func (m *metadataVolume) updateSymlinksToCurrentDir() error {
	for _, f := range m.fieldReferenceFileNames {
		dir, _ := filepath.Split(f)
		nbOfSubdir := 0
		if len(dir) > 0 {
			nbOfSubdir = len(strings.Split(dir, "/")) - 1
			if e := os.MkdirAll(path.Join(m.GetPath(), dir), os.ModePerm); e != nil {
				return e
			}
		}
		if _, e := os.Readlink(path.Join(m.GetPath(), f)); e != nil {
			// link does not exist create it
			if e := os.Symlink(path.Join(strings.Repeat("../", nbOfSubdir)+currentDir, f), path.Join(m.GetPath(), f)); e != nil {
				return e
			}
		}
	}
	return nil
}

// readFile reads the file at the given path and returns the content as a string.
func readFile(path string) string {
	if data, err := ioutil.ReadFile(path); err == nil {
		return string(data)
	}
	return ""
}

// sortLines sorts the strings generated from map based metadata
// (annotations and labels)
func sortLines(values string) string {
	splitted := strings.Split(values, "\n")
	sort.Strings(splitted)
	return strings.Join(splitted, "\n")
}

func (m *metadataVolume) GetPath() string {
	return m.plugin.host.GetPodVolumeDir(m.pod.UID, util.EscapeQualifiedNameForDisk(metadataPluginName), m.volName)
}

func (m *metadataVolume) TearDown() error {
	return m.TearDownAt(m.GetPath())
}

func (m *metadataVolume) TearDownAt(dir string) error {
	glog.V(3).Infof("Tearing down volume %v for pod %v at %v", m.volName, m.pod.UID, dir)

	// Wrap EmptyDir, let it do the teardown.
	wrapped, err := m.plugin.host.NewWrapperCleaner(wrappedVolumeSpec, m.pod.UID, m.mounter)
	if err != nil {
		return err
	}
	return wrapped.TearDownAt(dir)
}

func (m *metadataVolume) getMetaDir() string {
	return path.Join(m.plugin.host.GetPodPluginDir(m.pod.UID, util.EscapeQualifiedNameForDisk(metadataPluginName)), m.volName)
}
