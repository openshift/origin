// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfexec

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

type initConfig struct {
	backend       bool
	backendConfig []string
	dir           string
	forceCopy     bool
	fromModule    string
	get           bool
	getPlugins    bool
	lock          bool
	lockTimeout   string
	pluginDir     []string
	reattachInfo  ReattachInfo
	reconfigure   bool
	upgrade       bool
	verifyPlugins bool
}

var defaultInitOptions = initConfig{
	backend:       true,
	forceCopy:     false,
	get:           true,
	getPlugins:    true,
	lock:          true,
	lockTimeout:   "0s",
	reconfigure:   false,
	upgrade:       false,
	verifyPlugins: true,
}

// InitOption represents options used in the Init method.
type InitOption interface {
	configureInit(*initConfig)
}

func (opt *BackendOption) configureInit(conf *initConfig) {
	conf.backend = opt.backend
}

func (opt *BackendConfigOption) configureInit(conf *initConfig) {
	conf.backendConfig = append(conf.backendConfig, opt.path)
}

func (opt *DirOption) configureInit(conf *initConfig) {
	conf.dir = opt.path
}

func (opt *ForceCopyOption) configureInit(conf *initConfig) {
	conf.forceCopy = opt.forceCopy
}

func (opt *FromModuleOption) configureInit(conf *initConfig) {
	conf.fromModule = opt.source
}

func (opt *GetOption) configureInit(conf *initConfig) {
	conf.get = opt.get
}

func (opt *GetPluginsOption) configureInit(conf *initConfig) {
	conf.getPlugins = opt.getPlugins
}

func (opt *LockOption) configureInit(conf *initConfig) {
	conf.lock = opt.lock
}

func (opt *LockTimeoutOption) configureInit(conf *initConfig) {
	conf.lockTimeout = opt.timeout
}

func (opt *PluginDirOption) configureInit(conf *initConfig) {
	conf.pluginDir = append(conf.pluginDir, opt.pluginDir)
}

func (opt *ReattachOption) configureInit(conf *initConfig) {
	conf.reattachInfo = opt.info
}

func (opt *ReconfigureOption) configureInit(conf *initConfig) {
	conf.reconfigure = opt.reconfigure
}

func (opt *UpgradeOption) configureInit(conf *initConfig) {
	conf.upgrade = opt.upgrade
}

func (opt *VerifyPluginsOption) configureInit(conf *initConfig) {
	conf.verifyPlugins = opt.verifyPlugins
}

func (tf *Terraform) configureInitOptions(ctx context.Context, c *initConfig, opts ...InitOption) error {
	for _, o := range opts {
		switch o.(type) {
		case *LockOption, *LockTimeoutOption, *VerifyPluginsOption, *GetPluginsOption:
			err := tf.compatible(ctx, nil, tf0_15_0)
			if err != nil {
				return fmt.Errorf("-lock, -lock-timeout, -verify-plugins, and -get-plugins options are no longer available as of Terraform 0.15: %w", err)
			}
		}

		o.configureInit(c)
	}
	return nil
}

// Init represents the terraform init subcommand.
func (tf *Terraform) Init(ctx context.Context, opts ...InitOption) error {
	cmd, err := tf.initCmd(ctx, opts...)
	if err != nil {
		return err
	}
	return tf.runTerraformCmd(ctx, cmd)
}

// InitJSON represents the terraform init subcommand with the `-json` flag.
// Using the `-json` flag will result in
// [machine-readable](https://developer.hashicorp.com/terraform/internals/machine-readable-ui)
// JSON being written to the supplied `io.Writer`.
func (tf *Terraform) InitJSON(ctx context.Context, w io.Writer, opts ...InitOption) error {
	err := tf.compatible(ctx, tf1_9_0, nil)
	if err != nil {
		return fmt.Errorf("terraform init -json was added in 1.9.0: %w", err)
	}

	tf.SetStdout(w)

	cmd, err := tf.initJSONCmd(ctx, opts...)
	if err != nil {
		return err
	}

	return tf.runTerraformCmd(ctx, cmd)
}

func (tf *Terraform) initCmd(ctx context.Context, opts ...InitOption) (*exec.Cmd, error) {
	c := defaultInitOptions

	err := tf.configureInitOptions(ctx, &c, opts...)
	if err != nil {
		return nil, err
	}

	args, err := tf.buildInitArgs(ctx, c)
	if err != nil {
		return nil, err
	}

	// Optional positional argument; must be last as flags precede positional arguments.
	if c.dir != "" {
		args = append(args, c.dir)
	}

	return tf.buildInitCmd(ctx, c, args)
}

func (tf *Terraform) initJSONCmd(ctx context.Context, opts ...InitOption) (*exec.Cmd, error) {
	c := defaultInitOptions

	err := tf.configureInitOptions(ctx, &c, opts...)
	if err != nil {
		return nil, err
	}

	args, err := tf.buildInitArgs(ctx, c)
	if err != nil {
		return nil, err
	}

	args = append(args, "-json")

	// Optional positional argument; must be last as flags precede positional arguments.
	if c.dir != "" {
		args = append(args, c.dir)
	}

	return tf.buildInitCmd(ctx, c, args)
}

func (tf *Terraform) buildInitArgs(ctx context.Context, c initConfig) ([]string, error) {
	args := []string{"init", "-no-color", "-input=false"}

	// string opts: only pass if set
	if c.fromModule != "" {
		args = append(args, "-from-module="+c.fromModule)
	}

	// string opts removed in 0.15: pass if set and <0.15
	err := tf.compatible(ctx, nil, tf0_15_0)
	if err == nil {
		if c.lockTimeout != "" {
			args = append(args, "-lock-timeout="+c.lockTimeout)
		}
	}

	// boolean opts: always pass
	args = append(args, "-backend="+fmt.Sprint(c.backend))
	args = append(args, "-get="+fmt.Sprint(c.get))
	args = append(args, "-upgrade="+fmt.Sprint(c.upgrade))

	// boolean opts removed in 0.15: pass if <0.15
	err = tf.compatible(ctx, nil, tf0_15_0)
	if err == nil {
		args = append(args, "-lock="+fmt.Sprint(c.lock))
		args = append(args, "-get-plugins="+fmt.Sprint(c.getPlugins))
		args = append(args, "-verify-plugins="+fmt.Sprint(c.verifyPlugins))
	}

	if c.forceCopy {
		args = append(args, "-force-copy")
	}

	// unary flags: pass if true
	if c.reconfigure {
		args = append(args, "-reconfigure")
	}

	// string slice opts: split into separate args
	if c.backendConfig != nil {
		for _, bc := range c.backendConfig {
			args = append(args, "-backend-config="+bc)
		}
	}
	if c.pluginDir != nil {
		for _, pd := range c.pluginDir {
			args = append(args, "-plugin-dir="+pd)
		}
	}

	return args, nil
}

func (tf *Terraform) buildInitCmd(ctx context.Context, c initConfig, args []string) (*exec.Cmd, error) {
	mergeEnv := map[string]string{}
	if c.reattachInfo != nil {
		reattachStr, err := c.reattachInfo.marshalString()
		if err != nil {
			return nil, err
		}
		mergeEnv[reattachEnvVar] = reattachStr
	}

	return tf.buildTerraformCmd(ctx, mergeEnv, args...), nil
}
