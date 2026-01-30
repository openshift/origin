// Package core provides file utilities: permission constants, temp file creation, and template processing.
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/klog/v2"
)

// File permission constants for secure file handling.
const (
	SecureFileMode   os.FileMode = 0600 // Sensitive files (secrets, keys)
	ReadOnlyFileMode os.FileMode = 0400 // Read-only configs
	SecureDirMode    os.FileMode = 0700 // Sensitive directories
	StandardFileMode os.FileMode = 0644 // Non-sensitive files
	StandardDirMode  os.FileMode = 0755 // Standard directories
)

// WithLocalTempFile creates a temp file, executes fn with the path, then auto-cleans up.
//
//	err := WithLocalTempFile("secret-*.yaml", content, SecureFileMode, func(path string) error { return processFile(path) })
func WithLocalTempFile(pattern, content string, mode os.FileMode, fn func(path string) error) error {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return WrapError("create temp file", fmt.Sprintf("pattern=%s", pattern), err)
	}

	// Ensure cleanup
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			klog.V(4).Infof("Warning: failed to remove temp file %s: %v", tmpFile.Name(), err)
		}
	}()

	// Change file permissions
	if err := os.Chmod(tmpFile.Name(), mode); err != nil {
		return WrapError("set permissions", fmt.Sprintf("mode=%o file=%s", mode, tmpFile.Name()), err)
	}

	// Write content
	if _, err := tmpFile.WriteString(content); err != nil {
		return WrapError("write content to temp file", tmpFile.Name(), err)
	}

	// Close file
	if err := tmpFile.Close(); err != nil {
		return WrapError("close temp file", tmpFile.Name(), err)
	}

	klog.V(4).Infof("Created temporary file with mode %o: %s", mode, tmpFile.Name())

	// Execute function
	if err := fn(tmpFile.Name()); err != nil {
		return err
	}

	return nil
}

// CreateFromTemplate processes a template with placeholder substitution, returns temp file path and cleanup function.
//
//	tmpFile, cleanup, err := CreateFromTemplate(templatePath, map[string]string{"{NAME}": "master-0"})
func CreateFromTemplate(templatePath string, replacements map[string]string) (string, func(), error) {
	klog.V(4).Infof("Processing template: %s", templatePath)

	// Normalize path: if it starts with "testdata/", prepend "test/extended/" for validation
	normalizedPath := templatePath
	if strings.HasPrefix(templatePath, "testdata/") {
		normalizedPath = filepath.Join("test/extended", templatePath)
	}

	// Validate template path to prevent directory traversal attacks
	const allowedTemplateDir = "test/extended/testdata/edge-topologies/"
	if err := ValidateSafePath(normalizedPath, allowedTemplateDir); err != nil {
		return "", nil, WrapError("validate template path", templatePath, err)
	}

	// Use FixturePath to get the absolute path to the template file
	// FixturePath expects path components as separate arguments, not a single string with slashes
	pathComponents := strings.Split(templatePath, "/")
	absolutePath := exutil.FixturePath(pathComponents...)
	klog.V(4).Infof("Resolved template path to: %s", absolutePath)

	// Read the template file
	templateContent, err := os.ReadFile(absolutePath)
	if err != nil {
		return "", nil, WrapError("read template", fmt.Sprintf("%s (resolved from %s)", absolutePath, templatePath), err)
	}

	// Apply all placeholder replacements
	result := string(templateContent)
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// Create a temporary file with the processed content
	tmpFile, err := os.CreateTemp("", "template-*.yaml")
	if err != nil {
		return "", nil, WrapError("create temporary file for template", "", err)
	}

	// Write the processed content to the temporary file
	if _, err := tmpFile.WriteString(result); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", nil, WrapError("write processed template to temporary file", "", err)
	}

	// Close the file to flush contents
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", nil, WrapError("close temporary file", "", err)
	}

	// Create cleanup function to remove the temporary file
	cleanup := func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			klog.V(4).Infof("Failed to remove temporary file %s: %v", tmpFile.Name(), err)
		}
	}

	klog.V(4).Infof("Created temporary file: %s", tmpFile.Name())
	return tmpFile.Name(), cleanup, nil
}

// CreateResourceFromTemplate processes a template and creates a Kubernetes resource with "oc create -f".
//
//	err := CreateResourceFromTemplate(oc, templatePath, map[string]string{"{NAME}": "master-0", "{UUID}": uuid})
func CreateResourceFromTemplate(oc *exutil.CLI, templatePath string, replacements map[string]string) error {
	klog.V(4).Infof("Processing template: %s", templatePath)

	// Normalize path: if it starts with "testdata/", prepend "test/extended/" for validation
	normalizedPath := templatePath
	if strings.HasPrefix(templatePath, "testdata/") {
		normalizedPath = filepath.Join("test/extended", templatePath)
	}

	// Validate template path to prevent directory traversal attacks
	const allowedTemplateDir = "test/extended/testdata/edge-topologies/"
	if err := ValidateSafePath(normalizedPath, allowedTemplateDir); err != nil {
		return WrapError("validate template path", templatePath, err)
	}

	// Use FixturePath to get the absolute path to the template file
	// FixturePath expects path components as separate arguments, not a single string with slashes
	pathComponents := strings.Split(templatePath, "/")
	absolutePath := exutil.FixturePath(pathComponents...)
	klog.V(4).Infof("Resolved template path to: %s", absolutePath)

	// Read the template file
	templateContent, err := os.ReadFile(absolutePath)
	if err != nil {
		return WrapError("read template", fmt.Sprintf("%s (resolved from %s)", absolutePath, templatePath), err)
	}

	// Apply all placeholder replacements
	result := string(templateContent)
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// Use WithLocalTempFile to create temp file, run oc create, and auto-cleanup
	err = WithLocalTempFile("template-*.yaml", result, StandardFileMode, func(tmpPath string) error {
		_, err := oc.AsAdmin().Run("create").Args("-f", tmpPath).Output()
		if err != nil {
			return WrapError("create resource from template", templatePath, err)
		}
		klog.V(2).Infof("Successfully created resource from template: %s", templatePath)
		return nil
	})

	return err
}

// BackupResource saves a Kubernetes resource to a YAML file with secure permissions.
//
//	err := BackupResource(oc, "secret", secretName, "openshift-etcd", backupDir)
func BackupResource(oc *exutil.CLI, resourceType, name, namespace, backupDir string) error {
	output, err := oc.AsAdmin().Run("get").Args(resourceType, name, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		return WrapError("get resource", fmt.Sprintf("%s/%s from namespace %s", resourceType, name, namespace), err)
	}

	filename := filepath.Join(backupDir, fmt.Sprintf("%s.yaml", name))
	if err := os.WriteFile(filename, []byte(output), SecureFileMode); err != nil {
		return WrapError("write backup file", filename, err)
	}

	klog.V(2).Infof("Backed up %s/%s to %s", resourceType, name, filename)
	return nil
}

// RestoreResource creates a Kubernetes resource from a backup YAML file.
//
//	err := RestoreResource(oc, filepath.Join(backupDir, secretName+".yaml"))
func RestoreResource(oc *exutil.CLI, backupFile string) error {
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return NewError("restore resource", fmt.Sprintf("backup file not found: %s", backupFile))
	}

	_, err := oc.AsAdmin().Run("create").Args("-f", backupFile).Output()
	if err != nil {
		return WrapError("restore from backup", backupFile, err)
	}

	klog.V(2).Infof("Restored resource from %s", backupFile)
	return nil
}
