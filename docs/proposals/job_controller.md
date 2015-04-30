**Abstract**

First basic implementation proposal for a Job controller. Several exiting issues were already created regarding that particular subject:

    Distributed CRON jobs in k8s #2156
    Job Controller #1624

Several features also already exist that could be used by external tools to trigger batch execution on one pod within k8 cluster.

    Create a standalone pod (not linked to any existing replication controllers)
    Execute a new process within a given container from a running pod.

**Motivation**

The main goal is to provide a new pod controller type, called Job, with the following characteristics:

* Be able to start of one or several pods tracked as a single entity representing a one execution of this job. Final job execution status will reflect the completion status of all pods started for this run.

* Ability to trigger a new job execution either on demand (triggered through server API by an external agent or by another k8 subsystem), or periodically based on a time-based  schedule defined in job specification part (using ISO8601 syntax notation).

* Be able to restart a limited number of time failing containers of pods created by one job execution (using "Restart on failure" logic extended by a maximal restart count).

* Be able to limit the execution time of pods created by this job.

* Be able to provide status information about pending and current job executions, along with a (limited) history of previous job runs (containing start/stop times with completion exit code)

* (Future evolution) Trigger jobs based on success/failure of others.

    
**Job basic definition**

The new Job object json definition for a basic implementation will have the following content:
```
{
  "apiVersion": "v1beta3",
  "kind": "Job",
  "metadata" : {
    "name": "myJob"
    "labels" : {
      "name": "myJob"
    }
  }
  "spec": {
    "schedule": {
      "timeSpec": "R5/T01:00:00/PT01",
      "skipOutdated": true
      "blockOnFailure": false
    },
    "allowConcurrentRun": false,
    "maxRestart": 2,
    "execTimeout": 100,
    "PodCount": 1,
    "selector": { "name": "myjob"},
    "template": {
      "metadata" : {
        "name": "myJobPodTemplate"
        "labels" : {
          "name": "myJobPodTemplate"
        }
      }
      "spec": {
        "restartPolicy": "OnFailure"
        "containers": [{
          "name": "job-container",
          "image": "app/job",
          ...
        }]
      }
    }
  }
}
```
**Fields definition:**

Compared to replication controller, replica count is replaced by the PodCount field (number of Pods started simultaneously for a single job execution run). 
All other fields are characteritic of this new controller type:

* "maxRestart": (default to 1) maximal number of times a failing container is allowed to be restarted.
* "execTimeout": (not restricted by default) maximal duration a container within a pod started by the Job controller is allowed to run. Reaching this limit leads to the container to be killed.
* "PodCount": (default to 1) number of pods started by a single run of the Job.
* "allowConcurrentRun": (default to true) dictates if concurrent runs of this Job are allowed or not.

* "schedule": (optional) sub structure defining scheduling details 
  * "timeSpec": describes the job schedule, expressed (using the ISO 8601) with a 3 parts dash separated string: "R[n]/[start date]/[duration]"
  The first part gives the number of repetitions of the job (omitting this number means infinite repetitions)
  The second part gives the starting time of the scheduling period.
  The third part is the duration between successive runs of the scheduled pod(s).
  * "suspendOnFailure": dictated if job schedule should be suspended in case of failure of the last completed job run.
  * "skipOutdated": dictates if pending scheduled job runs should be discarded in favour of the latest one. This covers the case of a job having suspended its execution after a failed run, with several schedule points reached in the meantime (or the case of a long lasting run with concurrent execution disable). Instead of stacking pending executions, only the latest one is kept (and intermediate pending runs discarded).

**Implementation details**

Job objects are stored in a dedicated registry directory key (separated from the directory key used for replication controllers).

Similarly to replication controllers, a dedicated manager takes care of all jobs existing in the system, performing the following tasks:

* Executes a loop on the full list of defined jobs, calling on each a defined synchronization call-back, each execution loop being spaced by a configuration defined time interval (with default value of 5 seconds).
* Performs a (recursive) watch of the registry directory key containing jobs definition. For each job newly defined or updated in the system, the same synchronization routine is invoked.

This job manager provides a way for each job defined with schedule information to setup a dedicated timer for next schedule time. This allows a better accuracy of start execution time for pods launched by jobs and remove dependency on the interval value set for the synchronization loop.

The maximal restart count and maximal execution time for pods containers started by a job will be enforced at kubelet level, using dedicated resource limits added by the job in pod's containers definition. Two new resource types ("execution time" and "execution count") are introduced, that are used to add in the container's resources limits the figures defined in job specification and taken into account by the kubelet module (container is killed if the execution time is reached and restarted in case of failure exit up to this maximal count).

In order to precisely track job pods, 2 specific labels are pushed in created pod metadata: 
* the job name ("jobName")
* A tag ("jobTag", either a generated UID, or simply the schedule date of this job execution) allowing to group all pods started started for a given job execution.

Along with labels, specific environment variables are also dynamically added in created pods definition: the number of pods started for this job execution and for each one its index number in the started pods list.

Job has the responsibility to advertise job run completion status (success or failure) through events, to perform the necessary clean-up in pods registry, to provide a detailed status of active job runs and the completion status of previous completed runs up to a given amount of history).
Collecting the standard output/error of pod's containers is not covered by this design (a common solution for containers started by any controller is needed).

**API types definition:**

The following API objects can be introduced:

```
type JobScheduleSpec struct {

    // TimeSpec contains the schedule in ISO8601 format.
    TimeSpec string `json:"timeSpec"`

    // SkipOutdated specifies that if AllowConcurrent is false, only the latest pending job
    // that is waiting for the currently executing one to complete must be started (default: true)
    SkipOutdated bool `json:"skipOutdated"`
    
    // BlockOnFailure suspends scheduling of next job runs after a failed one
    BlockOnFailure bool `json:"blockOnFailure"`
}

type JobSpec struct {

    // JobScheduleSpec contains scheduling related infos
    JobScheduleSpec ScheduleSpec `json:"scheduleSpec"`

    // AllowConcurrent specified whether concurrent jobs may be started
    // (covers the case where new job schedule time is reached, while
    // other previously started jobs are still running)
    AllowConcurrent bool `json:"allowConcurrent"`

    // ExecTimeout specifies the max amount of time a job is allowed to run (in seconds).
    ExecTimeout int `json:"execTimeout"`

    // MaxRestart specifies the max number of restarts for a failing scheduled pod.
    MaxRestart int `json:"maxRestart"`
    
    // PodCount specifies the numbe of pods to start for each job run
    PodCount int `json:"podCount"`
    
    // Pod selector for that controller
    Selector map[string]string `json:"selector"`

    // Reference to stand alone PodTemplate
    TemplateRef *ObjectReference `json:"templateRef,omitempty"`

    // Embedded pod template
    Template *PodTemplateSpec `json:"template,omitempty"`
}

type JobExecStatus string 
const (
    JobExecPending JobExecStatus   = "pending"
    JobExecRunning JobExecStatus   = "running"
    JobExecCompleted JobExecStatus = "completed"
)

type JobExecution struct {
    // CreatedAt gives job creation time
    CreatedAt util.Time

    // StartedAt gives job starting time
    StartedAt util.Time

    // CompletedAt
    CompletedAt util.Time

    // Success gives job completion result
    Success bool

    // JobExecStatus
    Status JobExecStatus

    // Tag is added in labels of pod(s) created for this job execution.  It allows 
    // job object to safely group/track all pods started for one given job execution.
    Tag util.UID
}

// JobStatus represents the current state of Job 
type JobStatus struct {
    // PendingExecutions are the job runs pending for execution
    PendingExecutions []JobExecution      `json:"pendingExecutions"`

    // CurrentExecutions are the job run currently executing 
    // (With pods scheduled in the kubernetes cluster)
    RunningExecutions []JobExecution      `json:"runningExecutions"`

    // PreviousRunStates tracks previously scheduled jobs (up to a limited amount)
    CompletedExecution []JobExecution  `json:"completedExecution"`

    // ScheduleCount tracks the count of run for this job
    ScheduleCount int `json:"scheduleCount"`
}

// Job represents the configuration of a job controller.
type Job struct {
    TypeMeta   `json:",inline"`
    ObjectMeta `json:"metadata,omitempty"`

    // Spec defines the desired behavior of this job controller.
    Spec JobSpec `json:"spec,omitempty"`

    // Status is the current status of this job controller.
    Status JobStatus `json:"status,omitempty"`
}

// JobList is a collection of job controllers.
type JobList struct {
    TypeMeta `json:",inline"`
    ListMeta `json:"metadata,omitempty"`

    Items []Job `json:"items"`
}
```

JobExecution structure is defined to contain all informations for pending, running and completed job executions. The Job status structure holds objects of this type in three separated ordered lists (one by execution status). The "pending" execution list is used as a way to stack executions for job preventing concurrent runs, and also to trigger execution for job defined without time based scheduling. A new API server entry point is introduced to allow this on-demand job execution. Changes in this pending list is detected by the watch process in controller-manager module and the job instance called back takes the appropriate action (in the common case, creates pods and move this job execution in the running list) 

