package v1alpha1

// ImageDigestFormat is a type that conforms to the format host[:port][/namespace]/name@sha256:<digest>.
// The digest must be 64 characters long, and consist only of lowercase hexadecimal characters, a-f and 0-9.
// The length of the field must be between 1 to 447 characters.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=447
// +kubebuilder:validation:XValidation:rule=`(self.split('@').size() == 2 && self.split('@')[1].matches('^sha256:[a-f0-9]{64}$'))`,message="the OCI Image reference must end with a valid '@sha256:<digest>' suffix, where '<digest>' is 64 characters long"
// +kubebuilder:validation:XValidation:rule=`(self.split('@')[0].matches('^([a-zA-Z0-9-]+\\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9-_]{0,61}/)?[a-zA-Z0-9-_.]*?$'))`,message="the OCI Image name should follow the host[:port][/namespace]/name format, resembling a valid URL without the scheme"
type ImageDigestFormat string
