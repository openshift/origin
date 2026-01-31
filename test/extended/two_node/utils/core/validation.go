// Package core provides validation utilities: resource names, SSH keys, paths, IPs, node names, and integer bounds to prevent security vulnerabilities.
package core

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// --- Error Pattern Standardization ---

// WrapError wraps an error with standardized context following the pattern: "failed to <operation> <details>: <original error>".
//
//	return WrapError("create temp file", "pattern=secret-*.yaml", err)
func WrapError(operation, details string, err error) error {
	if details != "" {
		return fmt.Errorf("failed to %s (%s): %w", operation, details, err)
	}
	return fmt.Errorf("failed to %s: %w", operation, err)
}

// NewError creates a new error with standardized context following the pattern: "failed to <operation> <details>".
//
//	return NewError("connect to hypervisor", "no SSH key provided")
func NewError(operation, details string) error {
	if details != "" {
		return fmt.Errorf("failed to %s: %s", operation, details)
	}
	return fmt.Errorf("failed to %s", operation)
}

// ValidationError creates a validation error with standardized format: "<field>: <reason>".
//
//	return ValidationError("node name", "contains invalid characters")
func ValidationError(field, reason string) error {
	return fmt.Errorf("%s: %s", field, reason)
}

// ValidationWarning logs a non-fatal validation issue that doesn't prevent test execution.
// Use for best practices or environment-specific checks that may differ in CI vs prod.
//
//	ValidationWarning("config", "suboptimal setting detected but will continue")
func ValidationWarning(field, reason string) {
	e2e.Logf("VALIDATION WARNING [%s]: %s", field, reason)
}

// ValidateResourceName ensures a name is safe for shell commands (prevents command injection).
//
//	if err := ValidateResourceName(vmName, "VM"); err != nil { return err }
func ValidateResourceName(name, resourceType string) error {
	if name == "" {
		return ValidationError(resourceType+" name", "cannot be empty")
	}

	// Check for shell metacharacters and control characters
	if strings.ContainsAny(name, ";&|$`\\\"'<>()[]{}!*?~\n\r\t") {
		return ValidationError(resourceType+" name", fmt.Sprintf("contains invalid characters: %s", name))
	}

	// Check for hidden characters and control codes
	for _, char := range name {
		if char < 32 || char == 127 {
			return ValidationError(resourceType+" name", "contains control characters")
		}
	}

	// Validate against regex pattern for expected format
	// Must start with alphanumeric and contain only alphanumeric, dots, hyphens, underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`, name)
	if !matched {
		return ValidationError(resourceType+" name", "must start with alphanumeric and contain only alphanumeric, dots, hyphens, underscores")
	}

	// Check length limits (DNS subdomain name max length)
	if len(name) > 253 {
		return ValidationError(resourceType+" name", "too long (max 253 characters)")
	}

	return nil
}

// ValidateSSHKeyPermissions checks SSH private key has secure permissions (0600 or 0400).
// Logs a warning for insecure permissions but continues to support CI environments where
// file permissions may be set differently (e.g., 0644).
//
//	if err := ValidateSSHKeyPermissions(privateKeyPath); err != nil { return err }
func ValidateSSHKeyPermissions(keyPath string) error {
	info, err := os.Stat(keyPath)
	if err != nil {
		return WrapError("access SSH key", keyPath, err)
	}

	if info.IsDir() {
		return ValidationError("SSH key path", fmt.Sprintf("is a directory: %s", keyPath))
	}

	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		ValidationWarning("SSH key permissions",
			fmt.Sprintf("%s has permissions %o (recommended: 0600 or 0400). "+
				"This may be acceptable in CI environments but should be fixed in production.", keyPath, mode))
	}

	return nil
}

// ValidateSafePath ensures path is within baseDir (prevents path traversal attacks).
//
//	if err := ValidateSafePath(templatePath, "test/extended/testdata/two_node/"); err != nil { return err }
func ValidateSafePath(path, baseDir string) error {
	if strings.Contains(path, "..") {
		return ValidationError("path", fmt.Sprintf("contains directory traversal: %s", path))
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return WrapError("resolve path", path, err)
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return WrapError("resolve base directory", baseDir, err)
	}

	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return WrapError("compute relative path", fmt.Sprintf("%s relative to %s", path, baseDir), err)
	}

	if strings.HasPrefix(rel, "..") {
		return ValidationError("path", fmt.Sprintf("%s is outside allowed directory %s", path, baseDir))
	}

	return nil
}

// ValidateIPAddress checks string is valid IPv4/IPv6 (rejects unspecified 0.0.0.0/::).
//
//	if err := ValidateIPAddress(nodeIP); err != nil { return err }
func ValidateIPAddress(ip string) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ValidationError("IP address", fmt.Sprintf("invalid format: %s", ip))
	}

	if parsed.IsUnspecified() {
		return ValidationError("IP address", fmt.Sprintf("unspecified (0.0.0.0/::): %s", ip))
	}

	return nil
}

// ValidateNodeName checks name conforms to Kubernetes/RFC 1123 DNS subdomain rules (max 253 chars).
//
//	if err := ValidateNodeName(nodeName); err != nil { return err }
func ValidateNodeName(name string) error {
	if name == "" {
		return ValidationError("node name", "cannot be empty")
	}

	matched, _ := regexp.MatchString(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`, name)
	if !matched {
		return ValidationError("node name", fmt.Sprintf("invalid format: %s (must be lowercase alphanumeric, hyphens, dots)", name))
	}

	if len(name) > 253 {
		return ValidationError("node name", fmt.Sprintf("too long: %s (max 253 characters)", name))
	}

	return nil
}

// ValidateIntegerBounds checks value is within [min, max] range (prevents overflow/underflow).
//
//	if err := ValidateIntegerBounds(lineCount, 1, 10000, "journal line count"); err != nil { return err }
func ValidateIntegerBounds(value, min, max int, paramName string) error {
	if value < min {
		return ValidationError(paramName, fmt.Sprintf("%d is below minimum %d", value, min))
	}

	if value > max {
		return ValidationError(paramName, fmt.Sprintf("%d exceeds maximum %d", value, max))
	}

	return nil
}

// ExpectNotEmpty validates that a value is not empty with a descriptive error message.
//
//	ExpectNotEmpty(nodeName, "Expected node name to be set")
func ExpectNotEmpty(value interface{}, description string, args ...interface{}) {
	o.Expect(value).ToNot(o.BeEmpty(), fmt.Sprintf(description, args...))
}
