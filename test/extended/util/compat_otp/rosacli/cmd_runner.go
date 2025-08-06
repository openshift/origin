package rosacli

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
)

const (
	defaultRunnerFormat = "text"
	jsonRunnerFormat    = "json"
	yamlRunnerFormat    = "yaml"
)

type runner struct {
	cmds      []string
	cmdArgs   []string
	runnerCfg *runnerConfig
	sensitive bool
}

type runnerConfig struct {
	format string
	color  string
	debug  bool
}

func NewRunner() *runner {
	runner := &runner{
		runnerCfg: &runnerConfig{
			format: "text",
			debug:  false,
			color:  "auto",
		},
	}
	return runner
}

func (r *runner) Copy() *runner {
	return &runner{
		runnerCfg: r.runnerCfg.Copy(),
		sensitive: r.sensitive,
	}
}

func (rc *runnerConfig) Copy() *runnerConfig {
	return &runnerConfig{
		format: rc.format,
		color:  rc.color,
		debug:  rc.debug,
	}
}

func (r *runner) Sensitive(sensitive bool) *runner {
	r.sensitive = sensitive
	return r
}

func (r *runner) format(format string) *runner {
	r.runnerCfg.format = format
	return r
}

func (r *runner) Debug(debug bool) *runner {
	r.runnerCfg.debug = debug
	return r
}

func (r *runner) Color(color string) *runner {
	r.runnerCfg.color = color
	return r
}

func (r *runner) JsonFormat() *runner {
	return r.format(jsonRunnerFormat)
}

func (r *runner) YamlFormat() *runner {
	return r.format(yamlRunnerFormat)
}

func (r *runner) UnsetFormat() *runner {
	return r.format(defaultRunnerFormat)
}

func (r *runner) Cmd(commands ...string) *runner {
	r.cmds = commands
	return r
}

func (r *runner) CmdFlags(cmdFlags ...string) *runner {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, cmdFlags...)
	r.cmdArgs = cmdArgs
	return r
}

func (r *runner) AddCmdFlags(cmdFlags ...string) *runner {
	cmdArgs := append(r.cmdArgs, cmdFlags...)
	r.cmdArgs = cmdArgs
	return r
}

func (r *runner) UnsetBoolFlag(flag string) *runner {
	var newCmdArgs []string
	cmdArgs := r.cmdArgs
	for _, vv := range cmdArgs {
		if vv == flag {
			continue
		}
		newCmdArgs = append(newCmdArgs, vv)
	}

	r.cmdArgs = newCmdArgs
	return r
}

func (r *runner) UnsetFlag(flag string) *runner {
	cmdArgs := r.cmdArgs
	flagIndex := 0
	for n, key := range cmdArgs {
		if key == flag {
			flagIndex = n
			break
		}
	}

	cmdArgs = append(cmdArgs[:flagIndex], cmdArgs[flagIndex+2:]...)
	r.cmdArgs = cmdArgs
	return r
}

func (r *runner) ReplaceFlag(flag string, value string) *runner {
	cmdArgs := r.cmdArgs
	for n, key := range cmdArgs {
		if key == flag {
			cmdArgs[n+1] = value
			break
		}
	}

	r.cmdArgs = cmdArgs
	return r
}

func (rc *runnerConfig) GenerateCmdFlags() (flags []string) {
	if rc.format == jsonRunnerFormat || rc.format == yamlRunnerFormat {
		flags = append(flags, "--output", rc.format)
	}
	if rc.debug {
		flags = append(flags, "--debug")
	}
	if rc.color != "auto" {
		flags = append(flags, "--color", rc.color)
	}
	return
}

func (r *runner) Run() (bytes.Buffer, error) {
	rosacmd := "rosa"
	cmdElements := r.cmds
	if len(r.cmdArgs) > 0 {
		cmdElements = append(cmdElements, r.cmdArgs...)
	}
	cmdElements = append(cmdElements, r.runnerCfg.GenerateCmdFlags()...)

	var output bytes.Buffer
	var err error
	retry := 0
	for {
		if retry > 4 {
			err = fmt.Errorf("executing failed: %s", output.String())
			return output, err
		}
		if r.sensitive {
			logger.Infof("Running command: rosa %s", strings.Join(cmdElements[:2], " "))
		} else {
			logger.Infof("Running command: rosa %s", strings.Join(cmdElements, " "))
		}

		output.Reset()
		cmd := exec.Command(rosacmd, cmdElements...)
		cmd.Stdout = &output
		cmd.Stderr = cmd.Stdout

		err = cmd.Run()
		if !r.sensitive {
			logger.Infof("Get Combining Stdout and Stder is :\n%s", output.String())
		}

		if strings.Contains(output.String(), "Not able to get authentication token") {
			retry = retry + 1
			logger.Warnf("[Retry] Not able to get authentication token!! Wait and sleep 5s to do the %d retry", retry)
			time.Sleep(5 * time.Second)
			continue
		}
		return output, err
	}
}

func (r *runner) RunCMD(command []string) (bytes.Buffer, error) {
	var output bytes.Buffer
	var err error

	if !r.sensitive {
		logger.Infof("Running command: %s", strings.Join(command, " "))
	} else {
		logger.Infof("%s command is running", command[0])
	}
	output.Reset()
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = &output
	cmd.Stderr = cmd.Stdout

	err = cmd.Run()
	logger.Infof("Get Combining Stdout and Stder is :\n%s", output.String())

	return output, err

}
