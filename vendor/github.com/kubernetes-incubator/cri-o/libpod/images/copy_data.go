package images

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/containers/image/docker/reference"
	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/kubernetes-incubator/cri-o/cmd/kpod/docker"
	"github.com/kubernetes-incubator/cri-o/libpod/common"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	// Package is used to identify working containers
	Package       = "kpod"
	containerType = Package + " 0.0.1"
	stateFile     = Package + ".json"
	// OCIv1ImageManifest is the MIME type of an OCIv1 image manifest,
	// suitable for specifying as a value of the PreferredManifestType
	// member of a CommitOptions structure.  It is also the default.
	OCIv1ImageManifest = v1.MediaTypeImageManifest
)

// CopyData stores the basic data used when copying a container or image
type CopyData struct {
	store storage.Store

	// Type is used to help identify a build container's metadata.  It
	// should not be modified.
	Type string `json:"type"`
	// FromImage is the name of the source image which was used to create
	// the container, if one was used.  It should not be modified.
	FromImage string `json:"image,omitempty"`
	// FromImageID is the ID of the source image which was used to create
	// the container, if one was used.  It should not be modified.
	FromImageID string `json:"image-id"`
	// Config is the source image's configuration.  It should not be
	// modified.
	Config []byte `json:"config,omitempty"`
	// Manifest is the source image's manifest.  It should not be modified.
	Manifest []byte `json:"manifest,omitempty"`

	// Container is the name of the build container.  It should not be modified.
	Container string `json:"container-name,omitempty"`
	// ContainerID is the ID of the build container.  It should not be modified.
	ContainerID string `json:"container-id,omitempty"`
	// MountPoint is the last location where the container's root
	// filesystem was mounted.  It should not be modified.
	MountPoint string `json:"mountpoint,omitempty"`

	// ImageAnnotations is a set of key-value pairs which is stored in the
	// image's manifest.
	ImageAnnotations map[string]string `json:"annotations,omitempty"`
	// ImageCreatedBy is a description of how this container was built.
	ImageCreatedBy string `json:"created-by,omitempty"`

	// Image metadata and runtime settings, in multiple formats.
	OCIv1  v1.Image       `json:"ociv1,omitempty"`
	Docker docker.V2Image `json:"docker,omitempty"`
}

func (c *CopyData) initConfig() {
	image := ociv1.Image{}
	dimage := docker.V2Image{}
	if len(c.Config) > 0 {
		// Try to parse the image config.  If we fail, try to start over from scratch
		if err := json.Unmarshal(c.Config, &dimage); err == nil && dimage.DockerVersion != "" {
			image, err = makeOCIv1Image(&dimage)
			if err != nil {
				image = ociv1.Image{}
			}
		} else {
			if err := json.Unmarshal(c.Config, &image); err != nil {
				if dimage, err = makeDockerV2S2Image(&image); err != nil {
					dimage = docker.V2Image{}
				}
			}
		}
		c.OCIv1 = image
		c.Docker = dimage
	} else {
		// Try to dig out the image configuration from the manifest
		manifest := docker.V2S1Manifest{}
		if err := json.Unmarshal(c.Manifest, &manifest); err == nil && manifest.SchemaVersion == 1 {
			if dimage, err = makeDockerV2S1Image(manifest); err == nil {
				if image, err = makeOCIv1Image(&dimage); err != nil {
					image = ociv1.Image{}
				}
			}
		}
		c.OCIv1 = image
		c.Docker = dimage
	}

	if len(c.Manifest) > 0 {
		// Attempt to recover format-specific data from the manifest
		v1Manifest := ociv1.Manifest{}
		if json.Unmarshal(c.Manifest, &v1Manifest) == nil {
			c.ImageAnnotations = v1Manifest.Annotations
		}
	}

	c.fixupConfig()
}

func (c *CopyData) fixupConfig() {
	if c.Docker.Config != nil {
		// Prefer image-level settings over those from the container it was built from
		c.Docker.ContainerConfig = *c.Docker.Config
	}
	c.Docker.Config = &c.Docker.ContainerConfig
	c.Docker.DockerVersion = ""
	now := time.Now().UTC()
	if c.Docker.Created.IsZero() {
		c.Docker.Created = now
	}
	if c.OCIv1.Created.IsZero() {
		c.OCIv1.Created = &now
	}
	if c.OS() == "" {
		c.SetOS(runtime.GOOS)
	}
	if c.Architecture() == "" {
		c.SetArchitecture(runtime.GOARCH)
	}
	if c.WorkDir() == "" {
		c.SetWorkDir(string(filepath.Separator))
	}
}

// OS returns a name of the OS on which a container built using this image
//is intended to be run.
func (c *CopyData) OS() string {
	return c.OCIv1.OS
}

// SetOS sets the name of the OS on which a container built using this image
// is intended to be run.
func (c *CopyData) SetOS(os string) {
	c.OCIv1.OS = os
	c.Docker.OS = os
}

// Architecture returns a name of the architecture on which a container built
// using this image is intended to be run.
func (c *CopyData) Architecture() string {
	return c.OCIv1.Architecture
}

// SetArchitecture sets the name of the architecture on which ta container built
// using this image is intended to be run.
func (c *CopyData) SetArchitecture(arch string) {
	c.OCIv1.Architecture = arch
	c.Docker.Architecture = arch
}

// WorkDir returns the default working directory for running commands in a container
// built using this image.
func (c *CopyData) WorkDir() string {
	return c.OCIv1.Config.WorkingDir
}

// SetWorkDir sets the location of the default working directory for running commands
// in a container built using this image.
func (c *CopyData) SetWorkDir(there string) {
	c.OCIv1.Config.WorkingDir = there
	c.Docker.Config.WorkingDir = there
}

// makeOCIv1Image builds the best OCIv1 image structure we can from the
// contents of the docker image structure.
func makeOCIv1Image(dimage *docker.V2Image) (ociv1.Image, error) {
	config := dimage.Config
	if config == nil {
		config = &dimage.ContainerConfig
	}
	dimageCreatedTime := dimage.Created.UTC()
	image := ociv1.Image{
		Created:      &dimageCreatedTime,
		Author:       dimage.Author,
		Architecture: dimage.Architecture,
		OS:           dimage.OS,
		Config: ociv1.ImageConfig{
			User:         config.User,
			ExposedPorts: map[string]struct{}{},
			Env:          config.Env,
			Entrypoint:   config.Entrypoint,
			Cmd:          config.Cmd,
			Volumes:      config.Volumes,
			WorkingDir:   config.WorkingDir,
			Labels:       config.Labels,
		},
		RootFS: ociv1.RootFS{
			Type:    "",
			DiffIDs: []digest.Digest{},
		},
		History: []ociv1.History{},
	}
	for port, what := range config.ExposedPorts {
		image.Config.ExposedPorts[string(port)] = what
	}
	RootFS := docker.V2S2RootFS{}
	if dimage.RootFS != nil {
		RootFS = *dimage.RootFS
	}
	if RootFS.Type == docker.TypeLayers {
		image.RootFS.Type = docker.TypeLayers
		for _, id := range RootFS.DiffIDs {
			image.RootFS.DiffIDs = append(image.RootFS.DiffIDs, digest.Digest(id.String()))
		}
	}
	for _, history := range dimage.History {
		historyCreatedTime := history.Created.UTC()
		ohistory := ociv1.History{
			Created:    &historyCreatedTime,
			CreatedBy:  history.CreatedBy,
			Author:     history.Author,
			Comment:    history.Comment,
			EmptyLayer: history.EmptyLayer,
		}
		image.History = append(image.History, ohistory)
	}
	return image, nil
}

// makeDockerV2S2Image builds the best docker image structure we can from the
// contents of the OCI image structure.
func makeDockerV2S2Image(oimage *ociv1.Image) (docker.V2Image, error) {
	image := docker.V2Image{
		V1Image: docker.V1Image{Created: oimage.Created.UTC(),
			Author:       oimage.Author,
			Architecture: oimage.Architecture,
			OS:           oimage.OS,
			ContainerConfig: docker.Config{
				User:         oimage.Config.User,
				ExposedPorts: docker.PortSet{},
				Env:          oimage.Config.Env,
				Entrypoint:   oimage.Config.Entrypoint,
				Cmd:          oimage.Config.Cmd,
				Volumes:      oimage.Config.Volumes,
				WorkingDir:   oimage.Config.WorkingDir,
				Labels:       oimage.Config.Labels,
			},
		},
		RootFS: &docker.V2S2RootFS{
			Type:    "",
			DiffIDs: []digest.Digest{},
		},
		History: []docker.V2S2History{},
	}
	for port, what := range oimage.Config.ExposedPorts {
		image.ContainerConfig.ExposedPorts[docker.Port(port)] = what
	}
	if oimage.RootFS.Type == docker.TypeLayers {
		image.RootFS.Type = docker.TypeLayers
		for _, id := range oimage.RootFS.DiffIDs {
			d, err := digest.Parse(id.String())
			if err != nil {
				return docker.V2Image{}, err
			}
			image.RootFS.DiffIDs = append(image.RootFS.DiffIDs, d)
		}
	}
	for _, history := range oimage.History {
		dhistory := docker.V2S2History{
			Created:    history.Created.UTC(),
			CreatedBy:  history.CreatedBy,
			Author:     history.Author,
			Comment:    history.Comment,
			EmptyLayer: history.EmptyLayer,
		}
		image.History = append(image.History, dhistory)
	}
	image.Config = &image.ContainerConfig
	return image, nil
}

// makeDockerV2S1Image builds the best docker image structure we can from the
// contents of the V2S1 image structure.
func makeDockerV2S1Image(manifest docker.V2S1Manifest) (docker.V2Image, error) {
	// Treat the most recent (first) item in the history as a description of the image.
	if len(manifest.History) == 0 {
		return docker.V2Image{}, errors.Errorf("error parsing image configuration from manifest")
	}
	dimage := docker.V2Image{}
	err := json.Unmarshal([]byte(manifest.History[0].V1Compatibility), &dimage)
	if err != nil {
		return docker.V2Image{}, err
	}
	if dimage.DockerVersion == "" {
		return docker.V2Image{}, errors.Errorf("error parsing image configuration from history")
	}
	// The DiffID list is intended to contain the sums of _uncompressed_ blobs, and these are most
	// likely compressed, so leave the list empty to avoid potential confusion later on.  We can
	// construct a list with the correct values when we prep layers for pushing, so we don't lose.
	// information by leaving this part undone.
	rootFS := &docker.V2S2RootFS{
		Type:    docker.TypeLayers,
		DiffIDs: []digest.Digest{},
	}
	// Build a filesystem history.
	history := []docker.V2S2History{}
	for i := range manifest.History {
		h := docker.V2S2History{
			Created:    time.Now().UTC(),
			Author:     "",
			CreatedBy:  "",
			Comment:    "",
			EmptyLayer: false,
		}
		dcompat := docker.V1Compatibility{}
		if err2 := json.Unmarshal([]byte(manifest.History[i].V1Compatibility), &dcompat); err2 == nil {
			h.Created = dcompat.Created.UTC()
			h.Author = dcompat.Author
			h.Comment = dcompat.Comment
			if len(dcompat.ContainerConfig.Cmd) > 0 {
				h.CreatedBy = fmt.Sprintf("%v", dcompat.ContainerConfig.Cmd)
			}
			h.EmptyLayer = dcompat.ThrowAway
		}
		// Prepend this layer to the list, because a v2s1 format manifest's list is in reverse order
		// compared to v2s2, which lists earlier layers before later ones.
		history = append([]docker.V2S2History{h}, history...)
	}
	dimage.RootFS = rootFS
	dimage.History = history
	return dimage, nil
}

// Annotations gets the anotations of the container or image
func (c *CopyData) Annotations() map[string]string {
	return common.CopyStringStringMap(c.ImageAnnotations)
}

// Save the CopyData to disk
func (c *CopyData) Save() error {
	buildstate, err := json.Marshal(c)
	if err != nil {
		return err
	}
	cdir, err := c.store.ContainerDirectory(c.ContainerID)
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(filepath.Join(cdir, stateFile), buildstate, 0600)

}

// GetContainerCopyData gets the copy data for a container
func GetContainerCopyData(store storage.Store, name string) (*CopyData, error) {
	var data *CopyData
	var err error
	if name != "" {
		data, err = openCopyData(store, name)
		if os.IsNotExist(errors.Cause(err)) {
			data, err = importCopyData(store, name, "")
		}
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error reading build container")
	}
	if data == nil {
		return nil, errors.Errorf("error finding build container")
	}
	return data, nil

}

// GetImageCopyData gets the copy data for an image
func GetImageCopyData(store storage.Store, image string) (*CopyData, error) {
	if image == "" {
		return nil, errors.Errorf("image name must be specified")
	}
	img, err := FindImage(store, image)
	if err != nil {
		return nil, errors.Wrapf(err, "error locating image %q for importing settings", image)
	}

	systemContext := common.GetSystemContext("")
	data, err := ImportCopyDataFromImage(store, systemContext, img.ID, "", "")
	if err != nil {
		return nil, errors.Wrapf(err, "error reading image")
	}
	if data == nil {
		return nil, errors.Errorf("error mocking up build configuration")
	}
	return data, nil

}

func importCopyData(store storage.Store, container, signaturePolicyPath string) (*CopyData, error) {
	if container == "" {
		return nil, errors.Errorf("container name must be specified")
	}

	c, err := store.Container(container)
	if err != nil {
		return nil, err
	}

	systemContext := common.GetSystemContext(signaturePolicyPath)

	data, err := ImportCopyDataFromImage(store, systemContext, c.ImageID, container, c.ID)
	if err != nil {
		return nil, err
	}

	if data.FromImageID != "" {
		if d, err2 := digest.Parse(data.FromImageID); err2 == nil {
			data.Docker.Parent = docker.ID(d)
		} else {
			data.Docker.Parent = docker.ID(digest.NewDigestFromHex(digest.Canonical.String(), data.FromImageID))
		}
	}
	if data.FromImage != "" {
		data.Docker.ContainerConfig.Image = data.FromImage
	}

	err = data.Save()
	if err != nil {
		return nil, errors.Wrapf(err, "error saving CopyData state")
	}

	return data, nil
}

func openCopyData(store storage.Store, container string) (*CopyData, error) {
	cdir, err := store.ContainerDirectory(container)
	if err != nil {
		return nil, err
	}
	buildstate, err := ioutil.ReadFile(filepath.Join(cdir, stateFile))
	if err != nil {
		return nil, err
	}
	c := &CopyData{}
	err = json.Unmarshal(buildstate, &c)
	if err != nil {
		return nil, err
	}
	if c.Type != containerType {
		return nil, errors.Errorf("container is not a %s container", Package)
	}
	c.store = store
	c.fixupConfig()
	return c, nil

}

// ImportCopyDataFromImage creates copy data for an image with the given parameters
func ImportCopyDataFromImage(store storage.Store, systemContext *types.SystemContext, imageID, containerName, containerID string) (*CopyData, error) {
	manifest := []byte{}
	config := []byte{}
	imageName := ""

	if imageID != "" {
		ref, err := is.Transport.ParseStoreReference(store, "@"+imageID)
		if err != nil {
			return nil, errors.Wrapf(err, "no such image %q", "@"+imageID)
		}
		src, err2 := ref.NewImage(systemContext)
		if err2 != nil {
			return nil, errors.Wrapf(err2, "error instantiating image")
		}
		defer src.Close()
		config, err = src.ConfigBlob()
		if err != nil {
			return nil, errors.Wrapf(err, "error reading image configuration")
		}
		manifest, _, err = src.Manifest()
		if err != nil {
			return nil, errors.Wrapf(err, "error reading image manifest")
		}
		if img, err3 := store.Image(imageID); err3 == nil {
			if len(img.Names) > 0 {
				imageName = img.Names[0]
			}
		}
	}

	data := &CopyData{
		store:            store,
		Type:             containerType,
		FromImage:        imageName,
		FromImageID:      imageID,
		Config:           config,
		Manifest:         manifest,
		Container:        containerName,
		ContainerID:      containerID,
		ImageAnnotations: map[string]string{},
		ImageCreatedBy:   "",
	}

	data.initConfig()

	return data, nil

}

// MakeImageRef converts a CopyData struct into a types.ImageReference
func (c *CopyData) MakeImageRef(manifestType string, compress archive.Compression, names []string, layerID string, historyTimestamp *time.Time) (types.ImageReference, error) {
	var name reference.Named
	if len(names) > 0 {
		if parsed, err := reference.ParseNamed(names[0]); err == nil {
			name = parsed
		}
	}
	if manifestType == "" {
		manifestType = OCIv1ImageManifest
	}
	oconfig, err := json.Marshal(&c.OCIv1)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding OCI-format image configuration")
	}
	dconfig, err := json.Marshal(&c.Docker)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding docker-format image configuration")
	}
	created := time.Now().UTC()
	if historyTimestamp != nil {
		created = historyTimestamp.UTC()
	}
	ref := &CopyRef{
		store:                 c.store,
		compression:           compress,
		name:                  name,
		names:                 names,
		layerID:               layerID,
		addHistory:            false,
		oconfig:               oconfig,
		dconfig:               dconfig,
		created:               created,
		createdBy:             c.ImageCreatedBy,
		annotations:           c.ImageAnnotations,
		preferredManifestType: manifestType,
		exporting:             true,
	}
	return ref, nil
}
