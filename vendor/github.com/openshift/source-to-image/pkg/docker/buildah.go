// +build !stinobuildah

package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	imageimage "github.com/containers/image/image"
	imagemanifest "github.com/containers/image/manifest"
	"github.com/containers/image/pkg/sysregistriesv2"
	istorage "github.com/containers/image/storage"
	imagetypes "github.com/containers/image/types"
	"github.com/containers/storage"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/constants"
	s2itar "github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util/fs"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah"
	"github.com/projectatomic/buildah/imagebuildah"
)

type stiBuildah struct {
	store         storage.Store
	ctx           context.Context
	systemContext imagetypes.SystemContext
	isolation     buildah.Isolation
	buildersLock  sync.Mutex
	builders      map[string]*buildah.Builder
}

// NewBuildah creates a new implementation of the STI Docker interface
func NewBuildah(ctx context.Context, store storage.Store, systemContext *imagetypes.SystemContext, isolation buildah.Isolation, auth api.AuthConfig) (Docker, error) {
	var b stiBuildah
	b.store = store
	if systemContext != nil {
		b.systemContext = *systemContext
	}
	if auth.Username != "" && auth.Password != "" {
		b.systemContext.DockerAuthConfig = &imagetypes.DockerAuthConfig{
			Username: auth.Username,
			Password: auth.Password,
		}
	}
	b.isolation = isolation
	b.builders = make(map[string]*buildah.Builder)
	return &b, nil
}

func (b *stiBuildah) IsImageInLocalRegistry(name string) (bool, error) {
	ref, err := istorage.Transport.ParseStoreReference(b.store, name)
	if err != nil {
		return false, err
	}
	img, err := ref.NewImageSource(context.TODO(), &b.systemContext)
	if err != nil {
		if errors.Cause(err) != storage.ErrImageUnknown && errors.Cause(err) != storage.ErrNotAnImage {
			return false, err
		}
	}
	if img != nil {
		img.Close()
	}
	return img != nil, nil
}

func (b *stiBuildah) IsImageOnBuild(name string) bool {
	onbuild, err := b.GetOnBuild(name)
	return len(onbuild) > 0 && err == nil
}

func (b *stiBuildah) GetOnBuild(name string) ([]string, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	_, _, onBuild, err := b.queryImageByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return onBuild, nil
}

func (b *stiBuildah) RemoveContainer(id string) error {
	b.buildersLock.Lock()
	defer b.buildersLock.Unlock()
	builder, ok := b.builders[id]
	if !ok {
		return errors.Errorf("no builder container %q", id)
	}
	err := builder.Delete()
	if err != nil {
		return err
	}
	delete(b.builders, builder.ContainerID)
	if builder.Container != "" {
		delete(b.builders, builder.Container)
	}
	return nil
}

func (b *stiBuildah) GetScriptsURL(name string) (string, error) {
	imageMetadata, err := b.CheckAndPullImage(name)
	if err != nil {
		return "", err
	}
	return getScriptsURL(imageMetadata), nil
}

func (b *stiBuildah) GetAssembleInputFiles(name string) (string, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	_, v1, _, err := b.queryImageByName(ctx, name)
	if err != nil {
		return "", err
	}
	return v1.Config.Labels[constants.AssembleInputFilesLabel], nil
}

func (b *stiBuildah) RunContainer(opts RunContainerOptions) error {
	if opts.Stdin != nil {
		defer opts.Stdin.Close()
	}
	if opts.Stdout != nil {
		defer opts.Stdout.Close()
	}
	if opts.Stderr != nil {
		defer opts.Stderr.Close()
	}
	if opts.TargetImage {
		glog.V(0).Infof("TargetImage is not implemented when using buildah as an engine")
		return errors.New("TargetImage is not implemented when using buildah as an engine")
	}
	if opts.NetworkMode != "" && opts.NetworkMode != string(api.DockerNetworkModeHost) {
		glog.V(0).Infof("Only %q networking is available when using buildah as an engine, requested '%s'", api.DockerNetworkModeHost, opts.NetworkMode)
		return errors.Errorf("Only %q networking is available when using buildah as an engine, requested '%s'", api.DockerNetworkModeHost, opts.NetworkMode)
	}

	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	pullPolicy := buildah.PullIfMissing
	if opts.PullImage {
		pullPolicy = buildah.PullAlways
	}

	systemContext := b.systemContext
	if opts.PullAuth.Username != "" && opts.PullAuth.Password != "" {
		systemContext.DockerAuthConfig = &imagetypes.DockerAuthConfig{
			Username: opts.PullAuth.Username,
			Password: opts.PullAuth.Password,
		}
	}

	registries, err := sysregistriesv2.GetRegistries(&systemContext)
	if err != nil {
		return errors.Wrapf(err, "error reading system registries configuration: %v")
	}
	if registry := sysregistriesv2.FindRegistry(opts.Image, registries); registry != nil {
		if registry.Insecure {
			glog.V(2).Infof("Registry %q is marked as insecure in the registries configuration.", registry.URL)
			systemContext.DockerInsecureSkipTLSVerify = true
			systemContext.OCIInsecureSkipTLSVerify = true
		} else {
			glog.V(2).Infof("Registry %q is marked as secure in the registries configuration.", registry.URL)
		}
	} else {
		glog.V(2).Infof("Registry for %q is not present in the registries configuration, assuming it is secure.", opts.Image)
	}

	var commonBuildOpts *buildah.CommonBuildOptions
	if opts.CGroupLimits != nil {
		commonBuildOpts = &buildah.CommonBuildOptions{
			Memory:       opts.CGroupLimits.MemoryLimitBytes,
			CPUShares:    uint64(opts.CGroupLimits.CPUShares),
			CPUPeriod:    uint64(opts.CGroupLimits.CPUPeriod),
			CPUQuota:     opts.CGroupLimits.CPUQuota,
			MemorySwap:   opts.CGroupLimits.MemorySwap,
			CgroupParent: opts.CGroupLimits.Parent,
			Volumes:      opts.Binds,
		}
	}

	builderOptions := buildah.BuilderOptions{
		FromImage:       opts.Image,
		PullPolicy:      pullPolicy,
		ReportWriter:    opts.Stderr,
		CommonBuildOpts: commonBuildOpts,
	}
	builder, err := buildah.NewBuilder(ctx, b.store, builderOptions)
	if err != nil {
		return err
	}
	b.buildersLock.Lock()
	if builder.Container != "" {
		b.builders[builder.Container] = builder
	}
	b.builders[builder.ContainerID] = builder
	b.buildersLock.Unlock()
	defer func() {
		if err := b.RemoveContainer(builder.ContainerID); err != nil {
			glog.V(0).Infof("Error removing container %q(%s): %v", builder.Container, builder.ContainerID, err)
		}
	}()

	imageMetadata, _, _, err := b.queryImageByName(ctx, builder.FromImageID)
	if err != nil {
		return err
	}
	entrypoint := DefaultEntrypoint
	if len(opts.Entrypoint) > 0 {
		entrypoint = opts.Entrypoint
	}

	var tarDestination string
	var cmd []string
	if !opts.TargetImage {
		if len(opts.CommandExplicit) != 0 {
			cmd = opts.CommandExplicit
		} else {
			tarDestination = determineTarDestinationDir(opts, imageMetadata)
			cmd = constructCommand(opts, imageMetadata, tarDestination)
		}
		glog.V(5).Infof("Setting %q command for container ...", strings.Join(cmd, " "))
	}

	//  TODO
	//  // SecurityOpt is passed through as security options to the underlying container.
	//  SecurityOpt []string

	var capDrops []string
	for _, cap := range opts.CapDrop {
		capDrops = append(capDrops, "CAP_"+cap)
	}

	runOptions := buildah.RunOptions{
		Isolation:        b.isolation,
		Env:              opts.Env,
		User:             opts.User,
		Entrypoint:       entrypoint,
		Cmd:              cmd,
		Stdin:            opts.Stdin,
		Stdout:           opts.Stdout,
		Stderr:           opts.Stderr,
		DropCapabilities: capDrops,
	}

	onStartDone := make(chan error, 1)
	if opts.OnStart != nil {
		go func() {
			onStartDone <- opts.OnStart(builder.ContainerID)
		}()
	}

	if opts.TargetImage {
		glog.V(0).Infof("\n\n\n\n\nThe image %s has been started in container %s as a result of the --run=true option.  The container's stdout/stderr will be redirected to this command's glog output to help you validate its behavior.  If the container is set up to stay running, you will have to Ctrl-C to exit this command, which should also stop the container %s.  This particular invocation attempts to run with no port mappings.\n", builder.FromImage, builder.ContainerID, builder.ContainerID)
	}

	err = builder.Run(append(entrypoint, cmd...), runOptions)
	if err != nil {
		return err
	}

	if opts.OnStart != nil {
		if err = <-onStartDone; err != nil {
			return err
		}
	}

	if opts.PostExec != nil {
		glog.V(2).Infof("Invoking PostExecute function")
		if err = opts.PostExec.PostExecute(builder.ContainerID, tarDestination); err != nil {
			return err
		}
	}

	return nil
}

func (b *stiBuildah) GetImageID(name string) (string, error) {
	imageMetadata, err := b.CheckAndPullImage(name)
	if err != nil {
		return "", err
	}
	return imageMetadata.ID, nil
}

func (b *stiBuildah) GetImageWorkdir(name string) (string, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	_, v1, _, err := b.queryImageByName(ctx, name)
	if err != nil {
		return "", err
	}
	return v1.Config.WorkingDir, nil
}

func (b *stiBuildah) CommitContainer(opts CommitContainerOptions) (string, error) {
	b.buildersLock.Lock()
	builder, ok := b.builders[opts.ContainerID]
	b.buildersLock.Unlock()
	if !ok {
		return "", errors.Errorf("no builder container %q", opts.ContainerID)
	}
	dest, err := istorage.Transport.ParseStoreReference(b.store, opts.Repository)
	if err != nil {
		return "", err
	}
	builder.SetUser(opts.User)
	builder.SetCmd(opts.Command)
	builder.SetEntrypoint(opts.Entrypoint)
	for _, env := range opts.Env {
		envSpec := strings.SplitN(env, "=", 2)
		if len(envSpec) > 1 {
			builder.SetEnv(envSpec[0], envSpec[1])
		}
	}
	for label, value := range opts.Labels {
		builder.SetLabel(label, value)
	}
	options := buildah.CommitOptions{
		SystemContext: &b.systemContext,
		Parent:        builder.FromImageID,
		ReportWriter:  os.Stderr,
	}
	return builder.Commit(context.TODO(), dest, options)
}

func (b *stiBuildah) RemoveImage(name string) error {
	ref, err := istorage.Transport.ParseStoreReference(b.store, name)
	if err != nil {
		return err
	}
	return ref.DeleteImage(context.TODO(), &b.systemContext)
}

func (b *stiBuildah) CheckImage(name string) (*api.Image, error) {
	ref, err := istorage.Transport.ParseStoreReference(b.store, name)
	if err != nil {
		return nil, err
	}
	img, err := istorage.Transport.GetStoreImage(b.store, ref)
	if err != nil {
		return nil, err
	}
	return &api.Image{
		ID: img.ID,
		ContainerConfig: &api.ContainerConfig{
			Labels: map[string]string{},
			Env:    []string{},
		},
		Config: &api.ContainerConfig{
			Labels: map[string]string{},
			Env:    []string{},
		},
	}, nil
}

func (b *stiBuildah) queryImage(ctx context.Context, ref imagetypes.ImageReference) (*api.Image, *v1.Image, []string, error) {
	var closer io.Closer
	defer func() {
		if closer != nil {
			closer.Close()
		}
	}()

	src, err := ref.NewImageSource(ctx, &b.systemContext)
	if err != nil {
		return nil, nil, nil, err
	}
	closer = src

	img, err := imageimage.FromSource(ctx, nil, src)
	if err != nil {
		return nil, nil, nil, err
	}
	closer = img

	config, err := img.OCIConfig(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	_, manifestType, err := img.Manifest(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	configInfo := img.ConfigInfo()
	var configBlob bytes.Buffer
	var rc io.ReadCloser
	var onBuild []string
	switch manifestType {
	case imagemanifest.DockerV2Schema2MediaType:
		rc, _, err = src.GetBlob(ctx, configInfo)
		if err != nil {
			return nil, nil, nil, err
		}
		defer rc.Close()
		io.Copy(&configBlob, rc)
		var image imagemanifest.Schema2Image
		if err = json.Unmarshal(configBlob.Bytes(), &image); err != nil {
			return nil, nil, nil, err
		}
		if image.Config != nil {
			onBuild = image.Config.OnBuild
		}
	}

	return &api.Image{
		ID: "TBD",
		ContainerConfig: &api.ContainerConfig{
			Labels: config.Config.Labels,
			Env:    config.Config.Env,
		},
		Config: &api.ContainerConfig{
			Labels: config.Config.Labels,
			Env:    config.Config.Env,
		},
	}, config, onBuild, nil
}

func (b *stiBuildah) queryImageByName(ctx context.Context, name string) (*api.Image, *v1.Image, []string, error) {
	ref, err := istorage.Transport.ParseStoreReference(b.store, name)
	if err != nil {
		return nil, nil, nil, err
	}
	return b.queryImage(ctx, ref)
}

func (b *stiBuildah) PullImage(name string) (*api.Image, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	systemContext := b.systemContext

	registries, err := sysregistriesv2.GetRegistries(&systemContext)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading system registries configuration: %v")
	}
	if registry := sysregistriesv2.FindRegistry(name, registries); registry != nil {
		if registry.Insecure {
			glog.V(2).Infof("Registry %q is marked as insecure in the registries configuration.", registry.URL)
			systemContext.DockerInsecureSkipTLSVerify = true
			systemContext.OCIInsecureSkipTLSVerify = true
		} else {
			glog.V(2).Infof("Registry %q is marked as secure in the registries configuration.", registry.URL)
		}
	} else {
		glog.V(2).Infof("Registry for %q is not present in the registries configuration, assuming it is secure.", name)
	}

	options := buildah.PullOptions{
		Store:         b.store,
		SystemContext: &systemContext,
		ReportWriter:  os.Stderr,
	}
	ref, err := buildah.Pull(ctx, "docker://"+name, options)
	if err != nil {
		return nil, err
	}
	image, _, _, err := b.queryImage(ctx, ref)
	return image, err
}

func (b *stiBuildah) CheckAndPullImage(name string) (*api.Image, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	ref, err := istorage.Transport.ParseStoreReference(b.store, name)
	if err != nil {
		return nil, err
	}
	img, err := ref.NewImageSource(ctx, &b.systemContext)
	if err != nil {
		if errors.Cause(err) != storage.ErrImageUnknown && errors.Cause(err) != storage.ErrNotAnImage {
			return nil, err
		}
		return b.PullImage(name)
	}
	if img != nil {
		img.Close()
	}
	image, _, _, err := b.queryImage(ctx, ref)
	return image, err
}

func (b *stiBuildah) BuildImage(opts BuildImageOptions) error {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	if opts.Stdout != nil {
		defer opts.Stdout.Close()
	}

	var commonBuildOpts *buildah.CommonBuildOptions
	if opts.CGroupLimits != nil {
		commonBuildOpts = &buildah.CommonBuildOptions{
			Memory:       opts.CGroupLimits.MemoryLimitBytes,
			CPUShares:    uint64(opts.CGroupLimits.CPUShares),
			CPUPeriod:    uint64(opts.CGroupLimits.CPUPeriod),
			CPUQuota:     opts.CGroupLimits.CPUQuota,
			MemorySwap:   opts.CGroupLimits.MemorySwap,
			CgroupParent: opts.CGroupLimits.Parent,
		}
	}

	tmppath, err := ioutil.TempDir("/var/tmp", "buildah")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmppath)

	if err = s2itar.New(fs.NewFileSystem()).ExtractTarStreamWithLogging(tmppath, opts.Stdin, opts.Stdout); err != nil {
		return err
	}

	options := imagebuildah.BuildOptions{
		Output:                  opts.Name,
		Out:                     opts.Stdout,
		Err:                     opts.Stdout,
		CommonBuildOpts:         commonBuildOpts,
		RemoveIntermediateCtrs:  true,
		ForceRmIntermediateCtrs: true,
	}

	return imagebuildah.BuildDockerfiles(ctx, b.store, options, tmppath)
}

func (b *stiBuildah) GetImageUser(name string) (string, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	_, v1, _, err := b.queryImageByName(ctx, name)
	if err != nil {
		return "", err
	}
	return v1.Config.User, nil
}

func (b *stiBuildah) GetImageEntrypoint(name string) ([]string, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	_, v1, _, err := b.queryImageByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return v1.Config.Entrypoint, nil
}

func (b *stiBuildah) GetLabels(name string) (map[string]string, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	_, v1, _, err := b.queryImageByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return v1.Config.Labels, nil
}

func (b *stiBuildah) UploadToContainer(fs fs.FileSystem, srcPath, destPath, container string) error {
	makeTarWriter := func(w io.Writer) s2itar.Writer {
		return tar.NewWriter(w)
	}
	return b.UploadToContainerWithTarWriter(fs, srcPath, destPath, container, makeTarWriter)
}

type mappingTarWriter struct {
	writer   s2itar.Writer
	mappings buildah.IDMappingOptions
	mapID    func(idmap []specs.LinuxIDMapping, id int) (int, error)
}

func (m *mappingTarWriter) Write(p []byte) (n int, err error) {
	return m.writer.Write(p)
}

func (m *mappingTarWriter) Close() error {
	return m.writer.Close()
}

func (m *mappingTarWriter) Flush() error {
	return m.writer.Flush()
}

func (m *mappingTarWriter) WriteHeader(hdr *tar.Header) error {
	if m.mappings.HostUIDMapping && m.mappings.HostGIDMapping {
		return m.writer.WriteHeader(hdr)
	}
	uid := hdr.Uid
	if !m.mappings.HostUIDMapping {
		mapped, err := m.mapID(m.mappings.UIDMap, uid)
		if err != nil {
			return err
		}
		hdr.Uid = mapped
	}
	gid := hdr.Gid
	if !m.mappings.HostGIDMapping {
		mapped, err := m.mapID(m.mappings.GIDMap, gid)
		if err != nil {
			return err
		}
		hdr.Gid = mapped
	}
	return m.writer.WriteHeader(hdr)
}

func mapToContainerID(idmap []specs.LinuxIDMapping, id int) (int, error) {
	uid := uint32(id)
	for _, m := range idmap {
		if uid >= m.HostID && uid < m.HostID+m.Size {
			return int(m.ContainerID + (uid - m.HostID)), nil
		}
	}
	return 0, errors.Errorf("unable to map ID %d to an in-container ID using map %#v", id, idmap)
}

func mapToHostID(idmap []specs.LinuxIDMapping, id int) (int, error) {
	uid := uint32(id)
	for _, m := range idmap {
		if uid >= m.ContainerID && uid < m.ContainerID+m.Size {
			return int(m.HostID + (uid - m.ContainerID)), nil
		}
	}
	return 0, errors.Errorf("unable to map ID %d to an outside-of-container ID using map %#v", id, idmap)
}

func wrapTarWriterToContainer(w s2itar.Writer, b *buildah.Builder) s2itar.Writer {
	return &mappingTarWriter{
		writer:   w,
		mappings: b.IDMappingOptions,
		mapID:    mapToHostID,
	}
}

func wrapTarWriterFromContainer(w s2itar.Writer, b *buildah.Builder) s2itar.Writer {
	return &mappingTarWriter{
		writer:   w,
		mappings: b.IDMappingOptions,
		mapID:    mapToContainerID,
	}
}

func (b *stiBuildah) UploadToContainerWithTarWriter(fsops fs.FileSystem, srcPath, destPath, container string, makeTarWriter func(io.Writer) s2itar.Writer) error {
	b.buildersLock.Lock()
	builder, ok := b.builders[container]
	b.buildersLock.Unlock()
	if !ok {
		return errors.Errorf("no builder container %q", container)
	}
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	root, err := builder.Mount("")
	if err != nil {
		return err
	}
	defer func() {
		if err := builder.Unmount(); err != nil {
			glog.V(0).Infof("Error umounting container %q(%s): %v", builder.Container, builder.ContainerID, err)
		}
	}()
	if err := s2itar.New(fsops).CreateTarStreamToTarWriter(srcPath, false, wrapTarWriterToContainer(makeTarWriter(pipeWriter), builder), nil); err != nil {
		return err
	}
	return s2itar.New(fs.NewFileSystem()).ExtractTarStreamWithLogging(filepath.Join(root, destPath), pipeReader, nil)
}

func (b *stiBuildah) DownloadFromContainer(containerPath string, w io.Writer, container string) error {
	b.buildersLock.Lock()
	builder, ok := b.builders[container]
	b.buildersLock.Unlock()
	if !ok {
		return errors.Errorf("no builder container %q", container)
	}
	root, err := builder.Mount("")
	if err != nil {
		return err
	}
	defer func() {
		if err := builder.Unmount(); err != nil {
			glog.V(0).Infof("Error umounting container %q(%s): %v", builder.Container, builder.ContainerID, err)
		}
	}()
	return s2itar.New(fs.NewFileSystem()).CreateTarStreamToTarWriter(filepath.Join(root, containerPath), false, wrapTarWriterFromContainer(tar.NewWriter(w), builder), nil)
}

func (b *stiBuildah) Version() (dockertypes.Version, error) {
	return dockertypes.Version{
		Version:       "",
		APIVersion:    "", // `json:"ApiVersion"`
		MinAPIVersion: "", // `json:"MinAPIVersion,omitempty"`
		GitCommit:     "",
		GoVersion:     "",
		Os:            "",
		Arch:          "",
		KernelVersion: "",   // `json:",omitempty"`
		Experimental:  true, // `json:",omitempty"`
		BuildTime:     "",   // `json:",omitempty"`
	}, nil
}

func (b *stiBuildah) CheckReachable() error {
	return nil
}
