/*
Copyright 2022 The Kubernetes Authors.

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

package kubeletplugin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	resourceapi "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	drapbv1alpha4 "k8s.io/kubelet/pkg/apis/dra/v1alpha4"
	drapbv1beta1 "k8s.io/kubelet/pkg/apis/dra/v1beta1"
	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
)

// DRAPlugin gets returned by Start and defines the public API of the generic
// dynamic resource allocation plugin.
type DRAPlugin interface {
	// Stop ensures that all spawned goroutines are stopped and frees
	// resources.
	Stop()

	// RegistrationStatus returns the result of registration, nil if none
	// received yet.
	RegistrationStatus() *registerapi.RegistrationStatus

	// PublishResources may be called one or more times to publish
	// resource information in ResourceSlice objects. If it never gets
	// called, then the kubelet plugin does not manage any ResourceSlice
	// objects.
	//
	// PublishResources does not block, so it might still take a while
	// after it returns before all information is actually written
	// to the API server.
	//
	// The caller must not modify the content after the call.
	//
	// Returns an error if KubeClient or NodeName options were not
	// set in Start() to create the DRAPlugin instance.
	PublishResources(ctx context.Context, resources Resources) error

	// This unexported method ensures that we can modify the interface
	// without causing an API break of the package
	// (https://pkg.go.dev/golang.org/x/exp/apidiff#section-readme).
	internal()
}

// Resources currently only supports devices. Might get extended in the
// future.
type Resources struct {
	Devices []resourceapi.Device
}

// Option implements the functional options pattern for Start.
type Option func(o *options) error

// DriverName defines the driver name for the dynamic resource allocation driver.
// Must be set.
func DriverName(driverName string) Option {
	return func(o *options) error {
		o.driverName = driverName
		return nil
	}
}

// GRPCVerbosity sets the verbosity for logging gRPC calls. Default is 4. A negative
// value disables logging.
func GRPCVerbosity(level int) Option {
	return func(o *options) error {
		o.grpcVerbosity = level
		return nil
	}
}

// RegistrarSocketPath sets the file path for a Unix domain socket.
// If RegistrarListener is not used, then Start will remove
// a file at that path, should one exist, and creates a socket
// itself. Otherwise it uses the provided listener and only
// removes the socket at the specified path during shutdown.
//
// At least one of these two options is required.
func RegistrarSocketPath(path string) Option {
	return func(o *options) error {
		o.pluginRegistrationEndpoint.path = path
		return nil
	}
}

// RegistrarListener sets an already created listener for the plugin
// registration API. Can be combined with RegistrarSocketPath.
//
// At least one of these two options is required.
func RegistrarListener(listener net.Listener) Option {
	return func(o *options) error {
		o.pluginRegistrationEndpoint.listener = listener
		return nil
	}
}

// PluginSocketPath sets the file path for a Unix domain socket.
// If PluginListener is not used, then Start will remove
// a file at that path, should one exist, and creates a socket
// itself. Otherwise it uses the provided listener and only
// removes the socket at the specified path during shutdown.
//
// At least one of these two options is required.
func PluginSocketPath(path string) Option {
	return func(o *options) error {
		o.draEndpoint.path = path
		return nil
	}
}

// PluginListener sets an already created listener for the dynamic resource
// allocation plugin API. Can be combined with PluginSocketPath.
//
// At least one of these two options is required.
func PluginListener(listener net.Listener) Option {
	return func(o *options) error {
		o.draEndpoint.listener = listener
		return nil
	}
}

// KubeletPluginSocketPath defines how kubelet will connect to the dynamic
// resource allocation plugin. This corresponds to PluginSocketPath, except
// that PluginSocketPath defines the path in the filesystem of the caller and
// KubeletPluginSocketPath in the filesystem of kubelet.
func KubeletPluginSocketPath(path string) Option {
	return func(o *options) error {
		o.draAddress = path
		return nil
	}
}

// GRPCInterceptor is called for each incoming gRPC method call. This option
// may be used more than once and each interceptor will get called.
func GRPCInterceptor(interceptor grpc.UnaryServerInterceptor) Option {
	return func(o *options) error {
		o.unaryInterceptors = append(o.unaryInterceptors, interceptor)
		return nil
	}
}

// GRPCStreamInterceptor is called for each gRPC streaming method call. This option
// may be used more than once and each interceptor will get called.
func GRPCStreamInterceptor(interceptor grpc.StreamServerInterceptor) Option {
	return func(o *options) error {
		o.streamInterceptors = append(o.streamInterceptors, interceptor)
		return nil
	}
}

// NodeV1alpha4 explicitly chooses whether the DRA gRPC API v1alpha4
// gets enabled.
func NodeV1alpha4(enabled bool) Option {
	return func(o *options) error {
		o.nodeV1alpha4 = enabled
		return nil
	}
}

// NodeV1beta1 explicitly chooses whether the DRA gRPC API v1beta1
// gets enabled.
func NodeV1beta1(enabled bool) Option {
	return func(o *options) error {
		o.nodeV1beta1 = enabled
		return nil
	}
}

// KubeClient grants the plugin access to the API server. This is needed
// for syncing ResourceSlice objects. It's the responsibility of the DRA driver
// developer to ensure that this client has permission to read, write,
// patch and list such objects. It also needs permission to read node objects.
// Ideally, a validating admission policy should be used to limit write
// access to ResourceSlices which belong to the node.
func KubeClient(kubeClient kubernetes.Interface) Option {
	return func(o *options) error {
		o.kubeClient = kubeClient
		return nil
	}
}

// NodeName tells the plugin on which node it is running. This is needed for
// syncing ResourceSlice objects.
func NodeName(nodeName string) Option {
	return func(o *options) error {
		o.nodeName = nodeName
		return nil
	}
}

// NodeUID tells the plugin the UID of the v1.Node object. This is used
// when syncing ResourceSlice objects, but doesn't have to be used. If
// not supplied, the controller will look up the object once.
func NodeUID(nodeUID types.UID) Option {
	return func(o *options) error {
		o.nodeUID = nodeUID
		return nil
	}
}

type options struct {
	logger                     klog.Logger
	grpcVerbosity              int
	driverName                 string
	nodeName                   string
	nodeUID                    types.UID
	draEndpoint                endpoint
	draAddress                 string
	pluginRegistrationEndpoint endpoint
	unaryInterceptors          []grpc.UnaryServerInterceptor
	streamInterceptors         []grpc.StreamServerInterceptor
	kubeClient                 kubernetes.Interface

	nodeV1alpha4 bool
	nodeV1beta1  bool
}

// draPlugin combines the kubelet registration service and the DRA node plugin
// service.
type draPlugin struct {
	// backgroundCtx is for activities that are started later.
	backgroundCtx context.Context
	// cancel cancels the backgroundCtx.
	cancel     func(cause error)
	wg         sync.WaitGroup
	registrar  *nodeRegistrar
	plugin     *grpcServer
	driverName string
	nodeName   string
	nodeUID    types.UID
	kubeClient kubernetes.Interface

	// Information about resource publishing changes concurrently and thus
	// must be protected by the mutex. The controller gets started only
	// if needed.
	mutex                   sync.Mutex
	resourceSliceController *resourceslice.Controller
}

// Start sets up two gRPC servers (one for registration, one for the DRA node
// client). By default, all APIs implemented by the nodeServer get registered.
//
// The context and/or DRAPlugin.Stop can be used to stop all background activity.
// Stop also blocks. A logger can be stored in the context to add values or
// a name to all log entries.
//
// If the plugin will be used to publish resources, [KubeClient] and [NodeName]
// options are mandatory.
//
// The DRA driver decides which gRPC interfaces it implements. At least one
// implementation of [drapbv1alpha4.NodeServer] or [drapbv1beta1.DRAPluginServer]
// is required. Implementing drapbv1beta1.DRAPluginServer is recommended for
// DRA driver targeting Kubernetes >= 1.32. To be compatible with Kubernetes 1.31,
// DRA drivers must implement only [drapbv1alpha4.NodeServer].
func Start(ctx context.Context, nodeServers []interface{}, opts ...Option) (result DRAPlugin, finalErr error) {
	logger := klog.FromContext(ctx)
	o := options{
		logger:        klog.Background(),
		grpcVerbosity: 6, // Logs requests and responses, which can be large.
		nodeV1alpha4:  true,
		nodeV1beta1:   true,
	}
	for _, option := range opts {
		if err := option(&o); err != nil {
			return nil, err
		}
	}

	if o.driverName == "" {
		return nil, errors.New("driver name must be set")
	}
	if o.draAddress == "" {
		return nil, errors.New("DRA address must be set")
	}
	var emptyEndpoint endpoint
	if o.draEndpoint == emptyEndpoint {
		return nil, errors.New("a Unix domain socket path and/or listener must be set for the kubelet plugin")
	}
	if o.pluginRegistrationEndpoint == emptyEndpoint {
		return nil, errors.New("a Unix domain socket path and/or listener must be set for the registrar")
	}

	d := &draPlugin{
		driverName: o.driverName,
		nodeName:   o.nodeName,
		nodeUID:    o.nodeUID,
		kubeClient: o.kubeClient,
	}

	// Stop calls cancel and therefore both cancellation
	// and Stop cause goroutines to stop.
	ctx, cancel := context.WithCancelCause(ctx)
	d.backgroundCtx, d.cancel = ctx, cancel
	logger.V(3).Info("Starting")
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer logger.V(3).Info("Stopping")
		<-ctx.Done()
	}()

	// Clean up if we don't finish succcessfully.
	defer func() {
		if r := recover(); r != nil {
			d.Stop()
			panic(r)
		}
		if finalErr != nil {
			d.Stop()
		}
	}()

	// Run the node plugin gRPC server first to ensure that it is ready.
	var supportedServices []string
	plugin, err := startGRPCServer(klog.NewContext(ctx, klog.LoggerWithName(logger, "dra")), o.grpcVerbosity, o.unaryInterceptors, o.streamInterceptors, o.draEndpoint, func(grpcServer *grpc.Server) {
		for _, nodeServer := range nodeServers {
			if nodeServer, ok := nodeServer.(drapbv1alpha4.NodeServer); ok && o.nodeV1alpha4 {
				logger.V(5).Info("registering v1alpha4.Node gGRPC service")
				drapbv1alpha4.RegisterNodeServer(grpcServer, nodeServer)
				supportedServices = append(supportedServices, drapbv1alpha4.NodeService)
			}
			if nodeServer, ok := nodeServer.(drapbv1beta1.DRAPluginServer); ok && o.nodeV1beta1 {
				logger.V(5).Info("registering v1beta1.DRAPlugin gRPC service")
				drapbv1beta1.RegisterDRAPluginServer(grpcServer, nodeServer)
				supportedServices = append(supportedServices, drapbv1beta1.DRAPluginService)
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("start node client: %v", err)
	}
	d.plugin = plugin
	if len(supportedServices) == 0 {
		return nil, errors.New("no supported DRA gRPC API is implemented and enabled")
	}

	// Backwards compatibility hack: if only the alpha gRPC service is enabled,
	// then we can support registration against a 1.31 kubelet by reporting "1.0.0"
	// as version. That also works with 1.32 because 1.32 supports that legacy
	// behavior and 1.31 works because it doesn't fail while parsing "v1alpha3.Node"
	// as version.
	if len(supportedServices) == 1 && supportedServices[0] == drapbv1alpha4.NodeService {
		supportedServices = []string{"1.0.0"}
	}

	// Now make it available to kubelet.
	registrar, err := startRegistrar(klog.NewContext(ctx, klog.LoggerWithName(logger, "registrar")), o.grpcVerbosity, o.unaryInterceptors, o.streamInterceptors, o.driverName, supportedServices, o.draAddress, o.pluginRegistrationEndpoint)
	if err != nil {
		return nil, fmt.Errorf("start registrar: %v", err)
	}
	d.registrar = registrar

	// startGRPCServer and startRegistrar don't implement cancellation
	// themselves, we add that for both here.
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		<-ctx.Done()

		// Time to stop.
		d.plugin.stop()
		d.registrar.stop()

		// d.resourceSliceController is set concurrently.
		d.mutex.Lock()
		d.resourceSliceController.Stop()
		d.mutex.Unlock()
	}()

	return d, nil
}

// Stop implements [DRAPlugin.Stop].
func (d *draPlugin) Stop() {
	if d == nil {
		return
	}
	d.cancel(errors.New("DRA plugin was stopped"))
	// Wait for goroutines in Start to clean up and exit.
	d.wg.Wait()
}

// PublishResources implements [DRAPlugin.PublishResources]. Returns en error if
// kubeClient or nodeName are unset.
func (d *draPlugin) PublishResources(ctx context.Context, resources Resources) error {
	if d.kubeClient == nil {
		return errors.New("no KubeClient found to publish resources")
	}
	if d.nodeName == "" {
		return errors.New("no NodeName was set to publish resources")
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	owner := resourceslice.Owner{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       d.nodeName,
		UID:        d.nodeUID, // Optional, will be determined by controller if empty.
	}
	driverResources := &resourceslice.DriverResources{
		Pools: map[string]resourceslice.Pool{
			d.nodeName: {
				Slices: []resourceslice.Slice{{
					Devices: resources.Devices,
				}},
			},
		},
	}
	if d.resourceSliceController == nil {
		// Start publishing the information. The controller is using
		// our background context, not the one passed into this
		// function, and thus is connected to the lifecycle of the
		// plugin.
		controllerCtx := d.backgroundCtx
		controllerLogger := klog.FromContext(controllerCtx)
		controllerLogger = klog.LoggerWithName(controllerLogger, "ResourceSlice controller")
		controllerCtx = klog.NewContext(controllerCtx, controllerLogger)
		var err error
		if d.resourceSliceController, err = resourceslice.StartController(controllerCtx,
			resourceslice.Options{
				DriverName: d.driverName,
				KubeClient: d.kubeClient,
				Owner:      &owner,
				Resources:  driverResources,
			}); err != nil {
			return fmt.Errorf("start ResourceSlice controller: %w", err)
		}
		return nil
	}

	// Inform running controller about new information.
	d.resourceSliceController.Update(driverResources)

	return nil
}

// RegistrationStatus implements [DRAPlugin.RegistrationStatus].
func (d *draPlugin) RegistrationStatus() *registerapi.RegistrationStatus {
	if d.registrar == nil {
		return nil
	}
	return d.registrar.status
}

func (d *draPlugin) internal() {}
