# Job Controller

## Abstract
A proposal for implementing a new controller - Job controller - which will be responsible
for managing pod(s) that require to run-once to a completion, in contrast to what
ReplicationController currently offers.

Several existing issues and PRs were already created regarding that particular subject:
* Job Controller [#1624](https://github.com/GoogleCloudPlatform/kubernetes/issues/1624)
* New Job resource [#7380](https://github.com/GoogleCloudPlatform/kubernetes/pull/7380)


## Use Cases
1. Be able to start of one or several pods tracked as a single entity.
1. Be able to implement basic batch oriented tasks.
1. Be able to get the job status.
1. Be able to limit the execution time for a job.
1. Be able to specify the number of instances performing a task.
1. Be able to specify triggers on job’s success/failure.


## Motivation
Jobs are needed for executing multi-pod computation to completion; a good example
here would be the ability to implement a MapReduce or Hadoop style workload.
Additionally this new controller should take over pod management logic we currently
have in certain OpenShift controllers, namely build controller.


## Implementation
Job controller is similar to replication controller in that they manage pods.
This implies they will follow the same controller framework that replication
controllers already defined.  The biggest difference between `Job` and
`ReplicationController` objects is the purpose; `ReplicationController`
ensures that a specified number of Pods are running at any one time, whereas
`Job` is responsible for keeping the desired number of Pods to a completion of
a task. This will be represented by the `RestartPolicy` which is required to
always take value of `RestartPolicyNever` or `RestartOnFailure`.


The new `Job` object will have the following content:

```go
// Job represents the configuration of a single job.
type Job struct {
    TypeMeta
    ObjectMeta

    // Spec is a structure defining the expected behavior of a job.
    Spec JobSpec

    // Status is a structure describing current status of a job.
    Status JobStatus
}

// JobList is a collection of jobs.
type JobList struct {
    TypeMeta
    ListMeta

    Items []Job
}
```

`JobSpec` structure is defined to contain all the information how the actual job execution
will look like.

```go
// JobSpec describes how the job execution will look like.
type JobSpec struct {

    // TaskCount specifies the desired number of pods the job should be run with.
    TaskCount int

    // Optional duration in seconds relative to the StartTime that the job may be active
    // before the system actively tries to terminate it; value must be positive integer
    ActiveDeadlineSeconds *int64

    // Selector is a label query over pods that should match the pod count.
    Selector map[string]string

    // Spec is the object that describes the pod that will be created when
    // executing a job.
    Spec PodSpec
}
```

`JobStatus` structure is defined to contain informations about pods executing
specified job.  The structure holds information about pods currently executing
the job.

```go
// JobStatus represents the current state of a Job.
type JobStatus struct {
    // Executions holds a detailed information about each of the pods running a job.
    Executions []JobExec

    // Completions is the number of pods successfully completed their job.
    Completions int

}

// JobExec represents the current state of a single execution of a Job.
type JobExec struct {
    // CreationTime represents time when the job execution was created
    CreationTime util.Time

    // StartTime represents time when the job execution was started
    StartTime util.Time

    // CompletionTime represents time when the job execution was completed
    CompletionTime util.Time

    // Phase represents the point in the job execution lifecycle.
    Phase JobExecPhase

    // Tag is added in labels of pod(s) created for this job execution.  It allows
    // job object to safely group/track all pods started for one given job execution.
    Tag util.UID
}

// JobExecPhase represents job execution phase at given point in time.
type JobExecPhase string

// These are valid JobExec phases.
const (
    // JobExecPending means the pod has been accepted by the system but one or more
    // pods has not been started.
    JobExecPending JobExecPhase   = "Pending"
    // JobExecRunning means that all pods have been started.
    JobExecRunning JobExecPhase   = "Running"
    // JobExecComplete means that all pods have terminated with an exit code of 0.
    JobExecComplete JobExecPhase = "Complete"
    // JobExecFailed means that all pods have terminated and at least one of
    // them terminated with non-zero exit code.
    JobExecFailed JobExecPhase = "Failed"
)
```

## Events
Job controller will be emitting the following events:
* JobStart
* JobFinish

## Future evolution
Below are the possible future extensions to the Job controller:
* Be able to create a chain of jobs dependent one on another.

## Discussion points:
* triggers
* replacing build controller (others?)
*
