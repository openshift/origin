package rhcos

// Extensions is data specific to Red Hat Enterprise Linux CoreOS
type Extensions struct {
	AwsWinLi  *AwsWinLi  `json:"aws-winli,omitempty"`
	AzureDisk *AzureDisk `json:"azure-disk,omitempty"`
}

// AzureDisk represents an Azure disk image that can be imported
// into an image gallery or otherwise replicated, and then used
// as a boot source for virtual machines.
type AzureDisk struct {
	// Release is the source release version
	Release string `json:"release"`
	// URL to an image already stored in Azure infrastructure
	// that can be copied into an image gallery.  Avoid creating VMs directly
	// from this URL as that may lead to performance limitations.
	URL string `json:"url,omitempty"`
}

// AwsWinLi represents prebuilt AWS Windows License Included Images.
type AwsWinLi = ReplicatedImage

// ReplicatedImage represents an image in all regions of an AWS-like cloud
// This struct was copied from the release package to avoid an import cycle,
// and is used to describe all AWS WinLI Images in all regions.
type ReplicatedImage struct {
	Regions map[string]SingleImage `json:"regions,omitempty"`
}

// SingleImage represents a globally-accessible image or an image in a
// single region of an AWS-like cloud
// This struct was copied from the release package to avoid an import cycle,
// and is used to describe individual AWS WinLI Images.
type SingleImage struct {
	Release string `json:"release"`
	Image   string `json:"image"`
}
