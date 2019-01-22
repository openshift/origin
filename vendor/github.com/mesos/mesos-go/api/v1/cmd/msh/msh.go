// msh is a minimal mesos v1 scheduler; it executes a shell command on a mesos agent.
package main

// Usage: msh {...command line args...}
//
// For example:
//    msh -master 10.2.0.5:5050 -- ls -laF /tmp
//
// TODO: -gpu=1 to enable GPU_RESOURCES caps and request 1 gpu
//

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/agent"
	agentcalls "github.com/mesos/mesos-go/api/v1/lib/agent/calls"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/callrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/eventrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/offers"
	"github.com/mesos/mesos-go/api/v1/lib/extras/store"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli/httpagent"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli/httpsched"
	"github.com/mesos/mesos-go/api/v1/lib/resources"
	"github.com/mesos/mesos-go/api/v1/lib/roles"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/events"
)

const (
	RFC3339a = "20060102T150405Z0700"
)

var (
	FrameworkName = "msh"
	TaskName      = "msh"
	MesosMaster   = "127.0.0.1:5050"
	User          = "root"
	Role          = roles.Role("*")
	CPUs          = float64(0.010)
	Memory        = float64(64)

	fidStore               store.Singleton
	declineAndSuppress     bool
	refuseSeconds          = calls.RefuseSeconds(5 * time.Second)
	wantsResources         mesos.Resources
	taskPrototype          mesos.TaskInfo
	interactive            bool
	tty                    bool
	pod                    bool
	executorPrototype      mesos.ExecutorInfo
	wantsExecutorResources = mesos.Resources{
		resources.NewCPUs(0.01).Resource,
		resources.NewMemory(32).Resource,
		resources.NewDisk(5).Resource,
	}
	agentDirectory = make(map[mesos.AgentID]string)
	uponExit       = new(cleanups)
)

func init() {
	flag.StringVar(&FrameworkName, "framework_name", FrameworkName, "Name of the framework")
	flag.StringVar(&TaskName, "task_name", TaskName, "Name of the msh task")
	flag.StringVar(&MesosMaster, "master", MesosMaster, "IP:port of the mesos master")
	flag.StringVar(&User, "user", User, "OS user that owns the launched task")
	flag.Float64Var(&CPUs, "cpus", CPUs, "CPU resources to allocate for the remote command")
	flag.Float64Var(&Memory, "memory", Memory, "Memory resources to allocate for the remote command")
	flag.BoolVar(&tty, "tty", tty, "Route all container stdio, stdout, stderr communication through a TTY device")
	flag.BoolVar(&pod, "pod", pod, "Launch the remote command in a mesos task-group")
	flag.BoolVar(&interactive, "interactive", interactive, "Attach to the task's stdin, stdout, and stderr")

	fidStore = store.DecorateSingleton(
		store.NewInMemorySingleton(),
		store.DoSet().AndThen(func(_ store.Setter, v string, _ error) error {
			log.Println("FrameworkID", v)
			return nil
		}))
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 { // msh by itself prints usage
		flag.Usage()
		os.Exit(1)
	}

	wantsResources = mesos.Resources{
		resources.NewCPUs(CPUs).Resource,
		resources.NewMemory(Memory).Resource,
	}

	taskPrototype = mesos.TaskInfo{
		Name: TaskName,
		Command: &mesos.CommandInfo{
			Value: proto.String(args[0]),
			Shell: proto.Bool(false),
		},
	}
	taskPrototype.Command.Arguments = args
	if interactive {
		taskPrototype.Container = &mesos.ContainerInfo{
			Type:    mesos.ContainerInfo_MESOS.Enum(),
			TTYInfo: &mesos.TTYInfo{},
		}
	}
	if term := os.Getenv("TERM"); term != "" && tty {
		taskPrototype.Command.Environment = &mesos.Environment{
			Variables: []mesos.Environment_Variable{
				mesos.Environment_Variable{Name: "TERM", Value: &term},
			},
		}
	}
	if err := run(); err != nil {
		if exitErr, ok := err.(ExitError); ok {
			if code := int(exitErr); code != 0 {
				log.Println(exitErr)
				uponExit.unwind()
				os.Exit(code)
			}
			// else, code=0 indicates success, exit normally
		} else {
			uponExit.unwind()
			log.Fatalf("%#v", err)
		}
	}
	uponExit.unwind()
}

func run() error {
	var (
		ctx, cancel = context.WithCancel(context.Background())
		caller      = callrules.WithFrameworkID(store.GetIgnoreErrors(fidStore)).Caller(buildClient())
	)

	return controller.Run(
		ctx,
		&mesos.FrameworkInfo{User: User, Name: FrameworkName, Role: (*string)(&Role)},
		caller,
		controller.WithEventHandler(buildEventHandler(caller)),
		controller.WithFrameworkID(store.GetIgnoreErrors(fidStore)),
		controller.WithSubscriptionTerminated(func(err error) {
			defer cancel()
			if err == io.EOF {
				log.Println("disconnected")
			}
		}),
	)
}

func buildClient() calls.Caller {
	return httpsched.NewCaller(httpcli.New(
		httpcli.Endpoint(fmt.Sprintf("http://%s/api/v1/scheduler", MesosMaster)),
	))
}

func buildEventHandler(caller calls.Caller) events.Handler {
	logger := controller.LogEvents(nil)
	return controller.LiftErrors().Handle(events.Handlers{
		scheduler.Event_SUBSCRIBED: eventrules.Rules{
			logger,
			controller.TrackSubscription(fidStore, 0),
			updateExecutor,
		},

		scheduler.Event_OFFERS: eventrules.Rules{
			trackAgents,
			maybeDeclineOffers(caller),
			eventrules.DropOnError(),
			eventrules.Handle(resourceOffers(caller)),
		},

		scheduler.Event_UPDATE: controller.AckStatusUpdates(caller).AndThen().HandleF(statusUpdate),
	}.Otherwise(logger.HandleEvent))
}

func updateExecutor(ctx context.Context, e *scheduler.Event, err error, chain eventrules.Chain) (context.Context, *scheduler.Event, error) {
	if err != nil {
		return chain(ctx, e, err)
	}
	if e.GetType() != scheduler.Event_SUBSCRIBED {
		return chain(ctx, e, err)
	}
	if pod {
		executorPrototype = mesos.ExecutorInfo{
			Type:        mesos.ExecutorInfo_DEFAULT,
			FrameworkID: e.GetSubscribed().FrameworkID,
		}
	}
	return chain(ctx, e, err)
}

func trackAgents(ctx context.Context, e *scheduler.Event, err error, chain eventrules.Chain) (context.Context, *scheduler.Event, error) {
	if err != nil {
		return chain(ctx, e, err)
	}
	if e.GetType() != scheduler.Event_OFFERS {
		return chain(ctx, e, err)
	}
	off := e.GetOffers().GetOffers()
	for i := range off {
		// TODO(jdef) eventually implement an algorithm to purge agents that are gone
		agentDirectory[off[i].GetAgentID()] = off[i].GetHostname()
	}
	return chain(ctx, e, err)
}

func maybeDeclineOffers(caller calls.Caller) eventrules.Rule {
	return func(ctx context.Context, e *scheduler.Event, err error, chain eventrules.Chain) (context.Context, *scheduler.Event, error) {
		if err != nil {
			return chain(ctx, e, err)
		}
		if e.GetType() != scheduler.Event_OFFERS || !declineAndSuppress {
			return chain(ctx, e, err)
		}
		off := offers.Slice(e.GetOffers().GetOffers())
		err = calls.CallNoData(ctx, caller, calls.Decline(off.IDs()...).With(refuseSeconds))
		if err == nil {
			// we shouldn't have received offers, maybe the prior suppress call failed?
			err = calls.CallNoData(ctx, caller, calls.Suppress())
		}
		return ctx, e, err // drop
	}
}

func resourceOffers(caller calls.Caller) events.HandlerFunc {
	return func(ctx context.Context, e *scheduler.Event) (err error) {
		var (
			off            = e.GetOffers().GetOffers()
			index          = offers.NewIndex(off, nil)
			matchResources = func() mesos.Resources {
				if pod {
					return wantsResources.Plus(wantsExecutorResources...)
				} else {
					return wantsResources
				}
			}()
			match = index.Find(offers.ContainsResources(matchResources))
		)
		if match != nil {
			ts := time.Now().Format(RFC3339a)
			task := taskPrototype
			task.TaskID = mesos.TaskID{Value: ts}
			task.AgentID = match.AgentID
			task.Resources = resources.Find(
				resources.Flatten(wantsResources, Role.Assign()),
				match.Resources...,
			)

			if pod {
				executor := executorPrototype
				executor.ExecutorID = mesos.ExecutorID{Value: "msh_" + ts}
				executor.Resources = resources.Find(
					resources.Flatten(wantsExecutorResources, Role.Assign()),
					match.Resources...,
				)
				err = calls.CallNoData(ctx, caller, calls.Accept(
					calls.OfferOperations{calls.OpLaunchGroup(executor, task)}.WithOffers(match.ID),
				))
			} else {
				err = calls.CallNoData(ctx, caller, calls.Accept(
					calls.OfferOperations{calls.OpLaunch(task)}.WithOffers(match.ID),
				))
			}
			if err != nil {
				return
			}

			declineAndSuppress = true
		} else {
			log.Println("rejected insufficient offers")
		}
		// decline all but the possible match
		delete(index, match.GetID())
		err = calls.CallNoData(ctx, caller, calls.Decline(index.IDs()...).With(refuseSeconds))
		if err != nil {
			return
		}
		if declineAndSuppress {
			err = calls.CallNoData(ctx, caller, calls.Suppress())
		}
		return
	}
}

func statusUpdate(_ context.Context, e *scheduler.Event) error {
	s := e.GetUpdate().GetStatus()
	switch st := s.GetState(); st {
	case mesos.TASK_FINISHED, mesos.TASK_RUNNING, mesos.TASK_STAGING, mesos.TASK_STARTING:
		log.Printf("status update from agent %q: %v", s.GetAgentID().GetValue(), st)
		if st == mesos.TASK_RUNNING && interactive && s.AgentID != nil {
			cid := s.GetContainerStatus().GetContainerID()
			if cid != nil {
				log.Printf("attaching for interactive session to agent %q container %q", s.AgentID.Value, cid.Value)
				return tryInteractive(agentDirectory[*s.AgentID], *cid)
			}
		}
		if st != mesos.TASK_FINISHED {
			return nil
		}
	case mesos.TASK_LOST, mesos.TASK_KILLED, mesos.TASK_FAILED, mesos.TASK_ERROR:
		log.Println("Exiting because task " + s.GetTaskID().Value +
			" is in an unexpected state " + st.String() +
			" with reason " + s.GetReason().String() +
			" from source " + s.GetSource().String() +
			" with message '" + s.GetMessage() + "'")
		return ExitError(3)
	default:
		log.Println("unexpected task state, aborting", st)
		return ExitError(4)
	}
	return ExitError(0) // kind of ugly, but better than os.Exit(0)
}

type ExitError int

func (e ExitError) Error() string { return fmt.Sprintf("exit code %d", int(e)) }

func tryInteractive(agentHost string, cid mesos.ContainerID) (err error) {
	// TODO(jdef) only re-attach if we're disconnected (guard against redundant TASK_RUNNING)
	var (
		ctx, cancel = context.WithCancel(context.TODO())
		winCh       <-chan mesos.TTYInfo_WindowSize
	)
	if tty {
		ttyd, err := initTTY()
		if err != nil {
			cancel() // stop go-vet from complaining
			return err
		}

		uponExit.push(ttyd.Close) // fail-safe

		go func() {
			<-ctx.Done()
			//println("closing ttyd via ctx.Done")
			ttyd.Close()
		}()

		winCh = ttyd.winch
	}

	var (
		cli = httpagent.NewSender(
			httpcli.New(
				httpcli.Endpoint(fmt.Sprintf("http://%s/api/v1", net.JoinHostPort(agentHost, "5051"))),
			).Send,
		)
		aciCh = make(chan *agent.Call, 1) // must be buffered to avoid blocking below
	)
	aciCh <- agentcalls.AttachContainerInput(cid) // very first input message MUST be this
	go func() {
		defer cancel()
		acif := agentcalls.FromChan(aciCh)

		// blocking call, hence the goroutine; Send only returns when the input stream is severed
		err2 := agentcalls.SendNoData(ctx, cli, acif)
		if err2 != nil && err2 != io.EOF {
			log.Printf("attached input stream error %v", err2)
		}
	}()

	// attach to container stdout, stderr; Send returns immediately with a Response from which output
	// may be decoded.
	output, err := cli.Send(ctx, agentcalls.NonStreaming(agentcalls.AttachContainerOutput(cid)))
	if err != nil {
		log.Printf("attach output stream error: %v", err)
		if output != nil {
			output.Close()
		}
		cancel()
		return
	}

	go func() {
		defer cancel()
		attachContainerOutput(output, os.Stdout, os.Stderr)
	}()

	go attachContainerInput(ctx, os.Stdin, winCh, aciCh)

	return nil
}

func attachContainerInput(ctx context.Context, stdin io.Reader, winCh <-chan mesos.TTYInfo_WindowSize, aciCh chan<- *agent.Call) {
	defer close(aciCh)

	input := make(chan []byte)
	go func() {
		defer close(input)
		escape := []byte{0x10, 0x11} // CTRL-P, CTRL-Q
		var last byte
		for {
			buf := make([]byte, 512) // not efficient to always do this
			n, err := stdin.Read(buf)
			if n > 0 {
				if (last == escape[0] && buf[0] == escape[1]) || bytes.Index(buf, escape) > -1 {
					//println("escape sequence detected")
					return
				}
				buf = buf[:n]
				last = buf[n-1]
				select {
				case input <- buf:
				case <-ctx.Done():
					return
				}
			}
			// TODO(jdef) check for temporary error?
			if err != nil {
				return
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		// TODO(jdef) send a heartbeat message every so often
		// attach_container_input process_io heartbeats may act as keepalive's, `interval` field is ignored:
		// https://github.com/apache/mesos/blob/4e200e55d8ed282b892f650983ebdf516680d90d/src/slave/containerizer/mesos/io/switchboard.cpp#L1608
		case data, ok := <-input:
			if !ok {
				return
			}
			c := agentcalls.AttachContainerInputData(data)
			select {
			case aciCh <- c:
			case <-ctx.Done():
				return
			}
		case ws := <-winCh:
			c := agentcalls.AttachContainerInputTTY(&mesos.TTYInfo{WindowSize: &ws})
			select {
			case aciCh <- c:
			case <-ctx.Done():
				return
			}
		}
	}
}

func attachContainerOutput(resp mesos.Response, stdout, stderr io.Writer) error {
	defer resp.Close()
	forward := func(b []byte, out io.Writer) error {
		n, err := out.Write(b)
		if err == nil && len(b) != n {
			err = io.ErrShortWrite
		}
		return err
	}
	for {
		var pio agent.ProcessIO
		err := resp.Decode(&pio)
		if err != nil {
			return err
		}
		switch pio.GetType() {
		case agent.ProcessIO_DATA:
			data := pio.GetData()
			switch data.GetType() {
			case agent.ProcessIO_Data_STDOUT:
				if err := forward(data.GetData(), stdout); err != nil {
					return err
				}
			case agent.ProcessIO_Data_STDERR:
				if err := forward(data.GetData(), stderr); err != nil {
					return err
				}
			default:
				// ignore
			}
		default:
			// ignore
		}
	}
}
