package v1

import "k8s.io/apimachinery/pkg/runtime"

var _ runtime.NestedObjectDecoder = &MasterConfig{}

// DecodeNestedObjects handles encoding RawExtensions on the MasterConfig, ensuring the
// objects are decoded with the provided decoder.
func (c *MasterConfig) DecodeNestedObjects(d runtime.Decoder) error {
	// decoding failures result in a runtime.Unknown object being created in Object and passed
	// to conversion
	for k, v := range c.AdmissionConfig.PluginConfig {
		DecodeNestedRawExtensionOrUnknown(d, &v.Configuration)
		c.AdmissionConfig.PluginConfig[k] = v
	}
	if c.OAuthConfig != nil {
		for i := range c.OAuthConfig.IdentityProviders {
			DecodeNestedRawExtensionOrUnknown(d, &c.OAuthConfig.IdentityProviders[i].Provider)
		}
	}
	DecodeNestedRawExtensionOrUnknown(d, &c.AuditConfig.PolicyConfiguration)
	return nil
}

var _ runtime.NestedObjectEncoder = &MasterConfig{}

// EncodeNestedObjects handles encoding RawExtensions on the MasterConfig, ensuring the
// objects are encoded with the provided encoder.
func (c *MasterConfig) EncodeNestedObjects(e runtime.Encoder) error {
	for k, v := range c.AdmissionConfig.PluginConfig {
		if err := EncodeNestedRawExtension(e, &v.Configuration); err != nil {
			return err
		}
		c.AdmissionConfig.PluginConfig[k] = v
	}
	if c.OAuthConfig != nil {
		for i := range c.OAuthConfig.IdentityProviders {
			if err := EncodeNestedRawExtension(e, &c.OAuthConfig.IdentityProviders[i].Provider); err != nil {
				return err
			}
		}
	}
	if err := EncodeNestedRawExtension(e, &c.AuditConfig.PolicyConfiguration); err != nil {
		return err
	}
	return nil
}

// DecodeNestedRawExtensionOrUnknown
func DecodeNestedRawExtensionOrUnknown(d runtime.Decoder, ext *runtime.RawExtension) {
	if ext.Raw == nil || ext.Object != nil {
		return
	}
	obj, gvk, err := d.Decode(ext.Raw, nil, nil)
	if err != nil {
		unk := &runtime.Unknown{Raw: ext.Raw}
		if runtime.IsNotRegisteredError(err) {
			if _, gvk, err := d.Decode(ext.Raw, nil, unk); err == nil {
				unk.APIVersion = gvk.GroupVersion().String()
				unk.Kind = gvk.Kind
				ext.Object = unk
				return
			}
		}
		// TODO: record mime-type with the object
		if gvk != nil {
			unk.APIVersion = gvk.GroupVersion().String()
			unk.Kind = gvk.Kind
		}
		obj = unk
	}
	ext.Object = obj
}

// EncodeNestedRawExtension will encode the object in the RawExtension (if not nil) or
// return an error.
func EncodeNestedRawExtension(e runtime.Encoder, ext *runtime.RawExtension) error {
	if ext.Raw != nil || ext.Object == nil {
		return nil
	}
	data, err := runtime.Encode(e, ext.Object)
	if err != nil {
		return err
	}
	ext.Raw = data
	return nil
}
