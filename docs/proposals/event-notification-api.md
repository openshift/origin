# Event Notification API

This proposal describes new Origin API resources which enable the integration of Origin with external event notification providers. New features:

1. For notification providers integrating with Openshift, an API to extend Origin with arbitrarily defined notification types.
2. For users, the ability to configure notifications for events using the standard Origin console, tooling, and API.

This proposal does not include an implementation of a specific event notification provider for Origin. The scope of this proposal is limited to the minimal API necessary to enable the proposed features above.

## Design

An `Event` notification provider integrating with Origin requires user-provided data to inform the provider about users within a namespace interested in an event. Events are scoped to a namespace, and many users can access a namespace. Each `Event` could result in multiple notifications depending on the number of interested users and their notification configuration.

For the proposed API, an `Event` type is uniquely identified by a key computed as `{Event.InvolvedObject.Kind}:{Event.Reason}`.

### API

The Event Notification API provides:

1. A way for notification providers to augment Origin with support for provider-defined notification types.
2. A way for users to declare their notification preferences per event type.
3. A way for Origin to augment Event types with useful presentation metadata.

```go
// EventNotificationType is a type of notification the cluster supports. Provider implementations
// declare their support for notifications by creating EventNotificationTypes.
type EventNotificationType struct {
  unversioned.TypeMeta
  kapi.ObjectMeta
  // Description is the human readable description of the notification type.
  Description string
}

// UserEventNotification specifies how a user should be notified of a given event.
type UserEventNotification struct {
  unversioned.TypeMeta
  // Name is a User.Name.
  kapi.ObjectMeta
  // Notifications maps an event type to list of EventNotificationType references. A user
  // can be notified of an event in multiple ways.
  Notifications map[string][]ObjectReference
}

// TODO: define a small mapping API which translates event types to human readable descriptions. The Event.Message field is insufficient because it's object instance-specific.
```

### Console

With this API, the the Origin console could provide a UI for user notification management by presenting a list of possible `Event` types, and for each type, a selection of `EventNotificationTypes`. If the cluster has no `EventNotificationTypes`, the management feature can be disabled entirely in the console or presented in a way that indicates the cluster doesn't support any notifications.

### CLI

TODO: Write me.

## Use case: Separate email and SMS notification providers

Two notification providers are implemented as applications in the Origin cluster: one for email notification and one for SMS notification, both via third-party services. Each provider lives in its own namespace within the cluster.

In namespace *N*, users `alice` and `bob` want email for `Pod:PodFailed` events, while user `jane` wants both email and text notifications for `DeploymentConfig:Failed`.

The following resources express this cluster configuration:

```yaml
kind: EventNotificationType
namespace: email-provider
name: email
description: Email notification.

kind: EventNotificationType
namespace: sms-provider
name: sms
description: SMS notification.

kind: UserEventNotification
name: alice
notifications:
  "Pod:PodFailed":
  - kind: ObjectReference
    namespace: email-provider
    name: email

kind: UserEventNotification
name: bob
notifications:
  "Pod:PodFailed":
  - kind: ObjectReference
    namespace: email-provider
    name: email

kind: UserEventNotification
name: jane
notifications:
  "Pod:PodFailed":
  - kind: ObjectReference
    namespace: email-provider
    name: email
  "DeploymentConfig:Failed":
  - kind: ObjectReference
    namespace: email-provider
    name: email
  - kind: ObjectReference
    namespace: sms-provider
    name: sms
```

The email notification provider application has the following behavior:

1. Receive Event resources using the Origin watch API.
2. For an event, find `UserEventNotifications` in the event's namespace containing `notifications` matching the event type.
3. For any `email-provider/email` references in a `UserEventNotification`, send an email for that user using the third-party email service API.

The SMS notification provider application has the following behavior:

1. Receive Event resources using the Origin watch API.
2. For an event, find `UserEventNotifications` in the event's namespace containing `notifications` matching the event type.
3. For any `sms-provider/sms` references in a `UserEventNotification`, send an SMS message for that user using the third-party SMS service API.

Users can update their notification preferences via the Origin console, CLI, or API and their preferences are reflected immediately in the notification providers. Origin has no explicit awareness of the provider applications or third-party notification systems.

## Comparison With Custom Eventing

TODO: Explain why it's better to rely on native events emitted by Kubernetes/OpenShift rather than synthesizing events on the fly based on watching non-Event API types.

