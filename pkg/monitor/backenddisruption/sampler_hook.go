package backenddisruption

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// SamplerHook defines some hook functions for the sampler to call. This will give caller an opportunity
// to perform some tasks during different stages of disruption detection.
// DisruptionStarted is called whenever a new disruption is detected by this sampler.
// Other functions can be added as we need.
type SamplerHook interface {
	DisruptionStarted()
}

var tcpdumpLock = &sync.Mutex{}

// We only want one of this
var tcpdumpSamplerHook *TcpdumpSamplerHook

func NewTcpdumpSamplerHook() *TcpdumpSamplerHook {
	tcpdumpLock.Lock()
	defer tcpdumpLock.Unlock()
	if tcpdumpSamplerHook == nil {
		tcpdumpSamplerHook = &TcpdumpSamplerHook{}
	}
	return tcpdumpSamplerHook
}

type TcpdumpSamplerHook struct {
	tcpdumpInstalled bool
	installMutex     sync.Mutex
	tcpdumpRunning   bool
	runningMutex     sync.Mutex
	tcpdumpCancel    context.CancelFunc
	cancelMutex      sync.Mutex
	pcapFilePaths    []string
	pcapMutex        sync.RWMutex
}

func (h *TcpdumpSamplerHook) DisruptionStarted() {
	logger := logrus.WithField("hook", "TcpdumpSamplerHook")
	logger.Info("Disruption detected, checking if tcpdump capture should start")

	// Check if container has required capabilities for tcpdump
	if !h.hasRequiredCapabilities() {
		logger.Info("Container lacks NET_ADMIN and/or NET_RAW capabilities, skipping tcpdump")
		return
	}

	// Check if tcpdump is already running
	h.runningMutex.Lock()
	if h.tcpdumpRunning {
		h.runningMutex.Unlock()
		logger.Info("Tcpdump already running, skipping new capture")
		return
	}
	h.tcpdumpRunning = true
	h.runningMutex.Unlock()

	// Set up cleanup in case of early exit
	tcpdumpStarted := false
	defer func() {
		if !tcpdumpStarted {
			// If we're exiting before starting the goroutine, reset the flag
			h.runningMutex.Lock()
			h.tcpdumpRunning = false
			h.runningMutex.Unlock()

			// Also clear the cancel function and remove the last pcap file path if tcpdump failed to start
			h.cancelMutex.Lock()
			h.tcpdumpCancel = nil
			h.cancelMutex.Unlock()

			h.pcapMutex.Lock()
			if len(h.pcapFilePaths) > 0 {
				// Remove the last added path since tcpdump failed to start
				h.pcapFilePaths = h.pcapFilePaths[:len(h.pcapFilePaths)-1]
			}
			h.pcapMutex.Unlock()
		}
	}()

	// Ensure tcpdump is installed
	if err := h.ensureTcpdumpInstalled(); err != nil {
		logger.WithError(err).Error("Failed to install tcpdump")
		return
	}

	// Create log directory if it doesn't exist
	logDir := "/var/log/tcpdump"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		logger.WithError(err).Errorf("Failed to create tcpdump log directory: %s", logDir)
		return
	}

	// Generate timestamp for pcap filename with microseconds for uniqueness
	timestamp := time.Now().Format("2006-01-02T150405.000000")
	pcapFile := fmt.Sprintf("%s/tcpdump-%s.pcap", logDir, timestamp)

	// Add pcap file path to the list for later use
	h.pcapMutex.Lock()
	h.pcapFilePaths = append(h.pcapFilePaths, pcapFile)
	h.pcapMutex.Unlock()

	// Create context with 30-minute timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)

	// Store cancel function for external stopping
	h.cancelMutex.Lock()
	h.tcpdumpCancel = cancel
	h.cancelMutex.Unlock()

	defer cancel()

	// Build tcpdump command
	tcpdumpCmd := []string{
		"/usr/sbin/tcpdump",
		"-nn",       // Don't resolve hostnames or port names
		"-U",        // Write packets immediately
		"-i", "any", // Capture on all interfaces
		"-s", "256", // Capture first 256 bytes of each packet
		"-w", pcapFile, // Write to file
		"tcp", "and", "port", "80", // Filter for HTTP traffic
	}

	logger.WithField("command", tcpdumpCmd).WithField("pcap_file", pcapFile).WithField("duration", "30m").Info("Starting tcpdump with timeout")

	// Start tcpdump with context timeout
	cmd := exec.CommandContext(ctx, tcpdumpCmd[0], tcpdumpCmd[1:]...)
	if err := cmd.Start(); err != nil {
		logger.WithError(err).Error("Failed to start tcpdump")
		return
	}

	// Mark that tcpdump was started successfully
	tcpdumpStarted = true

	logger.WithField("pid", cmd.Process.Pid).WithField("pcap_file", pcapFile).Info("Tcpdump started successfully")

	// Run tcpdump in a goroutine to handle the 30-minute timeout
	go func() {
		defer func() {
			// Ensure we reset the running flag when the process completes
			h.runningMutex.Lock()
			h.tcpdumpRunning = false
			h.runningMutex.Unlock()

			// Clear the cancel function
			h.cancelMutex.Lock()
			h.tcpdumpCancel = nil
			h.cancelMutex.Unlock()

			// Note: Don't clear pcapFilePaths here - they're needed for MoveToStorage
		}()

		// Wait for the command to complete or timeout
		if err := cmd.Wait(); err != nil {
			// Check if it was killed due to timeout
			if ctx.Err() == context.DeadlineExceeded {
				logger.WithField("pcap_file", pcapFile).Info("Tcpdump completed after 30-minute timeout")
			} else {
				logger.WithError(err).WithField("pcap_file", pcapFile).Error("Tcpdump exited with error")
			}
		} else {
			logger.WithField("pcap_file", pcapFile).Info("Tcpdump completed successfully")
		}
	}()
}

func (h *TcpdumpSamplerHook) ensureTcpdumpInstalled() error {
	h.installMutex.Lock()
	defer h.installMutex.Unlock()

	// Check if already installed
	if h.tcpdumpInstalled {
		return nil
	}

	logger := logrus.WithField("function", "ensureTcpdumpInstalled")

	// Check if tcpdump is already available
	if _, err := exec.LookPath("tcpdump"); err == nil {
		logger.Info("tcpdump already available in PATH")
		h.tcpdumpInstalled = true
		return nil
	}

	// Check if tcpdump exists at expected location
	if _, err := os.Stat("/usr/sbin/tcpdump"); err == nil {
		logger.Info("tcpdump already installed at /usr/sbin/tcpdump")
		h.tcpdumpInstalled = true
		return nil
	}

	logger.Info("Installing tcpdump from CentOS 8 RPM")

	// Download and install tcpdump RPM
	rpmURL := "http://mirror.centos.org/centos/8/AppStream/x86_64/os/Packages/tcpdump-4.9.3-2.el8.x86_64.rpm"

	// Use rpm command to install from URL
	installCmd := exec.Command("rpm", "-ivh", rpmURL)
	if output, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install tcpdump RPM: %v, output: %s", err, output)
	}

	logger.Info("Successfully installed tcpdump")

	// Verify installation
	if _, err := os.Stat("/usr/sbin/tcpdump"); err != nil {
		return fmt.Errorf("tcpdump not found at /usr/sbin/tcpdump after installation: %v", err)
	}

	h.tcpdumpInstalled = true
	return nil
}

// hasRequiredCapabilities checks if the container has NET_ADMIN and NET_RAW capabilities
// by reading /proc/self/status
func (h *TcpdumpSamplerHook) hasRequiredCapabilities() bool {
	logger := logrus.WithField("function", "hasRequiredCapabilities")

	// Read /proc/self/status to check effective capabilities
	file, err := os.Open("/proc/self/status")
	if err != nil {
		logger.WithError(err).Warn("Failed to open /proc/self/status, assuming capabilities are NOT present")
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "CapEff:") {
			// Extract the hex capability value
			parts := strings.Fields(line)
			if len(parts) != 2 {
				logger.Warn("Unexpected format in CapEff line")
				return false
			}

			// Parse the hex value
			capHex := parts[1]
			capValue, err := strconv.ParseUint(capHex, 16, 64)
			if err != nil {
				logger.WithError(err).WithField("cap_hex", capHex).Warn("Failed to parse capabilities hex value")
				return false
			}

			// Check for NET_ADMIN (12) and NET_RAW (13) capabilities
			// Capabilities are bit flags, so we check if the corresponding bits are set
			netAdminBit := uint64(1) << 12 // NET_ADMIN
			netRawBit := uint64(1) << 13   // NET_RAW

			hasNetAdmin := (capValue & netAdminBit) != 0
			hasNetRaw := (capValue & netRawBit) != 0

			logger.WithFields(logrus.Fields{
				"cap_value":     fmt.Sprintf("0x%x", capValue),
				"has_net_admin": hasNetAdmin,
				"has_net_raw":   hasNetRaw,
			}).Debug("Capability check results from /proc/self/status")

			if hasNetAdmin && hasNetRaw {
				logger.Info("Container has required NET_ADMIN and NET_RAW capabilities")
				return true
			}

			missingCaps := []string{}
			if !hasNetAdmin {
				missingCaps = append(missingCaps, "NET_ADMIN")
			}
			if !hasNetRaw {
				missingCaps = append(missingCaps, "NET_RAW")
			}

			logger.WithField("missing_capabilities", missingCaps).Warn("Container missing required capabilities for tcpdump")
			return false
		}
	}

	if err := scanner.Err(); err != nil {
		logger.WithError(err).Warn("Error reading /proc/self/status")
	}

	logger.Warn("CapEff line not found in /proc/self/status")
	return false
}

// StopCollection stops any running tcpdump process. This should be called by monitor tests
// in their CollectData function to ensure proper cleanup when the test is terminating.
func (h *TcpdumpSamplerHook) StopCollection() {
	logger := logrus.WithField("hook", "TcpdumpSamplerHook")
	logger.Info("Stopping tcpdump collection")

	h.cancelMutex.Lock()
	defer h.cancelMutex.Unlock()

	if h.tcpdumpCancel != nil {
		logger.Info("Cancelling running tcpdump process")
		h.tcpdumpCancel()
		h.tcpdumpCancel = nil
	} else {
		logger.Debug("No running tcpdump process to stop")
	}
}

// MoveToStorage moves all captured pcap files to the specified storage directory under
// a "tcpdump" subfolder. This function is idempotent and thread-safe - it can be called
// multiple times safely. This should be called by monitor tests in their WriteContentToStorage
// function to archive the network capture data.
func (h *TcpdumpSamplerHook) MoveToStorage(storageDir string) error {
	logger := logrus.WithField("hook", "TcpdumpSamplerHook")

	// Use write lock for the entire operation to prevent race conditions
	h.pcapMutex.Lock()
	defer h.pcapMutex.Unlock()

	if len(h.pcapFilePaths) == 0 {
		logger.Debug("No pcap files to move - tcpdump may not have been started or files already moved")
		return nil
	}

	// Create tcpdump subfolder in storage directory
	tcpdumpDir := fmt.Sprintf("%s/tcpdump", storageDir)
	if err := os.MkdirAll(tcpdumpDir, 0755); err != nil {
		return fmt.Errorf("failed to create tcpdump storage directory %s: %w", tcpdumpDir, err)
	}

	var errors []string
	movedCount := 0

	// Move all pcap files
	for _, pcapFile := range h.pcapFilePaths {
		// Extract filename from full path
		_, filename := filepath.Split(pcapFile)
		destFile := fmt.Sprintf("%s/%s", tcpdumpDir, filename)

		// Move the file using rename only
		if err := os.Rename(pcapFile, destFile); err != nil {
			errorMsg := fmt.Sprintf("failed to move pcap file from %s to %s: %v", pcapFile, destFile, err)
			errors = append(errors, errorMsg)
			logger.WithError(err).WithFields(logrus.Fields{
				"source_file": pcapFile,
				"dest_file":   destFile,
			}).Warn("Failed to move pcap file")
			continue
		}

		logger.WithFields(logrus.Fields{
			"source_file": pcapFile,
			"dest_file":   destFile,
		}).Debug("Successfully moved pcap file to storage")
		movedCount++
	}

	// Clear all file paths - this makes subsequent calls idempotent
	h.pcapFilePaths = nil

	logger.WithFields(logrus.Fields{
		"moved_files":  movedCount,
		"failed_files": len(errors),
	}).Info("Completed moving pcap files to storage")

	// Return error if any files failed to move
	if len(errors) > 0 {
		return fmt.Errorf("failed to move %d pcap files: %v", len(errors), errors)
	}

	return nil
}
