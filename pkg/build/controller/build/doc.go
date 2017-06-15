// The BuildController is responsible for implementing a build's lifecycle,
// which can be represented by a simple state machine:
//
//  Active States:
//  +-------+      +-------------+      +-------------+
//  |  New  |  ->  | Pending (p) |  ->  | Running (p) |
//  +-------+      +-------------+      +-------------+
//
//  Terminal States:
//  +---------------+    +------------+    +-----------+    +-------+
//  | Succeeded (p) |    | Failed (p) |    | Cancelled |    | Error |
//  +---------------+    +------------+    +-----------+    +-------+
//
//  (p) denotes that a pod is associated with the build at that state
//
// A transition is valid from any active state to any terminal state.
// Transitions that are not valid:
// Pending -> New
// Running -> New
// Running -> Pending
// Any terminal state -> Any other state
//
// Following is a brief description of each state:
//
// Active States:
//
// New - this is the initial state for a build. It will not have a pod
// associated with it. When the controller sees a build in this state, it
// needs to determine whether it can create a pod for it and move it to
// Pending. A build can only transition from New to Pending if the following
// conditions are met:
// - If the build output is set to an ImageStreamTag, the ImageStream must
//   exist and must have a valid Docker reference.
// - The policy associated with the build (Serial, Parallel, SerialLatestOnly)
//   must allow the build to be created. For example, if there is another build
//   from the same BuildConfig already running and the policy is Serial,
//   the current build must remain in the New state.
//
// Pending - a build is set in this state when a build pod has been created.
// If the pod is either New or Pending state, the build will remain in Pending
// state. The pod could remain in Pending state for a long time if the push secret
// it needs to mount is not present. The build controller will check if the
// push secret exists, and if not, it will update the build reason and message
// with that information.
//
// Running - a build is updated to this state if the corresponding pod is in the
// Running state.
//
// Terminal States
// Once a build enters a terminal state, it must notify its corresponding policy
// that it has completed. That way the next build can be moved off of the New
// state if for example the BuildConfig uses the Serial policy. In general, the
// build controller updates the build's state based on the pod state. The one
// exception is the Failed state which can be set by the build pod itself when it
// updates the build/details subresource. This is done so that the build pod can
// set the reason/message for the build failure. Because the build controller
// can also update the reason/message while processing a build, the build storage
// prevents updates to the reason/message after the build's phase has been changed
// to Failed.
//
// Succeeded/Failed - reflect the final state of the build pod. The build's
//   Status.Phase field can be set to Failed by the build pod itself.
//
// Cancelled - is set when the build is cancelled from one of the active states.
//
// Error - is set when the build pod is deleted while the build is running or
// the build pod is in an invalid state when the build completes (for example, it
// has no containers).

package build
