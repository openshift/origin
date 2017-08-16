package server

import (
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/context"
	registryauth "github.com/docker/distribution/registry/auth"
	"github.com/docker/distribution/registry/middleware/registry"
	repomw "github.com/docker/distribution/registry/middleware/repository"
	storagedriver "github.com/docker/distribution/registry/storage/driver"
	registrystorage "github.com/docker/distribution/registry/storage/driver/middleware"
)

const (
	middlewareOpenShift = "openshift"
	middlewareAppParam  = "__app__"
)

func weaveAppIntoConfig(app *App, config *configuration.Configuration) {
	putApp := func(options configuration.Parameters, app *App) configuration.Parameters {
		if options == nil {
			options = make(configuration.Parameters)
		}
		options[middlewareAppParam] = app
		return options
	}

	if config.Auth.Type() == middlewareOpenShift {
		config.Auth[middlewareOpenShift] = putApp(config.Auth[middlewareOpenShift], app)
	}

	for _, typ := range []string{"storage", "registry", "repository"} {
		for i := range config.Middleware[typ] {
			middleware := &config.Middleware[typ][i]
			if middleware.Name == middlewareOpenShift {
				middleware.Options = putApp(middleware.Options, app)
			}
		}
	}
}

func init() {
	getApp := func(options map[string]interface{}) *App {
		app, _ := options[middlewareAppParam].(*App)
		return app
	}

	registryauth.Register(middlewareOpenShift, func(options map[string]interface{}) (registryauth.AccessController, error) {
		app := getApp(options)
		if app == nil {
			return nil, fmt.Errorf("failed to find an application instance in the access controller")
		}

		context.GetLogger(app.ctx).Info("Using Origin Auth handler")

		return app.newAccessController(options)
	})

	registrystorage.Register(middlewareOpenShift, func(driver storagedriver.StorageDriver, options map[string]interface{}) (storagedriver.StorageDriver, error) {
		app := getApp(options)
		if app == nil {
			return nil, fmt.Errorf("failed to find an application instance in the storage driver middleware")
		}

		context.GetLogger(app.ctx).Info("OpenShift middleware for storage driver initializing")

		// We can do this because of an initialization sequence of middlewares.
		// Storage driver is required to create registry. So we can be sure that
		// this assignment will happen before registry and repository initialization.
		app.driver = driver

		return driver, nil
	})

	middleware.Register(middlewareOpenShift, func(ctx context.Context, registry distribution.Namespace, options map[string]interface{}) (distribution.Namespace, error) {
		app := getApp(options)
		if app == nil {
			return nil, fmt.Errorf("failed to find an application instance in the registry middleware")
		}

		context.GetLogger(ctx).Info("OpenShift registry middleware initializing")

		app.registry = registry

		return registry, nil
	})

	repomw.Register(middlewareOpenShift, func(ctx context.Context, repo distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
		app := getApp(options)
		if app == nil {
			return nil, fmt.Errorf("failed to find an application instance in the repository middleware")
		}

		return app.newRepository(ctx, repo, options)
	})
}
