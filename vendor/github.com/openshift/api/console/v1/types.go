package v1

// Represents a standard link that could be generated in HTML
type Link struct {
	// text is the display text for the link
	Text string `json:"text"`
	// href is the absolute secure URL for the link (must use https)
	Href string `json:"href"`
}
