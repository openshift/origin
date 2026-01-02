package v1

// Represents a standard link that could be generated in HTML
type Link struct {
	// text is the display text for the link
	Text string `json:"text"`
	// href is the absolute URL for the link. Must use https:// for web URLs or mailto: for email links.
	// +kubebuilder:validation:Pattern=`^(https://|mailto:)`
	Href string `json:"href"`
}
