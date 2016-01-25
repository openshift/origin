FROM centos:7
USER root

RUN echo -n CGROUP= && cat /proc/self/cgroup
RUN echo -n MEMORY= && cat /sys/fs/cgroup/memory/memory.limit_in_bytes
RUN echo -n MEMORYSWAP= && cat /sys/fs/cgroup/memory/memory.memsw.limit_in_bytes
RUN echo -n QUOTA= && cat /sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us
RUN echo -n SHARES= && cat /sys/fs/cgroup/cpu,cpuacct/cpu.shares
RUN echo -n PERIOD= && cat /sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us

# exit 1 causes the docker build to fail which causes docker to show the output # of all commands like 'cat'.
RUN exit 1

