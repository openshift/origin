# ScheduledJob Controller

## Abstract
A proposal for implementing a new controller - ScheduledJob controller - which
will be responsible for managing time based jobs, namely:
* once at a specified point in time,
* repeatedly at a specified point in time.

There is already an upstream discussion regarding that particular subject:
* Distributed CRON jobs [#2156](https://github.com/GoogleCloudPlatform/kubernetes/issues/2156)

There are also similar solutions available already:
* [Mesos Chronos](https://github.com/mesos/chronos)
* [Quartz](http://quartz-scheduler.org/)


## Use Cases
1. Be able to schedule a job execution at a given point in time.
1. Be able to create a repetitive job, eg. database backup, sending emails.


## Motivation
ScheduledJobs are needed for performing all time related actions, namely backups,
report generation and alike.  Each of these tasks should be allowed to perform
repeatedly (once a day/month, etc.) or once at a given point in time.


## Implementation
ScheduledJob controller relies heavily on the [Job Controller API](https://github.com/openshift/origin/blob/master/docs/proposals/job.md)
for running actual jobs, on top of which it adds information regarding the date
and time part according to ISO8601 format.

The new `ScheduledJob` object will have the following content:

```go
// ScheduledJob represents the configuration of a single scheduled job.
type ScheduledJob struct {
    TypeMeta
    ObjectMeta

    // Spec is a structure defining the expected behavior of a job, including the schedule.
    Spec ScheduledJobSpec

    // Status is a structure describing current status of a job.
    Status ScheduledJobStatus
}

// ScheduledJobList is a collection of scheduled jobs.
type ScheduledJobList struct {
    TypeMeta
    ListMeta

    Items []ScheduledJob
}
```

`ScheduledJobSpec` structure is defined to contain all the information how the actual
job execution will look like, including the `JobSpec` from [Job Controller API](https://github.com/openshift/origin/blob/master/docs/proposals/job.md)
and the schedule in ISO8601 format.

```go
// ScheduledJobSpec describes how the job execution will look like and when it will actually run.
type ScheduledJobSpec struct {

    // Spec is a structure defining the expected behavior of a job.
    Spec JobSpec

    // Schedule contains the schedule in ISO8601 format, eg.
    // - 2015-07-21T14:00:00Z - represents date and time in UTC
    // - R/2015-07-21T14:00:00Z/P1D - represents endlessly repeating interval (1 day), starting from given date
    Schedule string

    // SkipOutdated specifies that if AllowConcurrent is false, only the newest job
    // will be started (default: true), ignoring executions that missed their schedule.
    SkipOutdated bool

    // BlockOnFailure suspends scheduling of next job runs after a failed one.
    BlockOnFailure bool

    // AllowConcurrent specified whether concurrent jobs may be started.
    AllowConcurrent bool
}
```

`ScheduledJobStatus` structure is defined to contain some information for scheduled
job executions (up to a limited amount).  The structure holds objects in three lists:
* "Pending" list is used as a way to stack executions for job preventing concurrent
runs, and also to trigger an execution for a job defined without time based scheduling;
* "Running" list contains all actually running jobs with detailed information;
* "Failed" list contains information about recently failed jobs.

```go
// ScheduledJobStatus represents the current state of a Job.
type ScheduledJobStatus struct {
    // PendingExecutions are the job runs pending for execution
    PendingExecutions []JobStatus

    // CurrentExecutions are the job run currently executing
    RunningExecutions []JobStatus

    // CompletedExecutions tracks previously scheduled jobs (up to a limited amount)
    CompletedExecutions []JobStatus

    // FailedExecutions tracks previously failed jobs
    FailedExecutions []JobStatus

    // ScheduleCount tracks the amount of successful executions for this job
    CompletedCount int

    // FailedCount tracks the amount of failures for this job
    FailedCount int
}
```
