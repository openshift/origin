/*
Package build contains the OpenShift build system.

It defines a Build resource type, along with associated storage and a controller
that executes builds and manages states of existing builds.

For newly created builds, the BuildController will assign a pod ID to the build
and set the build’s state to pending. This way, the assignment of the pod ID and
pending status is idempotent and won’t result in two BuildControllers
potentially scheduling two different pods for the same build.

For pending builds, the BuildController will attempt to create a pod to perform
the build. If the creation succeeds, it sets the build’s status to pending. If
the pod already exists, that means another BuildController already processed
this build in a pending state, resulting in a no-op. Any other pod creation
error would result in the build’s status being set to failed.

For running builds, the BuildController will monitor the status of the pod. If
the pod is still running and the build has exceeded its allotted execution time,
the BuildController will consider it failed. If the pod is terminated, the
BuildController will examine the exit codes for each of the pod’s containers. If
any exit code is non-zero, the build is marked as failed. Otherwise, it is
considered complete (successful).

Once the build has reached a terminal state (complete or failed), the
BuildController will delete the pod associated with the build. In the future, it
will be desirable to keep a record of the pod’s containers’ logs.
*/
package build
