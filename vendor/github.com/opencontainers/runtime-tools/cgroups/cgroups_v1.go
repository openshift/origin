package cgroups

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
)

// CgroupV1 used for cgroupv1 validation
type CgroupV1 struct {
	MountPath string
}

func getDeviceID(id string) (int64, int64, error) {
	elem := strings.Split(id, ":")
	major, err := strconv.ParseInt(elem[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	minor, err := strconv.ParseInt(elem[1], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

// GetBlockIOData gets cgroup blockio data
func (cg *CgroupV1) GetBlockIOData(pid int, cgPath string) (*rspec.LinuxBlockIO, error) {
	if filepath.IsAbs(cgPath) {
		path := filepath.Join(cg.MountPath, "blkio", cgPath)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsAbsPathRelToMount, fmt.Errorf("In the case of an absolute path, the runtime MUST take the path to be relative to the cgroups mount point"), rspec.Version)
			}
			return nil, err
		}
	}
	lb := &rspec.LinuxBlockIO{}
	names := []string{"weight", "leaf_weight", "weight_device", "leaf_weight_device", "throttle.read_bps_device", "throttle.write_bps_device", "throttle.read_iops_device", "throttle.write_iops_device"}
	for i, name := range names {
		fileName := strings.Join([]string{"blkio", name}, ".")
		filePath := filepath.Join(cg.MountPath, "blkio", cgPath, fileName)
		if !filepath.IsAbs(cgPath) {
			subPath, err := GetSubsystemPath(pid, "blkio")
			if err != nil {
				return nil, err
			}
			if !strings.Contains(subPath, cgPath) {
				return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "blkio")
			}
			filePath = filepath.Join(cg.MountPath, "blkio", subPath, fileName)
		}
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsPathAttach, fmt.Errorf("The runtime MUST consistently attach to the same place in the cgroups hierarchy given the same value of `cgroupsPath`"), rspec.Version)
			}

			return nil, err
		}
		switch i {
		case 0:
			res, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 16)
			if err != nil {
				return nil, err
			}
			weight := uint16(res)
			lb.Weight = &weight
		case 1:
			res, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 16)
			if err != nil {
				return nil, err
			}
			leafWeight := uint16(res)
			lb.LeafWeight = &leafWeight
		case 2:
			parts := strings.Split(strings.TrimSpace(string(contents)), "\n")
			for _, part := range parts {
				elem := strings.Split(part, " ")
				major, minor, err := getDeviceID(elem[0])
				if err != nil {
					return nil, err
				}
				res, err := strconv.ParseUint(elem[1], 10, 16)
				if err != nil {
					return nil, err
				}
				weight := uint16(res)
				lwd := rspec.LinuxWeightDevice{}
				lwd.Major = major
				lwd.Minor = minor
				lwd.Weight = &weight
				lb.WeightDevice = append(lb.WeightDevice, lwd)
			}
		case 3:
			parts := strings.Split(strings.TrimSpace(string(contents)), "\n")
			for _, part := range parts {
				elem := strings.Split(part, " ")
				major, minor, err := getDeviceID(elem[0])
				if err != nil {
					return nil, err
				}
				res, err := strconv.ParseUint(elem[1], 10, 16)
				if err != nil {
					return nil, err
				}
				leafWeight := uint16(res)
				exist := false
				for i, wd := range lb.WeightDevice {
					if wd.Major == major && wd.Minor == minor {
						exist = true
						lb.WeightDevice[i].LeafWeight = &leafWeight
						break
					}
				}
				if !exist {
					lwd := rspec.LinuxWeightDevice{}
					lwd.Major = major
					lwd.Minor = minor
					lwd.LeafWeight = &leafWeight
					lb.WeightDevice = append(lb.WeightDevice, lwd)
				}
			}
		case 4:
			parts := strings.Split(strings.TrimSpace(string(contents)), "\n")
			for _, part := range parts {
				elem := strings.Split(part, " ")
				major, minor, err := getDeviceID(elem[0])
				if err != nil {
					return nil, err
				}
				rate, err := strconv.ParseUint(elem[1], 10, 64)
				if err != nil {
					return nil, err
				}
				ltd := rspec.LinuxThrottleDevice{}
				ltd.Major = major
				ltd.Minor = minor
				ltd.Rate = rate
				lb.ThrottleReadBpsDevice = append(lb.ThrottleReadBpsDevice, ltd)
			}
		case 5:
			parts := strings.Split(strings.TrimSpace(string(contents)), "\n")
			for _, part := range parts {
				elem := strings.Split(part, " ")
				major, minor, err := getDeviceID(elem[0])
				if err != nil {
					return nil, err
				}
				rate, err := strconv.ParseUint(elem[1], 10, 64)
				if err != nil {
					return nil, err
				}
				ltd := rspec.LinuxThrottleDevice{}
				ltd.Major = major
				ltd.Minor = minor
				ltd.Rate = rate
				lb.ThrottleWriteBpsDevice = append(lb.ThrottleWriteBpsDevice, ltd)
			}
		case 6:
			parts := strings.Split(strings.TrimSpace(string(contents)), "\n")
			for _, part := range parts {
				elem := strings.Split(part, " ")
				major, minor, err := getDeviceID(elem[0])
				if err != nil {
					return nil, err
				}
				rate, err := strconv.ParseUint(elem[1], 10, 64)
				if err != nil {
					return nil, err
				}
				ltd := rspec.LinuxThrottleDevice{}
				ltd.Major = major
				ltd.Minor = minor
				ltd.Rate = rate
				lb.ThrottleReadIOPSDevice = append(lb.ThrottleReadIOPSDevice, ltd)
			}
		case 7:
			parts := strings.Split(strings.TrimSpace(string(contents)), "\n")
			for _, part := range parts {
				elem := strings.Split(part, " ")
				major, minor, err := getDeviceID(elem[0])
				if err != nil {
					return nil, err
				}
				rate, err := strconv.ParseUint(elem[1], 10, 64)
				if err != nil {
					return nil, err
				}
				ltd := rspec.LinuxThrottleDevice{}
				ltd.Major = major
				ltd.Minor = minor
				ltd.Rate = rate
				lb.ThrottleWriteIOPSDevice = append(lb.ThrottleWriteIOPSDevice, ltd)
			}
		}
	}

	return lb, nil
}

// GetCPUData gets cgroup cpus data
func (cg *CgroupV1) GetCPUData(pid int, cgPath string) (*rspec.LinuxCPU, error) {
	if filepath.IsAbs(cgPath) {
		path := filepath.Join(cg.MountPath, "cpu", cgPath)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsAbsPathRelToMount, fmt.Errorf("In the case of an absolute path, the runtime MUST take the path to be relative to the cgroups mount point"), rspec.Version)
			}
			return nil, err
		}
	}
	lc := &rspec.LinuxCPU{}
	names := []string{"shares", "cfs_quota_us", "cfs_period_us"}
	for i, name := range names {
		fileName := strings.Join([]string{"cpu", name}, ".")
		filePath := filepath.Join(cg.MountPath, "cpu", cgPath, fileName)
		if !filepath.IsAbs(cgPath) {
			subPath, err := GetSubsystemPath(pid, "cpu")
			if err != nil {
				return nil, err
			}
			if !strings.Contains(subPath, cgPath) {
				return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "cpu")
			}
			filePath = filepath.Join(cg.MountPath, "cpu", subPath, fileName)
		}
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsPathAttach, fmt.Errorf("The runtime MUST consistently attach to the same place in the cgroups hierarchy given the same value of `cgroupsPath`"), rspec.Version)
			}

			return nil, err
		}
		switch i {
		case 0:
			res, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			shares := res
			lc.Shares = &shares
		case 1:
			res, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			quota := res
			lc.Quota = &quota
		case 2:
			res, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			period := res
			lc.Period = &period
		}
	}
	// CONFIG_RT_GROUP_SCHED may be not set
	// Can always get rt data from /proc
	contents, err := ioutil.ReadFile("/proc/sys/kernel/sched_rt_period_us")
	if err != nil {
		return nil, err
	}
	rtPeriod, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil {
		return nil, err
	}
	lc.RealtimePeriod = &rtPeriod
	contents, err = ioutil.ReadFile("/proc/sys/kernel/sched_rt_runtime_us")
	if err != nil {
		return nil, err
	}
	rtQuota, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil {
		return nil, err
	}
	lc.RealtimeRuntime = &rtQuota

	names = []string{"cpus", "mems"}
	for i, name := range names {
		fileName := strings.Join([]string{"cpuset", name}, ".")
		filePath := filepath.Join(cg.MountPath, "cpuset", cgPath, fileName)
		if !filepath.IsAbs(cgPath) {
			subPath, err := GetSubsystemPath(pid, "cpuset")
			if err != nil {
				return nil, err
			}
			if !strings.Contains(subPath, cgPath) {
				return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "cpuset")
			}
			filePath = filepath.Join(cg.MountPath, "cpuset", subPath, fileName)
		}
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		switch i {
		case 0:
			lc.Cpus = strings.TrimSpace(string(contents))
		case 1:
			lc.Mems = strings.TrimSpace(string(contents))
		}
	}

	return lc, nil
}

// GetDevicesData gets cgroup devices data
func (cg *CgroupV1) GetDevicesData(pid int, cgPath string) ([]rspec.LinuxDeviceCgroup, error) {
	ld := []rspec.LinuxDeviceCgroup{}
	fileName := strings.Join([]string{"devices", "list"}, ".")
	filePath := filepath.Join(cg.MountPath, "devices", cgPath, fileName)
	if !filepath.IsAbs(cgPath) {
		subPath, err := GetSubsystemPath(pid, "devices")
		if err != nil {
			return nil, err
		}
		if !strings.Contains(subPath, cgPath) {
			return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "devices")
		}
		filePath = filepath.Join(cg.MountPath, "devices", subPath, fileName)
	}
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimSpace(string(contents)), "\n")
	for _, part := range parts {
		elem := strings.Split(part, " ")
		ele := strings.Split(elem[1], ":")
		var major, minor int64
		if ele[0] == "*" {
			major = 0
		} else {
			major, err = strconv.ParseInt(ele[0], 10, 64)
			if err != nil {
				return nil, err
			}
		}
		if ele[1] == "*" {
			minor = 0
		} else {
			minor, err = strconv.ParseInt(ele[1], 10, 64)
			if err != nil {
				return nil, err
			}
		}

		device := rspec.LinuxDeviceCgroup{}
		device.Allow = true
		device.Type = elem[0]
		device.Major = &major
		device.Minor = &minor
		device.Access = elem[2]
		ld = append(ld, device)
	}

	return ld, nil
}

func inBytes(size string) (int64, error) {
	KiB := 1024
	MiB := 1024 * KiB
	GiB := 1024 * MiB
	TiB := 1024 * GiB
	PiB := 1024 * TiB
	binaryMap := map[string]int64{"k": int64(KiB), "m": int64(MiB), "g": int64(GiB), "t": int64(TiB), "p": int64(PiB)}
	sizeRegex := regexp.MustCompile(`^(\d+(\.\d+)*) ?([kKmMgGtTpP])?[bB]?$`)
	matches := sizeRegex.FindStringSubmatch(size)
	if len(matches) != 4 {
		return -1, fmt.Errorf("invalid size: '%s'", size)
	}

	byteSize, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return -1, err
	}

	unitPrefix := strings.ToLower(matches[3])
	if mul, ok := binaryMap[unitPrefix]; ok {
		byteSize *= float64(mul)
	}

	return int64(byteSize), nil
}

func getHugePageSize() ([]string, error) {
	var pageSizes []string
	sizeList := []string{"B", "kB", "MB", "GB", "TB", "PB"}
	files, err := ioutil.ReadDir("/sys/kernel/mm/hugepages")
	if err != nil {
		return pageSizes, err
	}
	for _, st := range files {
		nameArray := strings.Split(st.Name(), "-")
		pageSize, err := inBytes(nameArray[1])
		if err != nil {
			return []string{}, err
		}
		size := float64(pageSize)
		base := float64(1024.0)
		i := 0
		unitsLimit := len(sizeList) - 1
		for size >= base && i < unitsLimit {
			size = size / base
			i++
		}
		sizeString := fmt.Sprintf("%g%s", size, sizeList[i])
		pageSizes = append(pageSizes, sizeString)
	}

	return pageSizes, nil
}

// GetHugepageLimitData gets cgroup hugetlb data
func (cg *CgroupV1) GetHugepageLimitData(pid int, cgPath string) ([]rspec.LinuxHugepageLimit, error) {
	if filepath.IsAbs(cgPath) {
		path := filepath.Join(cg.MountPath, "hugetlb", cgPath)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsAbsPathRelToMount, fmt.Errorf("In the case of an absolute path, the runtime MUST take the path to be relative to the cgroups mount point"), rspec.Version)
			}
			return nil, err
		}
	}
	lh := []rspec.LinuxHugepageLimit{}
	pageSizes, err := getHugePageSize()
	if err != nil {
		return lh, err
	}
	for _, pageSize := range pageSizes {
		maxUsage := strings.Join([]string{"hugetlb", pageSize, "limit_in_bytes"}, ".")
		filePath := filepath.Join(cg.MountPath, "hugetlb", cgPath, maxUsage)
		if !filepath.IsAbs(cgPath) {
			subPath, err := GetSubsystemPath(pid, "hugetlb")
			if err != nil {
				return lh, err
			}
			if !strings.Contains(subPath, cgPath) {
				return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "hugetlb")
			}
			filePath = filepath.Join(cg.MountPath, "hugetlb", subPath, maxUsage)
		}
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsPathAttach, fmt.Errorf("The runtime MUST consistently attach to the same place in the cgroups hierarchy given the same value of `cgroupsPath`"), rspec.Version)
			}

			return lh, err
		}
		res, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
		if err != nil {
			return nil, err
		}
		pageLimit := rspec.LinuxHugepageLimit{}
		pageLimit.Pagesize = pageSize
		pageLimit.Limit = res
		lh = append(lh, pageLimit)
	}

	return lh, nil
}

// GetMemoryData gets cgroup memory data
func (cg *CgroupV1) GetMemoryData(pid int, cgPath string) (*rspec.LinuxMemory, error) {
	if filepath.IsAbs(cgPath) {
		path := filepath.Join(cg.MountPath, "memory", cgPath)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsAbsPathRelToMount, fmt.Errorf("In the case of an absolute path, the runtime MUST take the path to be relative to the cgroups mount point"), rspec.Version)
			}
			return nil, err
		}
	}
	lm := &rspec.LinuxMemory{}
	names := []string{"limit_in_bytes", "soft_limit_in_bytes", "memsw.limit_in_bytes", "kmem.limit_in_bytes", "kmem.tcp.limit_in_bytes", "swappiness", "oom_control"}
	for i, name := range names {
		fileName := strings.Join([]string{"memory", name}, ".")
		filePath := filepath.Join(cg.MountPath, "memory", cgPath, fileName)
		if !filepath.IsAbs(cgPath) {
			subPath, err := GetSubsystemPath(pid, "memory")
			if err != nil {
				return nil, err
			}
			if !strings.Contains(subPath, cgPath) {
				return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "memory")
			}
			filePath = filepath.Join(cg.MountPath, "memory", subPath, fileName)
		}
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsPathAttach, fmt.Errorf("The runtime MUST consistently attach to the same place in the cgroups hierarchy given the same value of `cgroupsPath`"), rspec.Version)
			}

			return nil, err
		}
		switch i {
		case 0:
			res, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			limit := res
			lm.Limit = &limit
		case 1:
			res, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			sLimit := res
			lm.Reservation = &sLimit
		case 2:
			res, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			swLimit := res
			lm.Swap = &swLimit
		case 3:
			res, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			kernelLimit := res
			lm.Kernel = &kernelLimit
		case 4:
			res, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			tcpLimit := res
			lm.KernelTCP = &tcpLimit
		case 5:
			res, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
			if err != nil {
				return nil, err
			}
			swappiness := res
			lm.Swappiness = &swappiness
		case 6:
			parts := strings.Split(string(contents), "\n")
			part := strings.Split(parts[0], " ")
			res, err := strconv.ParseInt(part[1], 10, 64)
			if err != nil {
				return nil, err
			}
			oom := false
			if res == 1 {
				oom = true
			}
			lm.DisableOOMKiller = &oom
		}
	}

	return lm, nil
}

// GetNetworkData gets cgroup network data
func (cg *CgroupV1) GetNetworkData(pid int, cgPath string) (*rspec.LinuxNetwork, error) {
	if filepath.IsAbs(cgPath) {
		path := filepath.Join(cg.MountPath, "net_cls", cgPath)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsAbsPathRelToMount, fmt.Errorf("In the case of an absolute path, the runtime MUST take the path to be relative to the cgroups mount point"), rspec.Version)
			}
			return nil, err
		}
	}
	ln := &rspec.LinuxNetwork{}
	fileName := strings.Join([]string{"net_cls", "classid"}, ".")
	filePath := filepath.Join(cg.MountPath, "net_cls", cgPath, fileName)
	if !filepath.IsAbs(cgPath) {
		subPath, err := GetSubsystemPath(pid, "net_cls")
		if err != nil {
			return nil, err
		}
		if !strings.Contains(subPath, cgPath) {
			return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "net_cls")
		}
		filePath = filepath.Join(cg.MountPath, "net_cls", subPath, fileName)
	}
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, specerror.NewError(specerror.CgroupsPathAttach, fmt.Errorf("The runtime MUST consistently attach to the same place in the cgroups hierarchy given the same value of `cgroupsPath`"), rspec.Version)
		}

		return nil, err
	}
	res, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil {
		return nil, err
	}
	classid := uint32(res)
	ln.ClassID = &classid

	fileName = strings.Join([]string{"net_prio", "ifpriomap"}, ".")
	filePath = filepath.Join(cg.MountPath, "net_prio", cgPath, fileName)
	if !filepath.IsAbs(cgPath) {
		subPath, err := GetSubsystemPath(pid, "net_prio")
		if err != nil {
			return nil, err
		}
		if !strings.Contains(subPath, cgPath) {
			return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "net_prio")
		}
		filePath = filepath.Join(cg.MountPath, "net_prio", subPath, fileName)
	}
	contents, err = ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimSpace(string(contents)), "\n")
	for _, part := range parts {
		elem := strings.Split(part, " ")
		res, err := strconv.ParseUint(elem[1], 10, 64)
		if err != nil {
			return nil, err
		}
		lip := rspec.LinuxInterfacePriority{}
		lip.Name = elem[0]
		lip.Priority = uint32(res)
		ln.Priorities = append(ln.Priorities, lip)
	}

	return ln, nil
}

// GetPidsData gets cgroup pids data
func (cg *CgroupV1) GetPidsData(pid int, cgPath string) (*rspec.LinuxPids, error) {
	if filepath.IsAbs(cgPath) {
		path := filepath.Join(cg.MountPath, "pids", cgPath)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, specerror.NewError(specerror.CgroupsAbsPathRelToMount, fmt.Errorf("In the case of an absolute path, the runtime MUST take the path to be relative to the cgroups mount point"), rspec.Version)
			}
			return nil, err
		}
	}
	lp := &rspec.LinuxPids{}
	fileName := strings.Join([]string{"pids", "max"}, ".")
	filePath := filepath.Join(cg.MountPath, "pids", cgPath, fileName)
	if !filepath.IsAbs(cgPath) {
		subPath, err := GetSubsystemPath(pid, "pids")
		if err != nil {
			return nil, err
		}
		if !strings.Contains(subPath, cgPath) {
			return nil, fmt.Errorf("cgroup subsystem %s is not mounted as expected", "pids")
		}
		filePath = filepath.Join(cg.MountPath, "pids", subPath, fileName)
	}
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	res, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, specerror.NewError(specerror.CgroupsPathAttach, fmt.Errorf("The runtime MUST consistently attach to the same place in the cgroups hierarchy given the same value of `cgroupsPath`"), rspec.Version)
		}

		return nil, err
	}
	lp.Limit = res

	return lp, nil
}
