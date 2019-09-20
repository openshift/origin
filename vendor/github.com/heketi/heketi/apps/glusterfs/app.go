//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/rest"
	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/executors/injectexec"
	"github.com/heketi/heketi/executors/kubeexec"
	"github.com/heketi/heketi/executors/mockexec"
	"github.com/heketi/heketi/executors/sshexec"
	"github.com/heketi/heketi/pkg/logging"
)

const (
	ASYNC_ROUTE                    = "/queue"
	BOLTDB_BUCKET_CLUSTER          = "CLUSTER"
	BOLTDB_BUCKET_NODE             = "NODE"
	BOLTDB_BUCKET_VOLUME           = "VOLUME"
	BOLTDB_BUCKET_DEVICE           = "DEVICE"
	BOLTDB_BUCKET_BRICK            = "BRICK"
	BOLTDB_BUCKET_BLOCKVOLUME      = "BLOCKVOLUME"
	BOLTDB_BUCKET_DBATTRIBUTE      = "DBATTRIBUTE"
	DB_CLUSTER_HAS_FILE_BLOCK_FLAG = "DB_CLUSTER_HAS_FILE_BLOCK_FLAG"
	DB_BRICK_HAS_SUBTYPE_FIELD     = "DB_BRICK_HAS_SUBTYPE_FIELD"
	DEFAULT_OP_LIMIT               = 8
)

var (
	logger     = logging.NewLogger("[heketi]", logging.LEVEL_INFO)
	dbfilename = "heketi.db"
	// global var to track active node health cache
	// if multiple apps are started the content of this var is
	// undefined.
	// TODO: make a global not needed
	currentNodeHealthCache *NodeHealthCache

	// global var to enable the use of the health cache + monitor
	// when the GlusterFS App is created. This is mildly hacky but
	// avoids having to update config files to enable the feature
	// while avoiding having to touch all of the unit tests.
	MonitorGlusterNodes = false

	// global var to enable the use of the background pending
	// operations cleanup mechanism.
	EnableBackgroundCleaner = false

	// global var that contains list of volume options that are set *before*
	// setting the volume options that come as part of volume request.
	PreReqVolumeOptions = ""

	// global var that contains list of volume options that are set *after*
	// setting the volume options that come as part of volume request.
	PostReqVolumeOptions = ""
)

type App struct {
	asyncManager *rest.AsyncHttpManager
	db           *bolt.DB
	dbReadOnly   bool
	executor     executors.Executor
	_allocator   Allocator
	conf         *GlusterFSConfig

	// health monitor
	nhealth *NodeHealthCache
	// background operations cleaner
	bgcleaner *backgroundOperationCleaner

	// operations tracker
	optracker *OpTracker

	// For testing only.  Keep access to the object
	// not through the interface
	xo *mockexec.MockExecutor
}

// NewApp constructs a new glusterfs application object and populates
// the internal structures according to the passed configuration (
// and environment). If an error occurs the app object will be nil
// and the error type will be populated.
func NewApp(conf *GlusterFSConfig) (*App, error) {
	app := &App{}
	err := app.setup(conf)
	if err != nil {
		return nil, err
	}
	return app, nil
}

// setup fills in the internal types of the app based on
// the configuration.
func (app *App) setup(conf *GlusterFSConfig) error {
	var err error

	app.conf = conf

	// We would like to perform rebalance by default
	// As it is very difficult to distinguish missing parameter from
	// set-but-false parameter in json, we are going to ignore json config
	// We will provide a env method to set it to false again.
	app.conf.KubeConfig.RebalanceOnExpansion = true
	app.conf.SshConfig.RebalanceOnExpansion = true

	// Set values mentioned in environmental variable
	app.setFromEnvironmentalVariable()

	// Setup loglevel
	err = SetLogLevel(app.conf.Loglevel)
	if err != nil {
		// just log that the log level was bad, it never failed
		// anything in previous versions
		logger.Err(err)
	}

	// Setup asynchronous manager
	app.asyncManager = rest.NewAsyncHttpManager(ASYNC_ROUTE)

	// Setup executor
	switch app.conf.Executor {
	case "mock":
		app.xo, err = mockexec.NewMockExecutor()
		app.executor = app.xo
	case "kube", "kubernetes":
		app.executor, err = kubeexec.NewKubeExecutor(&app.conf.KubeConfig)
	case "ssh", "":
		app.executor, err = sshexec.NewSshExecutor(&app.conf.SshConfig)
	case "inject/ssh":
		app.executor, err = sshexec.NewSshExecutor(&app.conf.SshConfig)
		app.executor = injectexec.NewInjectExecutor(
			app.executor, &app.conf.InjectConfig)
	case "inject/mock":
		app.executor, err = mockexec.NewMockExecutor()
		app.executor = injectexec.NewInjectExecutor(
			app.executor, &app.conf.InjectConfig)
	default:
		return fmt.Errorf("invalid executor: %v", app.conf.Executor)
	}
	if err != nil {
		logger.Err(err)
		return err
	}
	logger.Info("Loaded %v executor", app.conf.Executor)

	// Set db is set in the configuration file
	if app.conf.DBfile != "" {
		dbfilename = app.conf.DBfile
	}

	err = app.initDB()
	if err != nil {
		logger.Err(err)
		return err
	}

	// Drop a note that the system had pending operations in the db
	// at start up time. Even though we now have auto-cleanup
	// This note can be helpful for curious users and or a debugging
	// hint for changes to the environment over time.
	if HasPendingOperations(app.db) {
		logger.Warning(
			"Heketi has existing pending operations in the db." +
				" Heketi will attempt to automatically clean up these items." +
				" See the Heketi troubleshooting docs for more information" +
				" about managing pending operations.")
	}

	// Set advanced settings
	app.setAdvSettings()

	// Set block settings
	app.setBlockSettings()

	// initialize sub-objects and background tasks
	app.initOpTracker()
	app.initNodeMonitor()
	app.initBackgroundCleaner()

	// Show application has loaded
	logger.Info("GlusterFS Application Loaded")

	return nil
}

func (app *App) initDB() error {
	// Setup database
	var err error
	app.db, err = OpenDB(dbfilename, false)
	if err != nil {
		logger.LogError("Unable to open database: %v. Retrying using read only mode", err)

		// Try opening as read-only
		app.db, err = OpenDB(dbfilename, true)
		if err != nil {
			return logger.LogError("Unable to open database: %v", err)
		}
		app.dbReadOnly = true
	} else {
		err = app.db.Update(func(tx *bolt.Tx) error {
			err := initializeBuckets(tx)
			if err != nil {
				return logger.LogError("Unable to initialize buckets: %v", err)
			}

			// Check that this is db we can safely use
			validAttributes := validDbAttributeKeys(tx, mapDbAtrributeKeys())
			if !validAttributes {
				return logger.LogError(
					"Unable to initialize db, unknown attributes are present" +
						" (db from a newer version of heketi?)")
			}

			// Handle Upgrade Changes
			err = UpgradeDB(tx)
			if err != nil {
				return logger.LogError("Unable to Upgrade DB: %v", err)
			}

			return nil
		})
	}
	return err
}

func (app *App) initNodeMonitor() {
	//default monitor gluster node refresh time
	var timer uint32 = 120
	var startDelay uint32 = 10
	if app.conf.RefreshTimeMonitorGlusterNodes > 0 {
		timer = app.conf.RefreshTimeMonitorGlusterNodes
	}
	if app.conf.StartTimeMonitorGlusterNodes > 0 {
		startDelay = app.conf.StartTimeMonitorGlusterNodes
	}
	if MonitorGlusterNodes {
		app.nhealth = NewNodeHealthCache(timer, startDelay, app.db, app.executor)
		app.nhealth.Monitor()
		currentNodeHealthCache = app.nhealth
	}
}

func (app *App) initBackgroundCleaner() {
	// configure background cleaner params
	if app.conf.StartTimeBackgroundCleaner == 0 {
		app.conf.StartTimeBackgroundCleaner = 60
	}
	if app.conf.RefreshTimeBackgroundCleaner == 0 {
		app.conf.RefreshTimeBackgroundCleaner = 3600
	}
	if EnableBackgroundCleaner {
		app.bgcleaner = app.BackgroundCleaner()
		app.bgcleaner.Start()
	}
}

func (app *App) initOpTracker() {
	oplimit := app.conf.MaxInflightOperations
	if oplimit == 0 {
		oplimit = DEFAULT_OP_LIMIT
	}
	app.optracker = newOpTracker(oplimit)
}

func SetLogLevel(level string) error {
	switch level {
	case "none":
		logger.SetLevel(logging.LEVEL_NOLOG)
	case "critical":
		logger.SetLevel(logging.LEVEL_CRITICAL)
	case "error":
		logger.SetLevel(logging.LEVEL_ERROR)
	case "warning":
		logger.SetLevel(logging.LEVEL_WARNING)
	case "info":
		logger.SetLevel(logging.LEVEL_INFO)
	case "debug":
		logger.SetLevel(logging.LEVEL_DEBUG)
	case "":
		// treat empty string as a no-op & don't complain
		// about it
	default:
		return fmt.Errorf("invalid log level: %s", level)
	}
	return nil
}

func (a *App) setFromEnvironmentalVariable() {
	var err error

	// environment variable overrides file config
	env := os.Getenv("HEKETI_EXECUTOR")
	if env != "" {
		a.conf.Executor = env
	}

	env = os.Getenv("HEKETI_DB_PATH")
	if env != "" {
		a.conf.DBfile = env
	}

	env = os.Getenv("HEKETI_GLUSTERAPP_LOGLEVEL")
	if env != "" {
		a.conf.Loglevel = env
	}

	env = os.Getenv("HEKETI_AUTO_CREATE_BLOCK_HOSTING_VOLUME")
	if "" != env {
		a.conf.CreateBlockHostingVolumes, err = strconv.ParseBool(env)
		if err != nil {
			logger.LogError("Error: Parse bool in Create Block Hosting Volumes: %v", err)
		}
	}

	env = os.Getenv("HEKETI_BLOCK_HOSTING_VOLUME_SIZE")
	if "" != env {
		a.conf.BlockHostingVolumeSize, err = strconv.Atoi(env)
		if err != nil {
			logger.LogError("Error: Atoi in Block Hosting Volume Size: %v", err)
		}
	}

	env = os.Getenv("HEKETI_BLOCK_HOSTING_VOLUME_OPTIONS")
	if "" != env {
		a.conf.BlockHostingVolumeOptions = env
	}

	env = os.Getenv("HEKETI_GLUSTERAPP_REBALANCE_ON_EXPANSION")
	if env != "" {
		value, err := strconv.ParseBool(env)
		if err != nil {
			logger.LogError("Error: While parsing HEKETI_GLUSTERAPP_REBALANCE_ON_EXPANSION as bool: %v", err)
		} else {
			a.conf.SshConfig.RebalanceOnExpansion = value
			a.conf.KubeConfig.RebalanceOnExpansion = value
		}
	}

	env = os.Getenv("HEKETI_MAX_INFLIGHT_OPERATIONS")
	if env != "" {
		value, err := strconv.ParseInt(env, 10, 64)
		if err != nil {
			logger.LogError("Error: While parsing HEKETI_MAX_INFLIGHT_OPERATIONS: %v", err)
		} else {
			a.conf.MaxInflightOperations = uint64(value)
		}
	}

	env = os.Getenv("HEKETI_PRE_REQUEST_VOLUME_OPTIONS")
	if "" != env {
		a.conf.PreReqVolumeOptions = env
	}

	env = os.Getenv("HEKETI_POST_REQUEST_VOLUME_OPTIONS")
	if "" != env {
		a.conf.PostReqVolumeOptions = env
	}

	env = os.Getenv("HEKETI_ZONE_CHECKING")
	if "" != env {
		a.conf.ZoneChecking = env
	}

	env = os.Getenv("HEKETI_GLUSTER_MAX_VOLUMES_PER_CLUSTER")
	if env != "" {
		a.conf.MaxVolumesPerCluster, err = strconv.Atoi(env)
		if err != nil {
			logger.LogError("Error: While parsing HEKETI_GLUSTER_MAX_VOLUMES_PER_CLUSTER: %v", err)
		}
	}
}

func (a *App) setAdvSettings() {
	if a.conf.BrickMaxNum != 0 {
		logger.Info("Adv: Max bricks per volume set to %v", a.conf.BrickMaxNum)

		// From volume_entry.go
		BrickMaxNum = a.conf.BrickMaxNum
	}
	if a.conf.BrickMaxSize != 0 {
		logger.Info("Adv: Max brick size %v GB", a.conf.BrickMaxSize)

		// From volume_entry.go
		// Convert to KB
		BrickMaxSize = uint64(a.conf.BrickMaxSize) * 1024 * 1024
	}
	if a.conf.BrickMinSize != 0 {
		logger.Info("Adv: Min brick size %v GB", a.conf.BrickMinSize)

		// From volume_entry.go
		// Convert to KB
		BrickMinSize = uint64(a.conf.BrickMinSize) * 1024 * 1024
	}
	if a.conf.AverageFileSize != 0 {
		logger.Info("Average file size on volumes set to %v KiB", a.conf.AverageFileSize)
		averageFileSize = a.conf.AverageFileSize
	}
	if a.conf.PreReqVolumeOptions != "" {
		logger.Info("Pre Request Volume Options: %v", a.conf.PreReqVolumeOptions)
		PreReqVolumeOptions = a.conf.PreReqVolumeOptions
	}
	if a.conf.PostReqVolumeOptions != "" {
		logger.Info("Post Request Volume Options: %v", a.conf.PostReqVolumeOptions)
		PostReqVolumeOptions = a.conf.PostReqVolumeOptions
	}
	if a.conf.ZoneChecking != "" {
		logger.Info("Zone checking: '%v'", a.conf.ZoneChecking)
		ZoneChecking = ZoneCheckingStrategy(a.conf.ZoneChecking)
	}
	if a.conf.MaxVolumesPerCluster < 0 {
		logger.Info("Volumes per cluster limit is removed as it is set to %v", a.conf.MaxVolumesPerCluster)
		maxVolumesPerCluster = math.MaxInt32
	} else if a.conf.MaxVolumesPerCluster == 0 {
		logger.Info("Volumes per cluster limit is set to default value of %v", maxVolumesPerCluster)
	} else {
		logger.Info("Volumes per cluster limit is set to %v", a.conf.MaxVolumesPerCluster)
		maxVolumesPerCluster = a.conf.MaxVolumesPerCluster
	}
}

func (a *App) setBlockSettings() {
	if a.conf.CreateBlockHostingVolumes != false {
		logger.Info("Block: Auto Create Block Hosting Volume set to %v", a.conf.CreateBlockHostingVolumes)

		// switch to auto creation of block hosting volumes
		CreateBlockHostingVolumes = a.conf.CreateBlockHostingVolumes
	}
	if a.conf.BlockHostingVolumeSize > 0 {
		logger.Info("Block: New Block Hosting Volume size %v GB", a.conf.BlockHostingVolumeSize)

		// Should be in GB as this is input for block hosting volume create
		BlockHostingVolumeSize = a.conf.BlockHostingVolumeSize
	}
	if a.conf.BlockHostingVolumeOptions != "" {
		logger.Info("Block: New Block Hosting Volume Options: %v", a.conf.BlockHostingVolumeOptions)
		BlockHostingVolumeOptions = a.conf.BlockHostingVolumeOptions
	}

}

// Register Routes
func (a *App) SetRoutes(router *mux.Router) error {

	routes := rest.Routes{

		// Asynchronous Manager
		rest.Route{
			Name:        "Async",
			Method:      "GET",
			Pattern:     ASYNC_ROUTE + "/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.asyncManager.HandlerStatus},

		// Cluster
		rest.Route{
			Name:        "ClusterCreate",
			Method:      "POST",
			Pattern:     "/clusters",
			HandlerFunc: a.ClusterCreate},
		rest.Route{
			Name:        "ClusterSetFlags",
			Method:      "POST",
			Pattern:     "/clusters/{id:[A-Fa-f0-9]+}/flags",
			HandlerFunc: a.ClusterSetFlags},
		rest.Route{
			Name:        "ClusterInfo",
			Method:      "GET",
			Pattern:     "/clusters/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.ClusterInfo},
		rest.Route{
			Name:        "ClusterList",
			Method:      "GET",
			Pattern:     "/clusters",
			HandlerFunc: a.ClusterList},
		rest.Route{
			Name:        "ClusterDelete",
			Method:      "DELETE",
			Pattern:     "/clusters/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.ClusterDelete},

		// Node
		rest.Route{
			Name:        "NodeAdd",
			Method:      "POST",
			Pattern:     "/nodes",
			HandlerFunc: a.NodeAdd},
		rest.Route{
			Name:        "NodeInfo",
			Method:      "GET",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NodeInfo},
		rest.Route{
			Name:        "NodeDelete",
			Method:      "DELETE",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.NodeDelete},
		rest.Route{
			Name:        "NodeSetState",
			Method:      "POST",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}/state",
			HandlerFunc: a.NodeSetState},
		rest.Route{
			Name:        "NodeSetTags",
			Method:      "POST",
			Pattern:     "/nodes/{id:[A-Fa-f0-9]+}/tags",
			HandlerFunc: a.NodeSetTags},

		// Devices
		rest.Route{
			Name:        "DeviceAdd",
			Method:      "POST",
			Pattern:     "/devices",
			HandlerFunc: a.DeviceAdd},
		rest.Route{
			Name:        "DeviceInfo",
			Method:      "GET",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.DeviceInfo},
		rest.Route{
			Name:        "DeviceDelete",
			Method:      "DELETE",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.DeviceDelete},
		rest.Route{
			Name:        "DeviceSetState",
			Method:      "POST",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}/state",
			HandlerFunc: a.DeviceSetState},
		rest.Route{
			Name:        "DeviceResync",
			Method:      "GET",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}/resync",
			HandlerFunc: a.DeviceResync},
		rest.Route{
			Name:        "DeviceSetTags",
			Method:      "POST",
			Pattern:     "/devices/{id:[A-Fa-f0-9]+}/tags",
			HandlerFunc: a.DeviceSetTags},

		// Volume
		rest.Route{
			Name:        "VolumeCreate",
			Method:      "POST",
			Pattern:     "/volumes",
			HandlerFunc: a.VolumeCreate},
		rest.Route{
			Name:        "VolumeInfo",
			Method:      "GET",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.VolumeInfo},
		rest.Route{
			Name:        "VolumeExpand",
			Method:      "POST",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}/expand",
			HandlerFunc: a.VolumeExpand},
		rest.Route{
			Name:        "VolumeDelete",
			Method:      "DELETE",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.VolumeDelete},
		rest.Route{
			Name:        "VolumeList",
			Method:      "GET",
			Pattern:     "/volumes",
			HandlerFunc: a.VolumeList},
		rest.Route{
			Name:        "VolumeSetBlockRestriction",
			Method:      "POST",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}/block-restriction",
			HandlerFunc: a.VolumeSetBlockRestriction},

		// Volume Cloning
		rest.Route{
			Name:        "VolumeClone",
			Method:      "POST",
			Pattern:     "/volumes/{id:[A-Fa-f0-9]+}/clone",
			HandlerFunc: a.VolumeClone},

		// BlockVolumes
		rest.Route{
			Name:        "BlockVolumeCreate",
			Method:      "POST",
			Pattern:     "/blockvolumes",
			HandlerFunc: a.BlockVolumeCreate},
		rest.Route{
			Name:        "BlockVolumeInfo",
			Method:      "GET",
			Pattern:     "/blockvolumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.BlockVolumeInfo},
		rest.Route{
			Name:        "BlockVolumeDelete",
			Method:      "DELETE",
			Pattern:     "/blockvolumes/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.BlockVolumeDelete},
		rest.Route{
			Name:        "BlockVolumeList",
			Method:      "GET",
			Pattern:     "/blockvolumes",
			HandlerFunc: a.BlockVolumeList},

		// Backup
		rest.Route{
			Name:        "Backup",
			Method:      "GET",
			Pattern:     "/backup/db",
			HandlerFunc: a.Backup},

		// Db
		rest.Route{
			Name:        "DbDump",
			Method:      "GET",
			Pattern:     "/db/dump",
			HandlerFunc: a.DbDump},
		rest.Route{
			Name:        "DbCheck",
			Method:      "GET",
			Pattern:     "/db/check",
			HandlerFunc: a.DbCheck},

		// Logging
		rest.Route{
			Name:        "GetLogLevel",
			Method:      "GET",
			Pattern:     "/internal/logging",
			HandlerFunc: a.GetLogLevel},
		rest.Route{
			Name:        "SetLogLevel",
			Method:      "POST",
			Pattern:     "/internal/logging",
			HandlerFunc: a.SetLogLevel},
		// Operations state on server
		rest.Route{
			Name:        "OperationsInfo",
			Method:      "GET",
			Pattern:     "/operations",
			HandlerFunc: a.OperationsInfo},
		// list of pending operations in db
		rest.Route{
			Name:        "PendingOperationList",
			Method:      "GET",
			Pattern:     "/operations/pending",
			HandlerFunc: a.PendingOperationList},
		// details about a specific operation
		rest.Route{
			Name:        "PendingOperationDetails",
			Method:      "GET",
			Pattern:     "/operations/pending/{id:[A-Fa-f0-9]+}",
			HandlerFunc: a.PendingOperationDetails},
		// request operation clean up
		rest.Route{
			Name:        "PendingOperationCleanUp",
			Method:      "POST",
			Pattern:     "/operations/pending/cleanup",
			HandlerFunc: a.PendingOperationCleanUp},

		// State examination
		rest.Route{
			Name:        "ExamineGluster",
			Method:      "GET",
			Pattern:     "/internal/state/examine/gluster",
			HandlerFunc: a.ExamineGluster},
	}

	// Register all routes from the App
	for _, route := range routes {

		// Add routes from the table
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)

	}

	// Set default error handler
	router.NotFoundHandler = http.HandlerFunc(a.NotFoundHandler)

	return nil
}

func (a *App) Close() {
	// stop the health goroutine
	if a.nhealth != nil {
		a.nhealth.Stop()
	}
	if a.bgcleaner != nil {
		a.bgcleaner.Stop()
	}

	// Close the DB
	a.db.Close()
	logger.Info("Closed")
}

func (a *App) Backup(w http.ResponseWriter, r *http.Request) {
	err := a.db.View(func(tx *bolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="heketi.db"`)
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	logger.Warning("Invalid path or request %v", r.URL.Path)
	http.Error(w, "Invalid path or request", http.StatusNotFound)
}

// ServerReset resets the app and its components to the state desired
// after the server process has restarted. The intent of this function
// is to perform cleanup & reset tasks that are needed by the server
// process only (should not be used by other callers of the app).
// This should be as part of the start-up of the server instance.
func (a *App) ServerReset() error {
	// currently this code just resets the operations in the db
	// to stale
	return a.db.Update(func(tx *bolt.Tx) error {
		if err := MarkPendingOperationsStale(tx); err != nil {
			logger.LogError("failed to mark operations stale: %v", err)
			return err
		}
		return nil
	})
}

// OfflineCleaner returns an operations cleaner based on the current
// app object that can be used to perform an offline cleanup.
// An offline cleanup assumes that the binary is only doing cleanups
// and nothing else.
func (a *App) OfflineCleaner() OperationCleaner {
	return OperationCleaner{
		db:       a.db,
		executor: a.executor,
		sel:      CleanAll,
	}
}

// OfflineExaminer returns an examiner based on the current
// app object that can be used to perform an offline examination.
// An offline examiner assumes that the binary is only doing examination of the
// state and nothing else.
func (a *App) OfflineExaminer() Examiner {
	return Examiner{
		db:       a.db,
		executor: a.executor,
		mode:     OfflineExaminer,
	}
}

// OnDemandCleaner returns an operations cleaner based on the current
// app object that can be used to perform clean ups requested by
// a user (on demand).
func (a *App) OnDemandCleaner(ops map[string]bool) OperationCleaner {
	sel := CleanAll
	if len(ops) > 0 {
		// user specified specific ops to clean
		sel = CleanSelectedOps(ops)
	}
	return OperationCleaner{
		db:        a.db,
		executor:  a.executor,
		sel:       sel,
		optracker: a.optracker,
		opClass:   TrackNormal,
	}
}

// OnDemandExaminer returns an examiner based on the current
// app object that can be used to examine state on user demand.
func (a *App) OnDemandExaminer() Examiner {
	return Examiner{
		db:        a.db,
		executor:  a.executor,
		optracker: a.optracker,
		mode:      OnDemandExaminer,
	}
}

// BackgroundCleaner returns a background operations cleaner
// suitable for use as a background "process" in the heketi server.
func (a *App) BackgroundCleaner() *backgroundOperationCleaner {
	godbc.Require(a.optracker != nil)
	startSec := time.Duration(a.conf.StartTimeBackgroundCleaner)
	checkSec := time.Duration(a.conf.RefreshTimeBackgroundCleaner)
	return &backgroundOperationCleaner{
		cleaner: OperationCleaner{
			db:        a.db,
			executor:  a.executor,
			sel:       CleanAll,
			optracker: a.optracker,
			opClass:   TrackClean,
		},
		StartInterval: startSec * time.Second,
		CheckInterval: checkSec * time.Second,
	}
}

// currentNodeHealthStatus returns a map of node ids to the most
// recently known health status (true is up, false is not up).
// If a node is not found in the map its status is unknown.
// If no heath monitor is active an empty map is always returned.
func currentNodeHealthStatus() (nodeUp map[string]bool) {
	if currentNodeHealthCache != nil {
		nodeUp = currentNodeHealthCache.Status()
	} else {
		// just an empty map
		nodeUp = map[string]bool{}
	}
	return
}
