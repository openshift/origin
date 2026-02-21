package storage

// ProvisionerType represents the type of CSI provisioner
type ProvisionerType string

const (
	// ProvisionerTypeBlock represents block storage provisioners (AWS EBS,Azure Disk, etc.)
	ProvisionerTypeBlock ProvisionerType = "block"
	// ProvisionerTypeFile represents file storage provisioners (AWS EFS, Azure File, etc.)
	ProvisionerTypeFile ProvisionerType = "file"
)

// FeatureSupport defines which features a provisioner supports
type FeatureSupport struct {
	SupportsBYOK bool
}

// ProvisionerInfo contains information about a CSI provisioner
type ProvisionerInfo struct {
	Name                     string
	Type                     ProvisionerType
	Features                 FeatureSupport
	EncryptionKeyName        string   // The parameter name for encryption key in StorageClass
	ManagedStorageClassNames []string // List of managed/preset storage classes name
}

// PlatformConfig contains configuration for a cloud platform
type PlatformConfig struct {
	Provisioners        []ProvisionerInfo
	DefaultStorageClass string
}

// Platforms maps cloud platforms to their configuration
var Platforms = map[string]PlatformConfig{
	"aws": {
		Provisioners: []ProvisionerInfo{
			{
				Name: "ebs.csi.aws.com",
				Type: ProvisionerTypeBlock,
				Features: FeatureSupport{
					SupportsBYOK: true,
				},
				EncryptionKeyName:        "kmsKeyId",
				ManagedStorageClassNames: []string{"gp2-csi", "gp3-csi"},
			},
			{
				Name:                     "efs.csi.aws.com",
				Type:                     ProvisionerTypeFile,
				Features:                 FeatureSupport{},
				EncryptionKeyName:        "",
				ManagedStorageClassNames: []string{"efs-sc"},
			},
		},
		DefaultStorageClass: "gp3-csi",
	},
	"gcp": {
		Provisioners: []ProvisionerInfo{
			{
				Name: "pd.csi.storage.gke.io",
				Type: ProvisionerTypeBlock,
				Features: FeatureSupport{
					SupportsBYOK: true,
				},
				EncryptionKeyName:        "disk-encryption-kms-key",
				ManagedStorageClassNames: []string{"standard-csi", "ssd-csi"},
			},
			{
				Name:                     "filestore.csi.storage.gke.io",
				Type:                     ProvisionerTypeFile,
				Features:                 FeatureSupport{},
				EncryptionKeyName:        "",
				ManagedStorageClassNames: []string{"filestore-csi"},
			},
		},
		DefaultStorageClass: "standard-csi",
	},
	"azure": {
		Provisioners: []ProvisionerInfo{
			{
				Name: "disk.csi.azure.com",
				Type: ProvisionerTypeBlock,
				Features: FeatureSupport{
					SupportsBYOK: true,
				},
				EncryptionKeyName:        "diskEncryptionSetID",
				ManagedStorageClassNames: []string{"managed-csi"},
			},
			{
				Name:                     "file.csi.azure.com",
				Type:                     ProvisionerTypeFile,
				Features:                 FeatureSupport{},
				EncryptionKeyName:        "",
				ManagedStorageClassNames: []string{"azurefile-csi"},
			},
		},
		DefaultStorageClass: "managed-csi",
	},
	"ibmcloud": {
		Provisioners: []ProvisionerInfo{
			{
				Name: "vpc.block.csi.ibm.io",
				Type: ProvisionerTypeBlock,
				Features: FeatureSupport{
					SupportsBYOK: true,
				},
				EncryptionKeyName: "encryptionKey",
				ManagedStorageClassNames: []string{
					"ibmc-vpc-block-10iops-tier",
					"ibmc-vpc-block-5iops-tier",
					"ibmc-vpc-block-custom"},
			},
		},
		DefaultStorageClass: "ibmc-vpc-block-10iops-tier",
	},
}

// GetProvisionersByPlatform returns all provisioners for a given platform
func GetProvisionersByPlatform(platform string) []ProvisionerInfo {
	if config, ok := Platforms[platform]; ok {
		return config.Provisioners
	}
	return nil
}

// GetProvisionerByName finds a provisioner by its name across all platforms
func GetProvisionerByName(provisioner string) *ProvisionerInfo {
	for _, config := range Platforms {
		for _, p := range config.Provisioners {
			if p.Name == provisioner {
				return &p
			}
		}
	}
	return nil
}

// GetBYOKProvisioners returns provisioners that support BYOK for a given platform
func GetBYOKProvisioners(platform string) []ProvisionerInfo {
	var result []ProvisionerInfo
	provisioners := GetProvisionersByPlatform(platform)
	for _, prov := range provisioners {
		if prov.Features.SupportsBYOK {
			result = append(result, prov)
		}
	}
	return result
}

// GetProvisionersByType returns provisioners of a specific type for a given platform
func GetProvisionersByType(platform string, provType ProvisionerType) []ProvisionerInfo {
	var result []ProvisionerInfo
	provisioners := GetProvisionersByPlatform(platform)
	for _, prov := range provisioners {
		if prov.Type == provType {
			result = append(result, prov)
		}
	}
	return result
}

// GetProvisionerNames returns just the names of provisioners for a given platform
func GetProvisionerNames(platform string) []string {
	provisioners := GetProvisionersByPlatform(platform)
	names := make([]string, len(provisioners))
	for i, p := range provisioners {
		names[i] = p.Name
	}
	return names
}

// GetBYOKProvisionerNames returns names of provisioners that support BYOK for a given platform
func GetBYOKProvisionerNames(platform string) []string {
	byokProvisioners := GetBYOKProvisioners(platform)
	names := make([]string, len(byokProvisioners))
	for i, p := range byokProvisioners {
		names[i] = p.Name
	}
	return names
}

// GetDefaultStorageClass returns the default storage class for a given platform
func GetDefaultStorageClass(platform string) string {
	if config, ok := Platforms[platform]; ok {
		return config.DefaultStorageClass
	}
	return ""
}
