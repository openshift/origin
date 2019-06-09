package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConsoleNotification is the extension for configuring openshift web console notifications.
type ConsoleNotification struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ConsoleNotificationSpec `json:"spec"`
}

// ConsoleNotificationSpec is the desired console notification configuration.
type ConsoleNotificationSpec struct {
	// text is the visible text of the notification.
	Text string `json:"text"`
	// location is the location of the notification in the console.
	// +optional
	Location ConsoleNotificationLocation `json:"location,omitempty"`
	// link is an object that holds notification link details.
	// +optional
	Link *Link `json:"link,omitempty"`
	// color is the color of the text for the notification as CSS data type color.
	// +optional
	Color string `json:"color,omitempty"`
	// backgroundColor is the color of the background for the notification as CSS data type color.
	// +optional
	BackgroundColor string `json:"backgroundColor,omitempty"`
}

// ConsoleNotificationLocationSelector is a set of possible notification targets
// to which a notification may be appended.
type ConsoleNotificationLocation string

const (
	// BannerTop indicates that the notification should appear at the top of the console.
	BannerTop ConsoleNotificationLocation = "BannerTop"
	// BannerBottom indicates that the notification should appear at the bottom of the console.
	BannerBottom ConsoleNotificationLocation = "BannerBottom"
	// BannerTopBottom indicates that the notification should appear both at the top and at the bottom of the console.
	BannerTopBottom ConsoleNotificationLocation = "BannerTopBottom"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleNotificationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []ConsoleNotification `json:"items"`
}
