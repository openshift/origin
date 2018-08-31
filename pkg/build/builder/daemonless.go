package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/image/pkg/docker/config"
	"github.com/containers/image/pkg/sysregistriesv2"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/opencontainers/runtime-spec/specs-go"
	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah"
	"github.com/projectatomic/buildah/imagebuildah"
	"github.com/projectatomic/buildah/util"
)

var (
	// TODO: run unprivileged https://github.com/openshift/origin/issues/662
	daemonlessStoreOptions = storage.DefaultStoreOptions
)

func checkForDaemonlessImage(imageName string) error {
	glog.V(2).Infof("Checking if %q has already been pulled....", imageName)

	store, err := storage.GetStore(daemonlessStoreOptions)
	if err != nil {
		return err
	}
	defer func() {
		if _, err := store.Shutdown(true); err != nil {
			glog.V(0).Infof("Error shutting down storage: %v", err)
		}
	}()

	var systemContext *types.SystemContext
	_, img, err := util.FindImage(store, "", systemContext, imageName)
	if err != nil {
		switch errors.Cause(err) {
		case storage.ErrImageUnknown, docker.ErrNoSuchImage:
			glog.V(2).Infof("Local copy of %q is not present.", imageName)
			return docker.ErrNoSuchImage
		}
		return err
	}
	if img == nil {
		return docker.ErrNoSuchImage
	}

	glog.V(2).Infof("Local copy of %q is present.", imageName)
	return err
}

func pullDaemonlessImage(imageName string, authConfig docker.AuthConfiguration) error {
	glog.V(2).Infof("Asked to pull fresh copy of %q.", imageName)

	_, err := alltransports.ParseImageName("docker://" + imageName)
	if err != nil {
		return err
	}

	glog.V(2).Infof("Deferring pull of %q.", imageName)
	return nil
}

func buildDaemonlessImage(dir string, optimization buildapiv1.ImageOptimizationPolicy, opts *docker.BuildImageOptions) error {
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

	squash := false
	layers := false
	switch optimization {
	case buildapiv1.ImageOptimizationDaemonless:
		// layers: false, squash: false
	case buildapiv1.ImageOptimizationDaemonlessWithLayers:
		layers = true
	case buildapiv1.ImageOptimizationDaemonlessSquashed:
		squash = true
	default:
		return fmt.Errorf("internal error: image optimization policy %q not fully implemented", string(optimization))
	}

	var systemContext types.SystemContext
	if credsDir, ok := os.LookupEnv("PULL_DOCKERCFG_PATH"); ok {
		systemContext.AuthFilePath = filepath.Join(credsDir, "config.json")
	}
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
		TransientMounts:  transientMounts,
		Args:             args,
		Output:           opts.Name,
		Out:              opts.OutputStream,
		Err:              opts.OutputStream,
		ReportWriter:     opts.OutputStream,
		OutputFormat:     imagebuildah.Dockerv2ImageFormat,
		SystemContext:    &systemContext,
		NamespaceOptions: buildah.NamespaceOptions{
			{Name: string(specs.NetworkNamespace), Host: true},
		},
		CommonBuildOpts: &buildah.CommonBuildOptions{
			Memory:       opts.Memory,
			MemorySwap:   opts.Memswap,
			CgroupParent: opts.CgroupParent,
		},
		Squash:                  squash,
		Layers:                  layers,
		NoCache:                 opts.NoCache,
		RemoveIntermediateCtrs:  opts.RmTmpContainer,
		ForceRmIntermediateCtrs: true,
	}

	store, err := storage.GetStore(daemonlessStoreOptions)
	if err != nil {
		return err
	}
	defer func() {
		if _, err := store.Shutdown(true); err != nil {
			glog.V(0).Infof("Error shutting down storage: %v", err)
		}
	}()

	return imagebuildah.BuildDockerfiles(opts.Context, store, options, opts.Dockerfile)
}

func tagDaemonlessImage(buildTag, pushTag string) error {
	glog.V(2).Infof("Tagging local image %q with name %q.", buildTag, pushTag)

	store, err := storage.GetStore(daemonlessStoreOptions)
	if err != nil {
		return err
	}
	defer func() {
		if _, err := store.Shutdown(true); err != nil {
			glog.V(0).Infof("Error shutting down storage: %v", err)
		}
	}()

	var systemContext *types.SystemContext
	_, img, err := util.FindImage(store, "", systemContext, buildTag)
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

func removeDaemonlessImage(buildTag string) error {
	glog.V(2).Infof("Removing name %q from local image.", buildTag)

	store, err := storage.GetStore(daemonlessStoreOptions)
	if err != nil {
		return err
	}
	defer func() {
		if _, err := store.Shutdown(true); err != nil {
			glog.V(0).Infof("Error shutting down storage: %v", err)
		}
	}()

	var systemContext *types.SystemContext
	_, img, err := util.FindImage(store, "", systemContext, buildTag)
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

func pushDaemonlessImage(imageName string, authConfig docker.AuthConfiguration) error {
	glog.V(2).Infof("Pushing image %q from local storage.", imageName)

	dest, err := alltransports.ParseImageName("docker://" + imageName)
	if err != nil {
		return fmt.Errorf("error parsing destination image name %s: %v", "docker://"+imageName, err)
	}

	var systemContext types.SystemContext
	if credsDir, ok := os.LookupEnv("PUSH_DOCKERCFG_PATH"); ok {
		systemContext.AuthFilePath = filepath.Join(credsDir, ".docker", "config.json")
	}
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

	store, err := storage.GetStore(daemonlessStoreOptions)
	if err != nil {
		return err
	}
	defer func() {
		if _, err := store.Shutdown(true); err != nil {
			glog.V(0).Infof("Error shutting down storage: %v", err)
		}
	}()

	options := buildah.PushOptions{
		ReportWriter:  os.Stdout,
		Store:         store,
		SystemContext: &systemContext,
	}

	return buildah.Push(context.TODO(), imageName, dest, options)
}
