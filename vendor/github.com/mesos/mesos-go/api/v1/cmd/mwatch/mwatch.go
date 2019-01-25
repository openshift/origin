package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"text/template"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli/httpmaster"
	"github.com/mesos/mesos-go/api/v1/lib/master"
	"github.com/mesos/mesos-go/api/v1/lib/master/calls"
)

var (
	masterHost = flag.String("master", "127.0.0.1", "IP address of mesos master")
	masterPort = flag.Int("port", 5050, "Port of mesos master")

	// example of using a template:
	// mwatch -task.template='{{if .TaskAdded}}{{with .TaskAdded.Task}} {{.Resources|formatResources}}{{end}}{{end}}'

	taskEvents        = flag.Bool("task.on", true, "When true, output mesos task events")
	taskTemplate      = flag.String("task.template", "", "When defined this golang text-template is used to format task events")
	frameworkEvents   = flag.Bool("framework.on", true, "When true, output mesos framework events")
	frameworkTemplate = flag.String("framework.template", "", "When defined this golang text-template is used to format framework events")
	agentEvents       = flag.Bool("agent.on", true, "When true, output mesos agent events")
	agentTemplate     = flag.String("agent.template", "", "When defined this golang text-template is used to format agent events")
)

func init() {
	flag.Parse()
}

func main() {
	var (
		uri = fmt.Sprintf("http://%s/api/v1", net.JoinHostPort(*masterHost, strconv.Itoa(*masterPort)))
		cli = httpmaster.NewSender(httpcli.New(httpcli.Endpoint(uri)).Send)
		ctx = context.Background()

		taskTemp, frameworkTemp, agentTemp *template.Template
	)
	if *taskTemplate != "" {
		fm := template.FuncMap(map[string]interface{}{
			"formatResources": func(r []mesos.Resource) string { return mesos.Resources(r).String() },
		})
		taskTemp = template.Must(template.New("task").Funcs(fm).Parse(*taskTemplate))
	}
	if *frameworkTemplate != "" {
		frameworkTemp = template.Must(template.New("framework").Parse(*frameworkTemplate))
	}
	if *agentTemplate != "" {
		agentTemp = template.Must(template.New("agent").Parse(*agentTemplate))
	}
	err := watch(taskTemp, frameworkTemp, agentTemp)(cli.Send(ctx, calls.NonStreaming(calls.Subscribe())))
	if err != nil {
		panic(err)
	}
}

func watch(taskTemp, frameworkTemp, agentTemp *template.Template) func(mesos.Response, error) error {
	return func(resp mesos.Response, err error) error {
		defer func() {
			if resp != nil {
				resp.Close()
			}
		}()
		for err == nil {
			var e master.Event
			if err := resp.Decode(&e); err != nil {
				if err == io.EOF {
					err = nil
					break
				}
				continue
			}
			switch t := e.GetType(); t {
			case master.Event_TASK_ADDED:
				if !*taskEvents {
					continue
				}
				if taskTemp != nil {
					err = taskTemp.Execute(os.Stdout, e)
					continue
				}
				task := e.GetTaskAdded().Task
				fmt.Println(t.String(), task.GetFrameworkID(), task.GetTaskID(), task.GetState(), task.GetLabels().Format(), mesos.Resources(task.GetResources()))
			case master.Event_TASK_UPDATED:
				if !*taskEvents {
					continue
				}
				if taskTemp != nil {
					err = taskTemp.Execute(os.Stdout, e)
					continue
				}
				task := e.GetTaskUpdated().GetStatus()
				fmt.Println(t.String(), task.GetTaskID(), task.GetState(), task.GetLabels().Format())
			case master.Event_AGENT_ADDED:
				if !*agentEvents {
					continue
				}
				if agentTemp != nil {
					err = agentTemp.Execute(os.Stdout, e)
					continue
				}
				fmt.Println(t.String(), e.GetAgentAdded().String())
			case master.Event_AGENT_REMOVED:
				if !*agentEvents {
					continue
				}
				if agentTemp != nil {
					err = agentTemp.Execute(os.Stdout, e)
					continue
				}
				fmt.Println(t.String(), e.GetAgentRemoved().String())
			case master.Event_FRAMEWORK_ADDED:
				if !*frameworkEvents {
					continue
				}
				if frameworkTemp != nil {
					err = frameworkTemp.Execute(os.Stdout, e)
					continue
				}
				fmt.Println(t.String(), e.GetFrameworkAdded().String())
			case master.Event_FRAMEWORK_UPDATED:
				if !*frameworkEvents {
					continue
				}
				if frameworkTemp != nil {
					err = frameworkTemp.Execute(os.Stdout, e)
					continue
				}
				fmt.Println(t.String(), e.GetFrameworkUpdated().String())
			case master.Event_FRAMEWORK_REMOVED:
				if !*frameworkEvents {
					continue
				}
				if frameworkTemp != nil {
					err = frameworkTemp.Execute(os.Stdout, e)
					continue
				}
				fmt.Println(t.String(), e.GetFrameworkRemoved().String())
			default:
				// noop
			}
		}
		return err
	}
}
