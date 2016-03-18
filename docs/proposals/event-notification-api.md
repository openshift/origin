# Event Notification API

This proposal describes new Origin API resources which enable the integration of Origin with external event notification systems. New features:

1. For cluster operators, the ability to seamlessly integrate Origin `Events` with external notification providers.
2. For users, the ability to seamlessly configure external notifications using the standard Origin console, tooling, and API.

This proposal does not include an implementation of a specific event notification provider for Origin. The scope of this proposal is limited to the minimal API necessary to enable the proposed features above.

## Design

An `Event` notifier integrating with Origin requires user-provided data to inform the notifier about interested users within a namespace and provide context for notification dispatch. Events are scoped to a namespace, and many users can access a namespace. Each `Event` could result in multiple notifications depending on the number of interested users and their notification configuration.

For the proposed API, an `Event` type is uniquely identified by a key computed as `{Event.InvolvedObject.Kind}:{Event.Reason}`.

### API

The Event Notification API provides:

1. A way for cluster operators to declare arbitrary notification types.
2. A way for cluster operators to declare arbitrary notification constraints per event type.
3. A way for users to declare their notification preferences per event type.

```go
// EventNotificationType is an event notification type supported generally the cluster.
type EventNotificationType struct {
  unversioned.TypeMeta
  kapi.ObjectMeta
  // Description is the human readable description of the type.
  Description string
}

// ClusterEventNotification specifies which event notification types are supported
// for a given event type within the cluster. There should only be one ClusterEventNotification
// document in the cluster.
type ClusterEventNotification struct {
  unversioned.TypeMeta
  kapi.ObjectMeta

  // SupportedNotifications maps event types to supported notification types.
  SupportedNotifications map[string][]string
}

// UserEventNotification specifies how a user should be notified of a given event.
type UserEventNotification struct {
  unversioned.TypeMeta
  // Name is a User.Name.
  kapi.ObjectMeta
  // Notifications is an event type mapped to the names of EventNotificationTypes
  // which are desired for the event.
  Notifications map[string][]string
}
```

### Console

The Origin console can provide a management view for user notification settings based on the cluster configuration, dynamically generating a UI using the available metadata. If the cluster has no event notification types, the feature can be disabled entirely in the console or presented in a way that indicates the cluster doesn't support any notifications.

### CLI

TODO: Write me.

## Use case: Cluster implements email and text message notifications

The cluster operator supports dispatching emails and text messages to users via a third-party system. The integration is provided by a custom application deployed in Origin in a system namespace. The cluster operator wants to make email notifications available to users when `Pod:PodFailed` events occur, and make both email and text notifications available for `DeploymentConfig:Failed` events.

In namespace *N*, users `alice` and `bob` want email for `Pod:PodFailed` events, while user `jane` wants both email and text notifications for `DeploymentConfig:Failed`.

The following resources support this use case:

```yaml
kind: EventNotificationType
name: email
description: Email notification.

kind: EventNotificationType
name: text
description: Text message notification.

kind: ClusterEventNotification
name: default
supportedNotifications:
- "Pod:PodFailed":
  - email
- "DeploymentConfig:Failed":
  - email
  - text

kind: UserEventNotification
name: alice
notifications:
  "Pod:PodFailed":
  - email

kind: UserEventNotification
name: bob
notifications:
  "Pod:PodFailed":
  - email

kind: UserEventNotification
name: jane
notifications:
  "DeploymentConfig:Failed":
  - email
  - text
```

The custom integration application has the following behavior:

1. Receive Event resources using the Origin watch API.
2. For an event, find `UserEventNotifications` in the event's namespace containing `notifications` matching the event type.
3. Dispatch `email` or `text` commands to the third-party mailing HTTP API targetting any users with a matching notification configuration.

Users can update their notifications via the Origin console, and their preferences are reflected immediately in the integration application. Origin has no explicit awareness of the integration application or third-party notification system.

## Comparison With Custom Eventing

TODO: Explain why it's better to rely on native events emitted by Kubernetes/OpenShift as rather than synthesizing events on the fly based on watching non-Event API types.
