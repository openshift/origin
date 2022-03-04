#!/bin/bash

# Seeing issues w/ buildah log output being intermingled with the container
# output, so adding a sleep in an attempt to let the buildah log output
# stop before the container output starts
sleep 10
unifiedMount=$(awk '{if ($3 == "cgroup2") {print $2; exit}}' /proc/self/mounts)
echo "cgroupv2 mount point is ${unifiedMount}"
unifiedName=$(awk -F: '/^0:/ {if ($1 == "0") {print $3; exit}}' /proc/self/cgroup)
echo "unified cgroup name is ${unifiedName}"
if test -e /"$unifiedMount"/"$unifiedName"/cgroup.controllers ; then
  unifiedControllers=$(cat /"$unifiedMount"/"$unifiedName"/cgroup.controllers)
fi
echo "cgroupv2 controllers are ${unifiedControllers}"

cgroupv1Val=$(cat /sys/fs/cgroup/memory/memory.limit_in_bytes) || true
echo "cgroupv1Val is ${cgroupv1Val}"
if [ "$cgroupv1Val" != "" ]; then
  echo "MEMORY=${cgroupv1Val}"
fi
cgroupv2Val=$(cat /"$unifiedMount"/"$unifiedName"/memory.max) || true
echo "cgroupv2Val is ${cgroupv2Val}"
if [ "$cgroupv2Val" != "" ]; then
  echo "MEMORY=${cgroupv2Val}"
  MEMORY="${cgroupv2Val}"
fi

cgroupv1Val=$(cat /sys/fs/cgroup/memory/memory.memsw.limit_in_bytes) || true
echo "cgroupv1Val is ${cgroupv1Val}"
if [ "$cgroupv1Val" != "" ]; then
  echo "MEMORYSWAP=${cgroupv1Val}"
fi
# ok swap is treated differently between cgroup v1 and v2.  In v1, memory.memsw.limit_in_bytes
# is memory+swap.  In v2, memory.swap.max is just swap.  So with our quota in place, we will
# find a memory.swap.max file with a value of '0' instead of 'max'.
cgroupv2Val=$(cat /"$unifiedMount"/"$unifiedName"/memory.swap.max) || true
echo "cgroupv2Val is ${cgroupv2Val}"
if [ "$cgroupv2Val" != "" ]; then
  # so that our associated ginkgo test case does not have to distinguish between cgroup v1
  # and v2, we calculate the equivalent v1 value
  echo MEMORYSWAP=$(($cgroup2Val + $MEMORY))
fi

if [ -e /sys/fs/cgroup/cpuacct,cpu ]; then
	quota=$(cat /sys/fs/cgroup/cpuacct,cpu/cpu.cfs_quota_us)
	echo QUOTA= && cat /sys/fs/cgroup/cpuacct,cpu/cpu.cfs_quota_us
	echo SHARES= && cat /sys/fs/cgroup/cpuacct,cpu/cpu.shares
	echo PERIOD= && cat /sys/fs/cgroup/cpuacct,cpu/cpu.cfs_period_us
else
	quota=$(cat /sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us)
	echo QUOTA= && cat /sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us
	echo SHARES= && cat /sys/fs/cgroup/cpu,cpuacct/cpu.shares
	echo PERIOD= && cat /sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us
fi

if [ "${quota}" = "-1" ]; then
	cat /proc/self/cgroup
	cat /proc/self/mountinfo
	findmnt
fi
