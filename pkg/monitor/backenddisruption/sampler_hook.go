package backenddisruption

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	logFilePaths     []string
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
				// Remove the last added paths since tcpdump failed to start
				h.pcapFilePaths = h.pcapFilePaths[:len(h.pcapFilePaths)-1]
			}
			if len(h.logFilePaths) > 0 {
				// Remove the last added log file path since tcpdump failed to start
				h.logFilePaths = h.logFilePaths[:len(h.logFilePaths)-1]
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
	logFile := fmt.Sprintf("%s/tcpdump-%s.log", logDir, timestamp)

	// Add file paths to the lists for later use
	h.pcapMutex.Lock()
	h.pcapFilePaths = append(h.pcapFilePaths, pcapFile)
	h.logFilePaths = append(h.logFilePaths, logFile)
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

	logger.WithField("command", tcpdumpCmd).WithField("pcap_file", pcapFile).WithField("log_file", logFile).WithField("duration", "30m").Info("Starting tcpdump with timeout")

	// Create log file for tcpdump stdout/stderr
	logFileHandle, err := os.Create(logFile)
	if err != nil {
		logger.WithError(err).Error("Failed to create tcpdump log file")
		return
	}
	defer logFileHandle.Close()

	// Start tcpdump with context timeout
	cmd := exec.CommandContext(ctx, tcpdumpCmd[0], tcpdumpCmd[1:]...)

	// Redirect stdout and stderr to the log file
	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle

	if err := cmd.Start(); err != nil {
		logger.WithError(err).Error("Failed to start tcpdump")
		return
	}

	// Mark that tcpdump was started successfully
	tcpdumpStarted = true

	pid := cmd.Process.Pid
	logger.WithFields(logrus.Fields{
		"pid":       pid,
		"pcap_file": pcapFile,
		"log_file":  logFile,
	}).Info("Tcpdump started successfully")

	// Log initial system resource state for baseline
	go func() {
		// Give the process a moment to start up
		time.Sleep(1 * time.Second)

		baselineLogger := logger.WithField("phase", "startup_baseline")

		// Log initial memory state
		if meminfo, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(meminfo), "\n")
			for _, line := range lines {
				if strings.Contains(line, "MemAvailable:") || strings.Contains(line, "MemFree:") {
					baselineLogger.WithField("memory_baseline", strings.TrimSpace(line)).Info("System memory at tcpdump startup")
					break
				}
			}
		}

		// Log initial process memory if available
		if status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
			statusLines := strings.Split(string(status), "\n")
			for _, line := range statusLines {
				if strings.HasPrefix(line, "VmRSS:") {
					baselineLogger.WithField("process_memory_baseline", strings.TrimSpace(line)).Info("Process memory at startup")
					break
				}
			}
		}
	}()

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
				// Collect detailed process exit information
				exitCode, signal, details := getProcessExitInfo(err)

				errorLogger := logger.WithError(err).WithFields(logrus.Fields{
					"pcap_file":    pcapFile,
					"log_file":     logFile,
					"exit_code":    exitCode,
					"signal":       signal,
					"exit_details": details,
				})

				errorLogger.Error("Tcpdump exited with error")

				// If process was killed by signal (especially SIGKILL), collect system diagnostics
				if signal != "" {
					errorLogger.Warn("Process was terminated by signal - collecting system diagnostics")
					collectSystemDiagnostics(errorLogger, cmd.Process.Pid)
				}
			}
		} else {
			logger.WithFields(logrus.Fields{
				"pcap_file": pcapFile,
				"log_file":  logFile,
			}).Info("Tcpdump completed successfully")
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
			logrus.Infof("CapEff: %s", line)
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
			logger.Infof("capValue: %d", capValue)

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

// collectSystemDiagnostics gathers system information that might explain why tcpdump was killed
func collectSystemDiagnostics(logger *logrus.Entry, pid int) {
	// Collect memory information
	if meminfo, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(meminfo), "\n")
		memFields := make(map[string]string)
		for _, line := range lines {
			if strings.Contains(line, "MemTotal:") || strings.Contains(line, "MemAvailable:") ||
				strings.Contains(line, "MemFree:") || strings.Contains(line, "Buffers:") ||
				strings.Contains(line, "Cached:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					memFields[parts[0]] = parts[1]
				}
			}
		}
		logrus.WithField("memory_info", memFields).WithField("pid", pid).Info("System memory information at tcpdump termination")
	}

	// Check for OOM killer activity in dmesg (recent entries)
	if dmesgCmd := exec.Command("dmesg", "-T", "--since", "5 minutes ago"); dmesgCmd != nil {
		if dmesgOutput, err := dmesgCmd.Output(); err == nil {
			dmesgStr := string(dmesgOutput)
			if strings.Contains(dmesgStr, "Out of memory") || strings.Contains(dmesgStr, "oom-killer") ||
				strings.Contains(dmesgStr, "Killed process") {
				logger.WithField("oom_activity", dmesgStr).Warn("OOM killer activity detected around tcpdump termination")
			} else {
				logger.Info("dmesgStr: %s", dmesgStr)
			}
		} else {
			logger.Infof("error reading dmesg output: %s", err)
		}
	} else {
		logrus.Warn("Failed to collect system diagnostics")
	}

	// Check process resource limits
	if pid > 0 {
		limitsPath := fmt.Sprintf("/proc/%d/limits", pid)
		if limits, err := os.ReadFile(limitsPath); err == nil {
			logger.WithField("process_limits", string(limits)).Info("Process resource limits")
		} else {
			logger.WithError(err).Warn("Failed to read /proc/limits")
		}

		// Check process status if still available
		statusPath := fmt.Sprintf("/proc/%d/status", pid)
		if status, err := os.ReadFile(statusPath); err == nil {
			logger.WithField("process_status", string(status)).Info("Process status file")
			// Parse relevant fields from status
			statusLines := strings.Split(string(status), "\n")
			for _, line := range statusLines {
				if strings.HasPrefix(line, "VmPeak:") || strings.HasPrefix(line, "VmSize:") ||
					strings.HasPrefix(line, "VmRSS:") || strings.HasPrefix(line, "VmData:") {
					logger.WithField("process_memory", line).Info("Process memory usage")
				}
			}
		} else {
			logrus.WithError(err).Warn("Failed to read /proc/status")
		}
	}

	// Check disk space
	if _, err := os.Stat("/var/log/tcpdump"); err == nil {
		var statfs syscall.Statfs_t
		if syscall.Statfs("/var/log/tcpdump", &statfs) == nil {
			available := statfs.Bavail * uint64(statfs.Bsize)
			total := statfs.Blocks * uint64(statfs.Bsize)
			logger.WithFields(logrus.Fields{
				"disk_available_bytes": available,
				"disk_total_bytes":     total,
				"disk_available_mb":    available / (1024 * 1024),
			}).Info("Disk space information for tcpdump directory")
		}
	}

	// Collect recent system log entries from system logs (/var/log/messages, /var/log/kern.log, etc.)
	collectSystemLogEntries(logger, pid)
}

// collectSystemLogEntries collects recent entries from system logs that might explain process termination
func collectSystemLogEntries(logger *logrus.Entry, pid int) {
	logrus.WithField("pid", pid).Info("Collecting system log entries")
	// Try multiple common system log paths - process all available logs for comprehensive coverage
	logPaths := []string{
		"/var/log/messages",
		"/var/log/syslog",
		"/var/log/kern.log",   // kernel messages - very important for process kills
		"/var/log/system.log", // macOS
	}

	// Parse log entries and filter for recent ones (last 10 minutes)
	cutoffTime := time.Now().Add(-10 * time.Minute)
	allRelevantEntries := []string{}
	allSuspiciousEntries := []string{}
	processedLogs := []string{}

	// Process all available log files
	for _, path := range logPaths {
		if _, statErr := os.Stat(path); statErr == nil {
			if logContent, err := os.ReadFile(path); err == nil {
				processedLogs = append(processedLogs, path)
				logger.WithField("log_path", path).Info("Analyzing system log entries")

				// Process this log file
				relevantEntries, suspiciousEntries := parseLogEntries(logContent, cutoffTime, pid)
				allRelevantEntries = append(allRelevantEntries, relevantEntries...)

				// Add log source info to suspicious entries
				for _, entry := range suspiciousEntries {
					sourcedEntry := fmt.Sprintf("[%s] %s", filepath.Base(path), entry)
					allSuspiciousEntries = append(allSuspiciousEntries, sourcedEntry)
				}
			} else {
				logrus.WithError(err).WithField("log_path", path).Info("Failed to read system log file")
			}
		} else {
			logrus.WithError(statErr).WithField("file", path).Info("Failed to stat system log file")
		}
	}

	// Log findings from all processed log files
	if len(allSuspiciousEntries) > 0 {
		logger.WithFields(logrus.Fields{
			"processed_logs":     processedLogs,
			"suspicious_entries": allSuspiciousEntries,
			"total_recent":       len(allRelevantEntries),
		}).Warn("Found suspicious system log entries around tcpdump termination")
	} else if len(allRelevantEntries) > 0 {
		// Log a few recent entries for context, but limit to avoid log spam
		entriesToLog := allRelevantEntries
		if len(entriesToLog) > 20 {
			entriesToLog = allRelevantEntries[len(allRelevantEntries)-20:] // Last 20 entries
		}
		logger.WithFields(logrus.Fields{
			"processed_logs": processedLogs,
			"recent_entries": entriesToLog,
			"total_recent":   len(allRelevantEntries),
		}).Info("Recent system log entries (no obvious issues detected)")
	} else {
		logrus.WithFields(logrus.Fields{
			"processed_logs": processedLogs,
		}).Info("No recent entries found in system logs")
	}
}

// parseLogEntries processes log content and returns relevant and suspicious entries
func parseLogEntries(logContent []byte, cutoffTime time.Time, pid int) (relevantEntries, suspiciousEntries []string) {
	lines := strings.Split(string(logContent), "\n")
	pidStr := fmt.Sprintf("%d", pid)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check if this line is recent enough (this is a basic heuristic)
		// Most system logs include timestamps, but formats vary
		if isRecentLogEntry(line, cutoffTime) {
			relevantEntries = append(relevantEntries, line)

			// Look for suspicious patterns that might indicate why tcpdump was killed
			lowerLine := strings.ToLower(line)

			if strings.Contains(lowerLine, "oom") ||
				strings.Contains(lowerLine, "out of memory") ||
				strings.Contains(lowerLine, "memory") ||
				strings.Contains(lowerLine, "killed") ||
				strings.Contains(lowerLine, "kill") ||
				strings.Contains(lowerLine, "signal") ||
				strings.Contains(lowerLine, "terminated") ||
				strings.Contains(lowerLine, "limit") ||
				strings.Contains(lowerLine, "quota") ||
				strings.Contains(lowerLine, "security") ||
				strings.Contains(lowerLine, "selinux") ||
				strings.Contains(lowerLine, "apparmor") ||
				strings.Contains(lowerLine, "tcpdump") ||
				strings.Contains(line, pidStr) ||
				// Kernel-specific patterns (especially from kern.log)
				strings.Contains(lowerLine, "kernel:") ||
				strings.Contains(lowerLine, "segfault") ||
				strings.Contains(lowerLine, "general protection") ||
				strings.Contains(lowerLine, "invalid opcode") ||
				strings.Contains(lowerLine, "cgroup") ||
				strings.Contains(lowerLine, "namespace") ||
				strings.Contains(lowerLine, "capability") ||
				strings.Contains(lowerLine, "audit") ||
				strings.Contains(lowerLine, "avc:") || // SELinux denials
				strings.Contains(lowerLine, "denied") ||
				strings.Contains(lowerLine, "permission") ||
				strings.Contains(lowerLine, "resource") ||
				strings.Contains(lowerLine, "rlimit") ||
				strings.Contains(lowerLine, "netfilter") ||
				strings.Contains(lowerLine, "iptables") ||
				strings.Contains(lowerLine, "network") ||
				strings.Contains(lowerLine, "interface") ||
				strings.Contains(lowerLine, "socket") ||
				strings.Contains(lowerLine, "packet") ||
				strings.Contains(lowerLine, "buffer") ||
				strings.Contains(lowerLine, "allocation failed") ||
				strings.Contains(lowerLine, "cannot allocate") ||
				strings.Contains(lowerLine, "no space left") ||
				strings.Contains(lowerLine, "disk full") {
				suspiciousEntries = append(suspiciousEntries, line)
			}
		}
	}

	return relevantEntries, suspiciousEntries
}

// isRecentLogEntry checks if a log entry is from within the specified cutoff time
func isRecentLogEntry(line string, cutoffTime time.Time) bool {
	// This is a heuristic approach since log formats vary significantly
	// We'll look for common timestamp patterns and try to parse them

	now := time.Now()

	// Common syslog format: "Dec 15 14:30:22" (assuming current year)
	if matches := regexp.MustCompile(`^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})`).FindStringSubmatch(line); len(matches) > 1 {
		timeStr := matches[1]
		if t, err := time.Parse("Jan 2 15:04:05", timeStr); err == nil {
			// Assume current year
			t = t.AddDate(now.Year(), 0, 0)
			// Handle year boundary (if parsed time is in future, assume it's from last year)
			if t.After(now.Add(24 * time.Hour)) {
				t = t.AddDate(-1, 0, 0)
			}
			return t.After(cutoffTime)
		}
	}

	// ISO 8601 format: "2024-01-15T14:30:22"
	if matches := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})`).FindStringSubmatch(line); len(matches) > 1 {
		if t, err := time.Parse("2006-01-02T15:04:05", matches[1]); err == nil {
			return t.After(cutoffTime)
		}
	}

	// RFC3339 format: "2024-01-15 14:30:22"
	if matches := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`).FindStringSubmatch(line); len(matches) > 1 {
		if t, err := time.Parse("2006-01-02 15:04:05", matches[1]); err == nil {
			return t.After(cutoffTime)
		}
	}

	// If we can't parse the timestamp, assume it's recent to be safe
	// This might include some older entries, but it's better than missing important ones
	return true
}

// getProcessExitInfo extracts detailed information about process termination
func getProcessExitInfo(err error) (exitCode int, signal string, details string) {
	if exitError, ok := err.(*exec.ExitError); ok {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			if status.Exited() {
				exitCode = status.ExitStatus()
				details = fmt.Sprintf("process exited with code %d", exitCode)
			} else if status.Signaled() {
				sig := status.Signal()
				signal = sig.String()
				details = fmt.Sprintf("process killed by signal %s (%d)", signal, sig)

				// Add common signal explanations
				switch sig {
				case syscall.SIGKILL:
					details += " - likely killed by OOM killer or system resource manager"
				case syscall.SIGTERM:
					details += " - terminated gracefully by system or container runtime"
				case syscall.SIGINT:
					details += " - interrupted (Ctrl+C or similar)"
				case syscall.SIGHUP:
					details += " - hangup, possibly terminal/connection closed"
				}
			} else if status.Stopped() {
				sig := status.StopSignal()
				signal = sig.String()
				details = fmt.Sprintf("process stopped by signal %s (%d)", signal, sig)
			}
		}
	}

	if details == "" {
		details = fmt.Sprintf("unknown error: %v", err)
	}

	return exitCode, signal, details
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Sync to ensure data is written to disk
	return destFile.Sync()
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

// MoveToStorage moves all captured pcap files and log files to the specified storage directory under
// a "tcpdump" subfolder. This function is idempotent and thread-safe - it can be called
// multiple times safely. This should be called by monitor tests in their WriteContentToStorage
// function to archive the network capture data.
func (h *TcpdumpSamplerHook) MoveToStorage(storageDir string) error {
	logger := logrus.WithField("hook", "TcpdumpSamplerHook")

	// Use write lock for the entire operation to prevent race conditions
	h.pcapMutex.Lock()
	defer h.pcapMutex.Unlock()

	if len(h.pcapFilePaths) == 0 && len(h.logFilePaths) == 0 {
		logger.Debug("No files to move - tcpdump may not have been started or files already moved")
		return nil
	}

	// Create tcpdump subfolder in storage directory
	tcpdumpDir := fmt.Sprintf("%s/tcpdump", storageDir)
	if err := os.MkdirAll(tcpdumpDir, 0755); err != nil {
		return fmt.Errorf("failed to create tcpdump storage directory %s: %w", tcpdumpDir, err)
	}

	var errors []string
	movedCount := 0

	// Helper function to move files
	moveFiles := func(filePaths []string, fileType string) {
		for _, sourceFile := range filePaths {
			// Extract filename from full path
			_, filename := filepath.Split(sourceFile)
			destFile := fmt.Sprintf("%s/%s", tcpdumpDir, filename)

			// Try to move the file using rename first (most efficient)
			if err := os.Rename(sourceFile, destFile); err != nil {
				logger.WithError(err).WithFields(logrus.Fields{
					"source_file": sourceFile,
					"dest_file":   destFile,
					"file_type":   fileType,
				}).Debug("Rename failed, attempting copy+delete fallback")

				// Fallback to copy+delete if rename fails (e.g., cross-filesystem move)
				if copyErr := copyFile(sourceFile, destFile); copyErr != nil {
					errorMsg := fmt.Sprintf("failed to copy %s file from %s to %s: %v", fileType, sourceFile, destFile, copyErr)
					errors = append(errors, errorMsg)
					logger.WithError(copyErr).WithFields(logrus.Fields{
						"source_file": sourceFile,
						"dest_file":   destFile,
						"file_type":   fileType,
					}).Warn("Failed to copy file")
					continue
				}

				// Copy succeeded, now delete the original
				if deleteErr := os.Remove(sourceFile); deleteErr != nil {
					logger.WithError(deleteErr).WithFields(logrus.Fields{
						"source_file": sourceFile,
						"file_type":   fileType,
					}).Warn("Failed to delete original file after copy")
					// Don't fail the entire operation for delete failures - the file was successfully copied
				}

				logger.WithFields(logrus.Fields{
					"source_file": sourceFile,
					"dest_file":   destFile,
					"file_type":   fileType,
				}).Debug("Successfully moved file using copy+delete fallback")
			} else {
				logger.WithFields(logrus.Fields{
					"source_file": sourceFile,
					"dest_file":   destFile,
					"file_type":   fileType,
				}).Debug("Successfully moved file using rename")
			}

			movedCount++
		}
	}

	// Move all pcap files
	moveFiles(h.pcapFilePaths, "pcap")

	// Move all log files
	moveFiles(h.logFilePaths, "log")

	// Clear all file paths - this makes subsequent calls idempotent
	h.pcapFilePaths = nil
	h.logFilePaths = nil

	logger.WithFields(logrus.Fields{
		"moved_files":  movedCount,
		"failed_files": len(errors),
	}).Info("Completed moving tcpdump files (pcap and logs) to storage")

	// Return error if any files failed to move
	if len(errors) > 0 {
		return fmt.Errorf("failed to move %d tcpdump files: %v", len(errors), errors)
	}

	return nil
}
