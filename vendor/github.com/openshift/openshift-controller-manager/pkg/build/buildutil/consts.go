package buildutil

const (
	// BuildStartedEventReason is the reason associated with the event registered when a build is started (pod is created).
	BuildStartedEventReason = "BuildStarted"
	// BuildStartedEventMessage is the message associated with the event registered when a build is started (pod is created).
	BuildStartedEventMessage = "Build %s/%s is now running"
	// BuildCompletedEventReason is the reason associated with the event registered when build completes successfully.
	BuildCompletedEventReason = "BuildCompleted"
	// BuildCompletedEventMessage is the message associated with the event registered when build completes successfully.
	BuildCompletedEventMessage = "Build %s/%s completed successfully"
	// BuildFailedEventReason is the reason associated with the event registered when build fails.
	BuildFailedEventReason = "BuildFailed"
	// BuildFailedEventMessage is the message associated with the event registered when build fails.
	BuildFailedEventMessage = "Build %s/%s failed"
	// BuildCancelledEventReason is the reason associated with the event registered when build is cancelled.
	BuildCancelledEventReason = "BuildCancelled"
	// BuildCancelledEventMessage is the message associated with the event registered when build is cancelled.
	BuildCancelledEventMessage = "Build %s/%s has been cancelled"
)
