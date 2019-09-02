//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package main

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/urfave/negroni"
	restclient "k8s.io/client-go/rest"

	"github.com/heketi/heketi/apps/glusterfs"
	"github.com/heketi/heketi/middleware"
	"github.com/heketi/heketi/pkg/metrics"
	"github.com/heketi/heketi/server/admin"
	"github.com/heketi/heketi/server/config"
	"github.com/heketi/heketi/server/profiling"
)

var (
	HEKETI_VERSION               = "(dev)"
	configfile                   string
	showVersion                  bool
	jsonFile                     string
	dbFile                       string
	debugOutput                  bool
	deleteAllBricksWithEmptyPath bool
	dryRun                       bool
	force                        bool
)

var RootCmd = &cobra.Command{
	Use:     "heketi",
	Short:   "Heketi is a restful volume management server",
	Long:    "Heketi is a restful volume management server",
	Example: "heketi --config=/config/file/path/",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Heketi %v\n", HEKETI_VERSION)
		if !showVersion {
			// Check configuration file was given
			if configfile == "" {
				fmt.Fprintln(os.Stderr, "Please provide configuration file")
				os.Exit(1)
			}
		} else {
			// Quit here if all we needed to do was show version
			os.Exit(0)

		}
	},
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "heketi db management",
	Long:  "heketi db management",
}

var offlineCmd = &cobra.Command{
	Use:   "offline",
	Short: "perform offline operations",
	Long:  "perform offline operations",
}

var importdbCmd = &cobra.Command{
	Use:     "import",
	Short:   "import creates a db file from JSON input",
	Long:    "import creates a db file from JSON input",
	Example: "heketi import db --jsonfile=/json/file/path/ --dbfile=/db/file/path/",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonFile == "" {
			fmt.Fprintln(os.Stderr, "Please provide file for input")
			os.Exit(1)
		}
		if dbFile == "" {
			fmt.Fprintln(os.Stderr, "Please provide path for db file")
			os.Exit(1)
		}
		if debugOutput {
			glusterfs.SetLogLevel("debug")
		}
		err := glusterfs.DbCreate(jsonFile, dbFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "db creation failed: %v\n", err.Error())
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "DB imported to", dbFile)
		os.Exit(0)
	},
}

var exportdbCmd = &cobra.Command{
	Use:     "export",
	Short:   "export creates a JSON file from a db file",
	Long:    "export creates a JSON file from a db file",
	Example: "heketi db export --jsonfile=/json/file/path/ --dbfile=/db/file/path/",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonFile == "" {
			fmt.Fprintln(os.Stderr, "Please provide file for input")
			os.Exit(1)
		}
		if dbFile == "" {
			fmt.Fprintln(os.Stderr, "Please provide path for db file")
			os.Exit(1)
		}
		if debugOutput {
			glusterfs.SetLogLevel("debug")
		}
		err := glusterfs.DbDump(jsonFile, dbFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to dump db: %v\n", err.Error())
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "DB exported to", jsonFile)
		os.Exit(0)
	},
}

var checkdbCmd = &cobra.Command{
	Use:     "consistency-check",
	Short:   "checks the db for inconsistencies",
	Long:    "checks the db for inconsistencies",
	Example: "heketi db consistency-check --dbfile=/db/file/path/",
	Run: func(cmd *cobra.Command, args []string) {
		if dbFile == "" {
			fmt.Fprintln(os.Stderr, "Please provide path for db file")
			os.Exit(1)
		}
		if debugOutput {
			glusterfs.SetLogLevel("debug")
		}
		err := glusterfs.DbCheck(dbFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to check db: %v\n", err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	},
}

var deleteBricksWithEmptyPath = &cobra.Command{
	Use:     "delete-bricks-with-empty-path",
	Short:   "removes brick entries from db that have empty path",
	Long:    "removes brick entries from db that have empty path",
	Example: "heketi db delete-bricks-with-empty-path --dbfile=/db/file/path/",
	Run: func(cmd *cobra.Command, args []string) {
		var clusterlist []string
		var nodelist []string
		var devicelist []string

		clusterlist, err := cmd.Flags().GetStringSlice("clusters")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to get flags %v\n", err)
			os.Exit(1)
		}
		nodelist, err = cmd.Flags().GetStringSlice("nodes")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to get flags %v\n", err)
			os.Exit(1)
		}
		devicelist, err = cmd.Flags().GetStringSlice("devices")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to get flags %v\n", err)
			os.Exit(1)
		}

		if len(clusterlist) == 0 &&
			len(nodelist) == 0 &&
			len(devicelist) == 0 &&
			deleteAllBricksWithEmptyPath == false {
			fmt.Fprintf(os.Stderr, "neither --all flag nor list of clusters/nodes/devices is given\n")
			os.Exit(1)
		}
		if debugOutput {
			glusterfs.SetLogLevel("debug")
		}
		db, err := glusterfs.OpenDB(dbFile, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to open database: %v\n", err)
			os.Exit(1)
		}
		err = glusterfs.DeleteBricksWithEmptyPath(db, deleteAllBricksWithEmptyPath, clusterlist, nodelist, devicelist)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to delete bricks with empty path: %v\n", err.Error())
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "bricks with empty path removed", jsonFile)
		os.Exit(0)
	},
}

var cleanupOperationsCmd = &cobra.Command{
	Use:     "cleanup-operations",
	Short:   "clean up all pending operations stored in heketi db",
	Long:    "clean up all pending operations stored in heketi db",
	Example: "heketi offline cleanup-operations --config=heketi.json",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "OFFLINE COMMAND: Clean All Pending Operations\n")
		if configfile == "" {
			fmt.Fprintf(os.Stderr, "Configuration file is required\n")
			os.Exit(1)
		}

		// Read configuration
		c, err := config.ReadConfig(configfile)
		if err != nil {
			os.Exit(1)
		}

		randSeed()
		// option to not start the background node monitor?
		// FIXME, this is a hacky way to disable this background activity
		// c.GlusterFS.DisableMonitorGlusterNodes = true
		// Never start the background cleaner when running
		// an offline cleanup
		c.GlusterFS.DisableBackgroundCleaner = true
		app := setupApp(c)

		// run the operation cleanup in the foreground (offline mode)
		fmt.Fprintf(os.Stderr, "Starting clean now...\n")
		err = app.OfflineCleaner().Clean()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error cleaning operations: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	},
}

var stateExamineCmd = &cobra.Command{
	Use:   "examine",
	Short: "compare heketi database with real state of backend/user systems",
	Long:  "compare heketi database with real state of backend/user systems",
}

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "view or modify heketi's state",
	Long:  "view or modify heketi's state",
}

var examineGlusterCmd = &cobra.Command{
	Use:     "gluster",
	Short:   "compare heketi database with state of Gluster",
	Long:    "compare heketi database with state of Gluster",
	Example: "heketi offline state examine gluster --config=keti.json",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "OFFLINE COMMAND: Examine state of Gluster\n")
		if configfile == "" {
			fmt.Fprintf(os.Stderr, "Configuration file is required\n")
			os.Exit(1)
		}

		// Read configuration
		c, err := config.ReadConfig(configfile)
		if err != nil {
			os.Exit(1)
		}

		randSeed()
		// option to not start the background node monitor?
		// FIXME, this is a hacky way to disable this background activity
		// c.GlusterFS.DisableMonitorGlusterNodes = true
		// Never start the background cleaner when running
		// an offline cleanup
		c.GlusterFS.DisableBackgroundCleaner = true
		app := setupApp(c)

		fmt.Fprintf(os.Stdout, "Starting examiner now...\n")
		response, err := app.OfflineExaminer().ExamineGluster()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to examine Gluster: %v\n", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "    ")

		fmt.Fprintf(os.Stdout, "EXAMINER_OUTPUT_STARTS\n")
		if err := enc.Encode(response); err != nil {
			fmt.Fprintf(os.Stderr, "Could not encode dump as JSON: %v", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "EXAMINER_OUTPUT_ENDS\n")
		os.Exit(0)
	},
}

func init() {
	RootCmd.Flags().StringVar(&configfile, "config", "", "Configuration file")
	RootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version")
	RootCmd.SilenceUsage = true

	RootCmd.AddCommand(dbCmd)
	dbCmd.SilenceUsage = true

	dbCmd.AddCommand(importdbCmd)
	importdbCmd.Flags().StringVar(&jsonFile, "jsonfile", "", "Input file with data in JSON format")
	importdbCmd.Flags().StringVar(&dbFile, "dbfile", "", "File path for db to be created")
	importdbCmd.Flags().BoolVar(&debugOutput, "debug", false, "Show debug logs on stdout")
	importdbCmd.SilenceUsage = true

	dbCmd.AddCommand(exportdbCmd)
	exportdbCmd.Flags().StringVar(&dbFile, "dbfile", "", "File path for db to be exported")
	exportdbCmd.Flags().StringVar(&jsonFile, "jsonfile", "", "File path for JSON file to be created")
	exportdbCmd.Flags().BoolVar(&debugOutput, "debug", false, "Show debug logs on stdout")
	exportdbCmd.SilenceUsage = true

	dbCmd.AddCommand(checkdbCmd)
	checkdbCmd.Flags().StringVar(&dbFile, "dbfile", "", "File path for db to be exported")
	checkdbCmd.Flags().BoolVar(&debugOutput, "debug", false, "Show debug logs on stdout")
	checkdbCmd.SilenceUsage = true

	dbCmd.AddCommand(deleteBricksWithEmptyPath)
	deleteBricksWithEmptyPath.Flags().StringVar(&dbFile, "dbfile", "", "File path for db to operate on")
	deleteBricksWithEmptyPath.Flags().BoolVar(&debugOutput, "debug", false, "Show debug logs on stdout")
	deleteBricksWithEmptyPath.Flags().BoolVar(&deleteAllBricksWithEmptyPath, "all", false, "if set true, then all bricks with empty path are removed")
	deleteBricksWithEmptyPath.Flags().StringSlice("clusters", []string{}, "comma separated list of cluster IDs")
	deleteBricksWithEmptyPath.Flags().StringSlice("nodes", []string{}, "comma separated list of node IDs")
	deleteBricksWithEmptyPath.Flags().StringSlice("devices", []string{}, "comma separated list of device IDs")
	deleteBricksWithEmptyPath.SilenceUsage = true

	RootCmd.AddCommand(offlineCmd)
	offlineCmd.SilenceUsage = true

	offlineCmd.AddCommand(cleanupOperationsCmd)
	cleanupOperationsCmd.SilenceUsage = true
	cleanupOperationsCmd.Flags().StringVar(&configfile, "config", "", "Configuration file")

	offlineCmd.AddCommand(stateCmd)
	stateCmd.SilenceUsage = true
	stateCmd.AddCommand(stateExamineCmd)
	stateExamineCmd.SilenceUsage = true
	stateExamineCmd.AddCommand(examineGlusterCmd)
	examineGlusterCmd.SilenceUsage = true
	examineGlusterCmd.Flags().StringVar(&configfile, "config", "", "Configuration file")

}

func setWithEnvVariables(options *config.Config) {
	// Check for user key
	env := os.Getenv("HEKETI_USER_KEY")
	if "" != env {
		options.AuthEnabled = true
		options.JwtConfig.User.PrivateKey = env
	}

	// Check for user key
	env = os.Getenv("HEKETI_ADMIN_KEY")
	if "" != env {
		options.AuthEnabled = true
		options.JwtConfig.Admin.PrivateKey = env
	}

	// Check for user key
	env = os.Getenv("HEKETI_HTTP_PORT")
	if "" != env {
		options.Port = env
	}

	env = os.Getenv("HEKETI_BACKUP_DB_TO_KUBE_SECRET")
	if "" != env {
		options.BackupDbToKubeSecret = true
	}

	env = os.Getenv("HEKETI_PROFILING")
	if "" != env {
		options.Profiling = true
	}

	env = os.Getenv("HEKETI_DEFAULT_STATE")
	if "" != env {
		options.DefaultState = env
	}
}

func setupApp(config *config.Config) (a *glusterfs.App) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Unable to start application: %s\n", err)
			os.Exit(1)
		}
	}()

	// If one really needs to disable the health monitor for
	// the server binary.
	glusterfs.MonitorGlusterNodes = enableBackgroundTask(
		config.GlusterFS.DisableMonitorGlusterNodes,
		"HEKETI_DISABLE_HEALTH_MONITOR")
	// If one really needs to disable the health monitor for
	// the server binary.
	glusterfs.EnableBackgroundCleaner = enableBackgroundTask(
		config.GlusterFS.DisableBackgroundCleaner,
		"HEKETI_DISABLE_BACKGROUND_CLEANER")

	a, e := glusterfs.NewApp(config.GlusterFS)
	if e != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Unable to start application: %s\n", e)
		os.Exit(1)
	}
	if err := a.ServerReset(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: Failed to reset server application")
		os.Exit(1)
	}
	return a
}

// check if the config file or environment has disabled a internal
// background task. Generally, this is only for developers or someone
// debugging server issues, and isn't meant to be easy to flip.
func enableBackgroundTask(cfgDisable bool, envDisable string) bool {
	if cfgDisable {
		return false
	}
	if os.Getenv(envDisable) == "true" {
		return false
	}
	return true
}

func randSeed() {
	// from rand.Seed docs: "Seed values that have the same remainder when
	// divided by 2^31-1 generate the same pseudo-random sequence."
	max := big.NewInt(1<<31 - 1)
	n, err := crand.Int(crand.Reader, max)
	if err != nil {
		rand.Seed(time.Now().UnixNano())
	} else {
		rand.Seed(n.Int64())
	}
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	// Quit here if all we needed to do was show usage/help
	if configfile == "" {
		return
	}

	// Seed PRNG
	randSeed()

	// Read configuration
	options, err := config.ReadConfig(configfile)
	if err != nil {
		os.Exit(1)
	}

	// Substitute values using any set environment variables
	setWithEnvVariables(options)

	// Use negroni to add middleware.  Here we add two
	// middlewares: Recovery and Logger, which come with
	// Negroni
	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger())

	// Setup a new GlusterFS application
	app := setupApp(options)

	// Add /hello router
	router := mux.NewRouter()
	router.Methods("GET").Path("/hello").Name("Hello").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Hello from Heketi")
		})

	router.Methods("GET").Path("/metrics").Name("Metrics").HandlerFunc(metrics.NewMetricsHandler(app))

	// Enable profiling on "/debug/pprof"
	if options.Profiling {
		profiling.EnableProfiling(router)
	}

	// Create a router and do not allow any routes
	// unless defined.
	heketiRouter := mux.NewRouter().StrictSlash(true)
	err = app.SetRoutes(heketiRouter)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: Unable to create http server endpoints")
		os.Exit(1)
	}

	// Load authorization JWT middleware
	if options.AuthEnabled {
		jwtauth := middleware.NewJwtAuth(&options.JwtConfig)
		if jwtauth == nil {
			fmt.Fprintln(os.Stderr, "ERROR: Missing JWT information in config file")
			os.Exit(1)
		}

		// Add Token parser
		n.Use(jwtauth)

		// Add application middleware check
		n.UseFunc(app.Auth)

		fmt.Println("Authorization loaded")
	}

	adminss := admin.New()
	n.Use(adminss)
	adminss.SetRoutes(heketiRouter)
	if err := adminss.SetString(options.DefaultState); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: unable to set admin state:", err)
		os.Exit(1)
	}

	if options.BackupDbToKubeSecret {
		// Check if running in a Kubernetes environment
		_, err = restclient.InClusterConfig()
		if err == nil {
			// Load middleware to backup database
			n.UseFunc(app.BackupToKubernetesSecret)
		}
	}

	// Add all endpoints after the middleware was added
	n.UseHandler(heketiRouter)

	// Setup complete routing
	router.NewRoute().Handler(n)

	// Reset admin mode on SIGUSR2
	admin.ResetStateOnSignal(adminss, syscall.SIGUSR2)

	// Shutdown on CTRL-C signal
	// For a better cleanup, we should shutdown the server and
	signalch := make(chan os.Signal, 1)
	signal.Notify(signalch, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)

	// Create a channel to know if the server was unable to start
	done := make(chan bool)
	go func() {
		// Start the server.
		if options.EnableTls {
			fmt.Printf("Listening on port %v with TLS enabled\n", options.Port)
			err = http.ListenAndServeTLS(":"+options.Port, options.CertFile, options.KeyFile, router)
		} else {
			fmt.Printf("Listening on port %v\n", options.Port)
			err = http.ListenAndServe(":"+options.Port, router)
		}
		if err != nil {
			fmt.Printf("ERROR: HTTP Server error: %v\n", err)
		}
		done <- true
	}()

	// Block here for signals and errors from the HTTP server
	select {
	case <-signalch:
	case <-done:
	}
	fmt.Printf("Shutting down...\n")

	// Shutdown the application
	// :TODO: Need to shutdown the server
	app.Close()

}
