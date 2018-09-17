// +build linux

package builder

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/image/pkg/docker/config"
	"github.com/containers/image/pkg/sysregistriesv2"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/opencontainers/runtime-spec/specs-go"
	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah"
	"github.com/projectatomic/buildah/imagebuildah"
	"github.com/projectatomic/buildah/util"
)

func pullDaemonlessImage(sc types.SystemContext, store storage.Store, imageName string, authConfig docker.AuthConfiguration) error {
	glog.V(2).Infof("Asked to pull fresh copy of %q.", imageName)

	if imageName == "" {
		return fmt.Errorf("unable to pull using empty image name")
	}

	_, err := alltransports.ParseImageName("docker://" + imageName)
	if err != nil {
		return err
	}

	systemContext := sc
	// if credsDir, ok := os.LookupEnv("PULL_DOCKERCFG_PATH"); ok {
	// 	systemContext.AuthFilePath = filepath.Join(credsDir, "config.json")
	// }
	systemContext.AuthFilePath = "/tmp/config.json"

	registries, err := sysregistriesv2.GetRegistries(&systemContext)
	if err != nil {
		return fmt.Errorf("error reading system registries configuration: %v", err)
	}
	if registry := sysregistriesv2.FindRegistry(imageName, registries); registry != nil {
		if authConfig.Username != "" && authConfig.Password != "" {
			glog.V(2).Infof("Setting authentication for registry %q for %q.", registry.URL, imageName)
			if err := config.SetAuthentication(&systemContext, registry.URL, authConfig.Username, authConfig.Password); err != nil {
				return err
			}
		}
		if registry.Insecure {
			glog.V(2).Infof("Registry %q is marked as insecure in the registries configuration.", registry.URL)
			systemContext.DockerInsecureSkipTLSVerify = true
			systemContext.OCIInsecureSkipTLSVerify = true
		} else {
			glog.V(2).Infof("Registry %q is marked as secure in the registries configuration.", registry.URL)
		}
	} else {
		glog.V(2).Infof("Registry for %q is not present in the registries configuration, assuming it is secure.", imageName)
	}

	options := buildah.PullOptions{
		ReportWriter:  os.Stderr,
		Store:         store,
		SystemContext: &systemContext,
	}
	_, err = buildah.Pull(context.TODO(), "docker://"+imageName, options)
	return err
}

func buildDaemonlessImage(sc types.SystemContext, store storage.Store, isolation buildah.Isolation, dir string, optimization buildapiv1.ImageOptimizationPolicy, opts *docker.BuildImageOptions) error {
	glog.V(2).Infof("Building...")

	args := make(map[string]string)
	for _, ev := range opts.BuildArgs {
		args[ev.Name] = ev.Value
	}

	pullPolicy := buildah.PullIfMissing
	if opts.Pull {
		glog.V(2).Infof("Forcing fresh pull of base image.")
		pullPolicy = buildah.PullAlways
	}

	layers := false
	switch optimization {
	case buildapiv1.ImageOptimizationSkipLayers, buildapiv1.ImageOptimizationSkipLayersAndWarn:
		// layers: false
	case buildapiv1.ImageOptimizationNone:
		// layers = true // TODO: enable
	default:
		return fmt.Errorf("internal error: image optimization policy %q not fully implemented", string(optimization))
	}

	systemContext := sc
	// if credsDir, ok := os.LookupEnv("PULL_DOCKERCFG_PATH"); ok {
	// 	systemContext.AuthFilePath = filepath.Join(credsDir, "config.json")
	// }
	systemContext.AuthFilePath = "/tmp/config.json"

	for registry, ac := range opts.AuthConfigs.Configs {
		glog.V(2).Infof("Setting authentication for registry %q at %q.", registry, ac.ServerAddress)
		if err := config.SetAuthentication(&systemContext, registry, ac.Username, ac.Password); err != nil {
			return err
		}
		if err := config.SetAuthentication(&systemContext, ac.ServerAddress, ac.Username, ac.Password); err != nil {
			return err
		}
	}

	var transientMounts []imagebuildah.Mount
	if st, err := os.Stat("/run/secrets"); err == nil && st.IsDir() {
		// Add a non-recursive bind of /run/secrets, to pass along
		// anything that the runtime mounted from the node into our
		// /run/secrets, without passing along API secrets that were
		// mounted under it exclusively for the builder's use.
		transientMounts = append(transientMounts, imagebuildah.Mount{
			Source:      "/run/secrets",
			Destination: "/run/secrets",
			Type:        "bind",
			Options:     []string{"ro", "nodev", "noexec", "nosuid"},
		})
	}

	options := imagebuildah.BuildOptions{
		ContextDirectory: dir,
		PullPolicy:       pullPolicy,
		Isolation:        isolation,
		TransientMounts:  transientMounts,
		Args:             args,
		Output:           opts.Name,
		Out:              opts.OutputStream,
		Err:              opts.OutputStream,
		ReportWriter:     opts.OutputStream,
		OutputFormat:     buildah.Dockerv2ImageManifest,
		SystemContext:    &systemContext,
		NamespaceOptions: buildah.NamespaceOptions{
			{Name: string(specs.NetworkNamespace), Host: true},
		},
		CommonBuildOpts: &buildah.CommonBuildOptions{
			Memory:       opts.Memory,
			MemorySwap:   opts.Memswap,
			CgroupParent: opts.CgroupParent,
		},
		Layers:                  layers,
		NoCache:                 opts.NoCache,
		RemoveIntermediateCtrs:  opts.RmTmpContainer,
		ForceRmIntermediateCtrs: true,
	}

	return imagebuildah.BuildDockerfiles(opts.Context, store, options, opts.Dockerfile)
}

func tagDaemonlessImage(sc types.SystemContext, store storage.Store, buildTag, pushTag string) error {
	glog.V(2).Infof("Tagging local image %q with name %q.", buildTag, pushTag)

	if buildTag == "" {
		return fmt.Errorf("unable to add tag to image with empty image name")
	}
	if pushTag == "" {
		return fmt.Errorf("unable to add empty tag to image")
	}

	systemContext := sc

	_, img, err := util.FindImage(store, "", &systemContext, buildTag)
	if err != nil {
		return err
	}
	if img == nil {
		return storage.ErrImageUnknown
	}
	if err := store.SetNames(img.ID, append(img.Names, pushTag)); err != nil {
		return err
	}

	return nil
}

func removeDaemonlessImage(sc types.SystemContext, store storage.Store, buildTag string) error {
	glog.V(2).Infof("Removing name %q from local image.", buildTag)

	if buildTag == "" {
		return fmt.Errorf("unable to remove image using empty image name")
	}

	systemContext := sc

	_, img, err := util.FindImage(store, "", &systemContext, buildTag)
	if err != nil {
		return err
	}
	if img == nil {
		return storage.ErrImageUnknown
	}

	filtered := make([]string, 0, len(img.Names))
	for _, name := range img.Names {
		if name != buildTag {
			filtered = append(filtered, name)
		}
	}
	if err := store.SetNames(img.ID, filtered); err != nil {
		return err
	}

	return nil
}

func pushDaemonlessImage(sc types.SystemContext, store storage.Store, imageName string, authConfig docker.AuthConfiguration) error {
	glog.V(2).Infof("Pushing image %q from local storage.", imageName)

	if imageName == "" {
		return fmt.Errorf("unable to push using empty destination image name")
	}

	dest, err := alltransports.ParseImageName("docker://" + imageName)
	if err != nil {
		return fmt.Errorf("error parsing destination image name %s: %v", "docker://"+imageName, err)
	}

	systemContext := sc
	// if credsDir, ok := os.LookupEnv("PUSH_DOCKERCFG_PATH"); ok {
	// 	systemContext.AuthFilePath = filepath.Join(credsDir, ".docker", "config.json")
	// }
	systemContext.AuthFilePath = "/tmp/config.json"

	if authConfig.Username != "" && authConfig.Password != "" {
		glog.V(2).Infof("Setting authentication secret for %q.", authConfig.ServerAddress)
		systemContext.DockerAuthConfig = &types.DockerAuthConfig{
			Username: authConfig.Username,
			Password: authConfig.Password,
		}
	} else {
		glog.V(2).Infof("No authentication secret provided for pushing to registry.")
	}

	registries, err := sysregistriesv2.GetRegistries(&systemContext)
	if err != nil {
		return fmt.Errorf("error reading system registries configuration: %v", err)
	}
	if registry := sysregistriesv2.FindRegistry(imageName, registries); registry != nil {
		if registry.Insecure {
			glog.V(2).Infof("Registry %q is marked as insecure in the registries configuration.", registry.URL)
			systemContext.DockerInsecureSkipTLSVerify = true
			systemContext.OCIInsecureSkipTLSVerify = true
		} else {
			glog.V(2).Infof("Registry %q is marked as secure in the registries configuration.", registry.URL)
		}
	} else {
		glog.V(2).Infof("Registry for %q is not present in the registries configuration, assuming it is secure.", imageName)
	}

	options := buildah.PushOptions{
		ReportWriter:  os.Stdout,
		Store:         store,
		SystemContext: &systemContext,
	}

	return buildah.Push(context.TODO(), imageName, dest, options)
}

func inspectDaemonlessImage(sc types.SystemContext, store storage.Store, name string) (*docker.Image, error) {
	systemContext := sc

	ref, img, err := util.FindImage(store, "", &systemContext, name)
	if err != nil {
		switch errors.Cause(err) {
		case storage.ErrImageUnknown, docker.ErrNoSuchImage:
			glog.V(2).Infof("Local copy of %q is not present.", name)
			return nil, docker.ErrNoSuchImage
		}
		return nil, err
	}
	if img == nil {
		return nil, docker.ErrNoSuchImage
	}

	image, err := ref.NewImage(context.TODO(), &systemContext)
	if err != nil {
		return nil, err
	}
	defer image.Close()

	size, err := image.Size()
	if err != nil {
		return nil, err
	}
	oconfig, err := image.OCIConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	var rootfs *docker.RootFS
	if len(oconfig.RootFS.DiffIDs) > 0 {
		rootfs = &docker.RootFS{
			Type: oconfig.RootFS.Type,
		}
		for _, d := range oconfig.RootFS.DiffIDs {
			rootfs.Layers = append(rootfs.Layers, d.String())
		}
	}

	exposedPorts := make(map[docker.Port]struct{})
	for port := range oconfig.Config.ExposedPorts {
		exposedPorts[docker.Port(port)] = struct{}{}
	}

	config := docker.Config{
		User:         oconfig.Config.User,
		ExposedPorts: exposedPorts,
		Env:          oconfig.Config.Env,
		Entrypoint:   oconfig.Config.Entrypoint,
		Cmd:          oconfig.Config.Cmd,
		Volumes:      oconfig.Config.Volumes,
		WorkingDir:   oconfig.Config.WorkingDir,
		Labels:       oconfig.Config.Labels,
		StopSignal:   oconfig.Config.StopSignal,
	}

	var created time.Time
	if oconfig.Created != nil {
		created = *oconfig.Created
	}

	return &docker.Image{
		ID:              img.ID,
		RepoTags:        []string{},
		Parent:          "",
		Comment:         "",
		Created:         created,
		Container:       "",
		ContainerConfig: config,
		DockerVersion:   "",
		Author:          oconfig.Author,
		Config:          &config,
		Architecture:    oconfig.Architecture,
		Size:            size,
		VirtualSize:     size,
		RepoDigests:     []string{},
		RootFS:          rootfs,
		OS:              oconfig.OS,
	}, nil
}

// daemonlessRun mimics the 'docker run --rm' CLI command well enough. It creates and
// starts a container and streams its logs. The container is removed after it terminates.
func daemonlessRun(ctx context.Context, store storage.Store, isolation buildah.Isolation, createOpts docker.CreateContainerOptions, attachOpts docker.AttachToContainerOptions) error {
	if createOpts.Config == nil {
		return fmt.Errorf("error calling daemonlessRun: expected a Config")
	}
	if createOpts.HostConfig == nil {
		return fmt.Errorf("error calling daemonlessRun: expected a HostConfig")
	}

	builderOptions := buildah.BuilderOptions{
		Container: createOpts.Name,
		FromImage: createOpts.Config.Image,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			Memory:       createOpts.HostConfig.Memory,
			MemorySwap:   createOpts.HostConfig.MemorySwap,
			CgroupParent: createOpts.HostConfig.CgroupParent,
		},
	}

	runOptions := buildah.RunOptions{
		Isolation:  isolation,
		Entrypoint: createOpts.Config.Entrypoint,
		Cmd:        createOpts.Config.Cmd,
		Stdout:     attachOpts.OutputStream,
		Stderr:     attachOpts.ErrorStream,
	}

	builder, err := buildah.NewBuilder(ctx, store, builderOptions)
	if err != nil {
		return err
	}
	defer func() {
		if err := builder.Delete(); err != nil {
			glog.V(0).Infof("Error deleting container %q(%s): %v", builder.Container, builder.ContainerID, err)
		}
	}()

	return builder.Run(append(createOpts.Config.Entrypoint, createOpts.Config.Cmd...), runOptions)
}

func downloadFromDaemonlessContainer(builder *buildah.Builder, id string, path string, outputStream io.Writer) error {
	mp, err := builder.Mount("")
	if err != nil {
		return err
	}
	defer func() {
		if err := builder.Unmount(); err != nil {
			glog.V(0).Infof("Error shutting down storage: %v", err)
		}
	}()
	var options archive.TarOptions
	if !builder.IDMappingOptions.HostUIDMapping {
		for _, m := range builder.IDMappingOptions.UIDMap {
			idmap := idtools.IDMap{
				HostID:      int(m.HostID),
				ContainerID: int(m.ContainerID),
				Size:        int(m.Size),
			}
			options.UIDMaps = append(options.UIDMaps, idmap)
		}
	}
	if !builder.IDMappingOptions.HostGIDMapping {
		for _, m := range builder.IDMappingOptions.GIDMap {
			idmap := idtools.IDMap{
				HostID:      int(m.HostID),
				ContainerID: int(m.ContainerID),
				Size:        int(m.Size),
			}
			options.GIDMaps = append(options.GIDMaps, idmap)
		}
	}
	rc, err := archive.TarWithOptions(filepath.Join(mp, path), &options)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(outputStream, rc)
	return err
}

// DaemonlessClient is a daemonless DockerClient-like implementation.
type DaemonlessClient struct {
	SystemContext types.SystemContext
	Store         storage.Store
	Isolation     buildah.Isolation
	builders      map[string]*buildah.Builder
}

// GetDaemonlessClient returns a valid implemenatation of the DockerClient
// interface, or an error if the implementation couldn't be created.
func GetDaemonlessClient(systemContext types.SystemContext, store storage.Store, isolationSpec string) (client DockerClient, err error) {
	isolation := buildah.IsolationDefault
	switch strings.ToLower(isolationSpec) {
	case "chroot":
		isolation = buildah.IsolationChroot
	case "oci":
		isolation = buildah.IsolationOCI
	case "rootless":
		isolation = buildah.IsolationOCIRootless
	case "":
	default:
		return nil, fmt.Errorf("unrecognized BUILD_ISOLATION setting %q", strings.ToLower(isolationSpec))
	}

	return &DaemonlessClient{
		SystemContext: systemContext,
		Store:         store,
		Isolation:     isolation,
		builders:      make(map[string]*buildah.Builder),
	}, nil
}

func (d *DaemonlessClient) BuildImage(opts docker.BuildImageOptions) error {
	return buildDaemonlessImage(d.SystemContext, d.Store, d.Isolation, opts.ContextDir, buildapiv1.ImageOptimizationNone, &opts)
}

func (d *DaemonlessClient) PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error {
	imageName := opts.Name
	if opts.Tag != "" {
		imageName = imageName + ":" + opts.Tag
	}
	return pushDaemonlessImage(d.SystemContext, d.Store, imageName, auth)
}

func (d *DaemonlessClient) RemoveImage(name string) error {
	return removeDaemonlessImage(d.SystemContext, d.Store, name)
}

func (d *DaemonlessClient) CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error) {
	options := buildah.BuilderOptions{
		FromImage: opts.Config.Image,
		Container: opts.Name,
	}
	builder, err := buildah.NewBuilder(opts.Context, d.Store, options)
	if err != nil {
		return nil, err
	}
	builder.SetCmd(opts.Config.Cmd)
	builder.SetEntrypoint(opts.Config.Entrypoint)
	if builder.Container != "" {
		d.builders[builder.Container] = builder
	}
	if builder.ContainerID != "" {
		d.builders[builder.ContainerID] = builder
	}
	return &docker.Container{ID: builder.ContainerID}, nil
}

func (d *DaemonlessClient) DownloadFromContainer(id string, opts docker.DownloadFromContainerOptions) error {
	builder, ok := d.builders[id]
	if !ok {
		return errors.Errorf("no such container as %q", id)
	}
	return downloadFromDaemonlessContainer(builder, id, opts.Path, opts.OutputStream)
}

func (d *DaemonlessClient) RemoveContainer(opts docker.RemoveContainerOptions) error {
	builder, ok := d.builders[opts.ID]
	if !ok {
		return errors.Errorf("no such container as %q", opts.ID)
	}
	name := builder.Container
	id := builder.ContainerID
	err := builder.Delete()
	if err == nil {
		if name != "" {
			if _, ok := d.builders[name]; ok {
				delete(d.builders, name)
			}
		}
		if id != "" {
			if _, ok := d.builders[id]; ok {
				delete(d.builders, id)
			}
		}
	}
	return err
}

func (d *DaemonlessClient) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	imageName := opts.Repository
	if opts.Tag != "" {
		imageName = imageName + ":" + opts.Tag
	}
	return pullDaemonlessImage(d.SystemContext, d.Store, imageName, auth)
}

func (d *DaemonlessClient) TagImage(name string, opts docker.TagImageOptions) error {
	imageName := opts.Repo
	if opts.Tag != "" {
		imageName = imageName + ":" + opts.Tag
	}
	return tagDaemonlessImage(d.SystemContext, d.Store, name, imageName)
}

func (d *DaemonlessClient) InspectImage(name string) (*docker.Image, error) {
	return inspectDaemonlessImage(d.SystemContext, d.Store, name)
}
