package app

import (
	"context"
	"errors"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/backoff"
	xmetrics "github.com/mesos/mesos-go/api/v1/lib/extras/metrics"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/callrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/eventrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/store"
	"github.com/mesos/mesos-go/api/v1/lib/resources"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/events"
)

var (
	RegistrationMinBackoff = 1 * time.Second
	RegistrationMaxBackoff = 15 * time.Second
)

// StateError is returned when the system encounters an unresolvable state transition error and
// should likely exit.
type StateError string

func (err StateError) Error() string { return string(err) }

func Run(cfg Config) error {
	log.Printf("scheduler running with configuration: %+v", cfg)
	ctx, cancel := context.WithCancel(context.Background())

	state, err := newInternalState(cfg, cancel)
	if err != nil {
		return err
	}

	// TODO(jdef) how to track/handle timeout errors that occur for SUBSCRIBE calls? we should
	// probably tolerate X number of subsequent subscribe failures before bailing. we'll need
	// to track the lastCallAttempted along with subsequentSubscribeTimeouts.

	fidStore := store.DecorateSingleton(
		store.NewInMemorySingleton(),
		store.DoSet().AndThen(func(_ store.Setter, v string, _ error) error {
			log.Println("FrameworkID", v)
			return nil
		}))

	state.cli = callrules.New(
		callrules.WithFrameworkID(store.GetIgnoreErrors(fidStore)),
		logCalls(map[scheduler.Call_Type]string{scheduler.Call_SUBSCRIBE: "connecting..."}),
		callMetrics(state.metricsAPI, time.Now, state.config.summaryMetrics),
	).Caller(state.cli)

	err = controller.Run(
		ctx,
		buildFrameworkInfo(state.config),
		state.cli,
		controller.WithEventHandler(buildEventHandler(state, fidStore)),
		controller.WithFrameworkID(store.GetIgnoreErrors(fidStore)),
		controller.WithRegistrationTokens(
			backoff.Notifier(RegistrationMinBackoff, RegistrationMaxBackoff, ctx.Done()),
		),
		controller.WithSubscriptionTerminated(func(err error) {
			if err != nil {
				if err != io.EOF {
					log.Println(err)
				}
				if _, ok := err.(StateError); ok {
					state.shutdown()
				}
				return
			}
			log.Println("disconnected")
		}),
	)
	if state.err != nil {
		err = state.err
	}
	return err
}

// buildEventHandler generates and returns a handler to process events received from the subscription.
func buildEventHandler(state *internalState, fidStore store.Singleton) events.Handler {
	// disable brief logs when verbose logs are enabled (there's no sense logging twice!)
	logger := controller.LogEvents(nil).Unless(state.config.verbose)
	return eventrules.New(
		logAllEvents().If(state.config.verbose),
		eventMetrics(state.metricsAPI, time.Now, state.config.summaryMetrics),
		controller.LiftErrors().DropOnError(),
	).Handle(events.Handlers{
		scheduler.Event_FAILURE: logger.HandleF(failure),
		scheduler.Event_OFFERS:  trackOffersReceived(state).HandleF(resourceOffers(state)),
		scheduler.Event_UPDATE:  controller.AckStatusUpdates(state.cli).AndThen().HandleF(statusUpdate(state)),
		scheduler.Event_SUBSCRIBED: eventrules.New(
			logger,
			controller.TrackSubscription(fidStore, state.config.failoverTimeout),
		),
	}.Otherwise(logger.HandleEvent))
}

func trackOffersReceived(state *internalState) eventrules.Rule {
	return func(ctx context.Context, e *scheduler.Event, err error, chain eventrules.Chain) (context.Context, *scheduler.Event, error) {
		if err == nil {
			state.metricsAPI.offersReceived.Int(len(e.GetOffers().GetOffers()))
		}
		return chain(ctx, e, err)
	}
}

func failure(_ context.Context, e *scheduler.Event) error {
	var (
		f              = e.GetFailure()
		eid, aid, stat = f.ExecutorID, f.AgentID, f.Status
	)
	if eid != nil {
		// executor failed..
		msg := "executor '" + eid.Value + "' terminated"
		if aid != nil {
			msg += " on agent '" + aid.Value + "'"
		}
		if stat != nil {
			msg += " with status=" + strconv.Itoa(int(*stat))
		}
		log.Println(msg)
	} else if aid != nil {
		// agent failed..
		log.Println("agent '" + aid.Value + "' terminated")
	}
	return nil
}

func resourceOffers(state *internalState) events.HandlerFunc {
	return func(ctx context.Context, e *scheduler.Event) error {
		var (
			offers                 = e.GetOffers().GetOffers()
			callOption             = calls.RefuseSecondsWithJitter(state.random, state.config.maxRefuseSeconds)
			tasksLaunchedThisCycle = 0
			offersDeclined         = 0
		)
		for i := range offers {
			var (
				remaining = mesos.Resources(offers[i].Resources)
				tasks     = []mesos.TaskInfo{}
			)

			if state.config.verbose {
				log.Println("received offer id '" + offers[i].ID.Value +
					"' with resources " + remaining.String())
			}

			var wantsExecutorResources mesos.Resources
			if len(offers[i].ExecutorIDs) == 0 {
				wantsExecutorResources = mesos.Resources(state.executor.Resources)
			}

			flattened := remaining.ToUnreserved()

			// avoid the expense of computing these if we can...
			if state.config.summaryMetrics && state.config.resourceTypeMetrics {
				for name, restype := range resources.TypesOf(flattened...) {
					if restype == mesos.SCALAR {
						sum, _ := name.Sum(flattened...)
						state.metricsAPI.offeredResources(sum.GetScalar().GetValue(), name.String())
					}
				}
			}

			taskWantsResources := state.wantsTaskResources.Plus(wantsExecutorResources...)
			for state.tasksLaunched < state.totalTasks && resources.ContainsAll(flattened, taskWantsResources) {
				found := func() mesos.Resources {
					if state.config.role == "*" {
						return resources.Find(state.wantsTaskResources, remaining...)
					}
					reservation := mesos.Resource_ReservationInfo{
						Type: mesos.Resource_ReservationInfo_STATIC.Enum(),
						Role: &state.config.role,
					}
					return resources.Find(state.wantsTaskResources.PushReservation(reservation))
				}()

				if len(found) == 0 {
					panic("illegal state: failed to find the resources that were supposedly contained")
				}

				state.tasksLaunched++
				taskID := state.tasksLaunched

				if state.config.verbose {
					log.Println("launching task " + strconv.Itoa(taskID) + " using offer " + offers[i].ID.Value)
				}

				task := mesos.TaskInfo{
					TaskID:    mesos.TaskID{Value: strconv.Itoa(taskID)},
					AgentID:   offers[i].AgentID,
					Executor:  state.executor,
					Resources: found,
				}
				task.Name = "Task " + task.TaskID.Value
				tasks = append(tasks, task)

				remaining.Subtract(task.Resources...)
				flattened = remaining.ToUnreserved()
			}

			// build Accept call to launch all of the tasks we've assembled
			accept := calls.Accept(
				calls.OfferOperations{calls.OpLaunch(tasks...)}.WithOffers(offers[i].ID),
			).With(callOption)

			// send Accept call to mesos
			err := calls.CallNoData(ctx, state.cli, accept)
			if err != nil {
				log.Printf("failed to launch tasks: %+v", err)
			} else {
				if n := len(tasks); n > 0 {
					tasksLaunchedThisCycle += n
				} else {
					offersDeclined++
				}
			}
		}
		state.metricsAPI.offersDeclined.Int(offersDeclined)
		state.metricsAPI.tasksLaunched.Int(tasksLaunchedThisCycle)
		if state.config.summaryMetrics {
			state.metricsAPI.launchesPerOfferCycle(float64(tasksLaunchedThisCycle))
		}
		if tasksLaunchedThisCycle == 0 && state.config.verbose {
			log.Println("zero tasks launched this cycle")
		}
		return nil
	}
}

func statusUpdate(state *internalState) events.HandlerFunc {
	return func(ctx context.Context, e *scheduler.Event) error {
		s := e.GetUpdate().GetStatus()
		if state.config.verbose {
			msg := "Task " + s.TaskID.Value + " is in state " + s.GetState().String()
			if m := s.GetMessage(); m != "" {
				msg += " with message '" + m + "'"
			}
			log.Println(msg)
		}

		switch st := s.GetState(); st {
		case mesos.TASK_FINISHED:
			state.tasksFinished++
			state.metricsAPI.tasksFinished()

			if state.tasksFinished == state.totalTasks {
				log.Println("mission accomplished, terminating")
				state.shutdown()
			} else {
				tryReviveOffers(ctx, state)
			}

		case mesos.TASK_LOST, mesos.TASK_KILLED, mesos.TASK_FAILED, mesos.TASK_ERROR:
			state.err = errors.New("Exiting because task " + s.GetTaskID().Value +
				" is in an unexpected state " + st.String() +
				" with reason " + s.GetReason().String() +
				" from source " + s.GetSource().String() +
				" with message '" + s.GetMessage() + "'")
			state.shutdown()
		}
		return nil
	}
}

func tryReviveOffers(ctx context.Context, state *internalState) {
	// limit the rate at which we request offer revival
	select {
	case <-state.reviveTokens:
		// not done yet, revive offers!
		err := calls.CallNoData(ctx, state.cli, calls.Revive())
		if err != nil {
			log.Printf("failed to revive offers: %+v", err)
			return
		}
	default:
		// noop
	}
}

// logAllEvents logs every observed event; this is somewhat expensive to do
func logAllEvents() eventrules.Rule {
	return func(ctx context.Context, e *scheduler.Event, err error, ch eventrules.Chain) (context.Context, *scheduler.Event, error) {
		log.Printf("%+v\n", *e)
		return ch(ctx, e, err)
	}
}

// eventMetrics logs metrics for every processed API event
func eventMetrics(metricsAPI *metricsAPI, clock func() time.Time, timingMetrics bool) eventrules.Rule {
	timed := metricsAPI.eventReceivedLatency
	if !timingMetrics {
		timed = nil
	}
	harness := xmetrics.NewHarness(metricsAPI.eventReceivedCount, metricsAPI.eventErrorCount, timed, clock)
	return eventrules.Metrics(harness, nil)
}

// callMetrics logs metrics for every outgoing Mesos call
func callMetrics(metricsAPI *metricsAPI, clock func() time.Time, timingMetrics bool) callrules.Rule {
	timed := metricsAPI.callLatency
	if !timingMetrics {
		timed = nil
	}
	harness := xmetrics.NewHarness(metricsAPI.callCount, metricsAPI.callErrorCount, timed, clock)
	return callrules.Metrics(harness, nil)
}

// logCalls logs a specific message string when a particular call-type is observed
func logCalls(messages map[scheduler.Call_Type]string) callrules.Rule {
	return func(ctx context.Context, c *scheduler.Call, r mesos.Response, err error, ch callrules.Chain) (context.Context, *scheduler.Call, mesos.Response, error) {
		if message, ok := messages[c.GetType()]; ok {
			log.Println(message)
		}
		return ch(ctx, c, r, err)
	}
}
