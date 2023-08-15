To be renamed to MonitorTests

# What is a MonitorTests

MonitorTests are tests that run while a set of tests (TestSuite) is running and observe the cluster during this time.
Once the TestSuite is finished, the MonitorTests can produce monitorapi.Intervals (these feed the timelines)
and produce junits (succeeded, failed, or skipped).
The junits are represented as TestCases in the job run and influence the pass/fail of the job run.

# Lifecycle of a MonitorTest

1. The TestSuite indicates a stability level to control the set of MonitorTests to be run during the TestSuite.
2. The TestSuite binary then selects the set of MonitorTests and initializes a controller called the Monitor with 
   the list of MonitorTests.
3. Before the TestSuite is started, the Monitor controller is started.
4. When the Monitor is started, for every MonitorTest it calls
   1. StartCollection with an admin kubeconfig and a Recorder.
      A MonitorTest may use the admin kubeconfig to create whatever resources are necessary on the running cluster.
      The same cluster host multiple TestSuites at the same time, be sure that your resources will not conflict and
      that you keep track of them to clean them up later.
      MonitorTests may save the admin kubeconfig for later, may spawn go routines, and may use the Recorder to
      save Intervals throughout the test.
5. The TestCases contained in the TestSuite are now run.  Sometimes in parallel, sometimes not.
6. When the TestSuite is done running the Monitor is stopped.
7. When the Monitor stops, for every MonitorTest, it records its beginning and end times and then calls each of these
   functions for all MonitorTests before calling the next one.  So all MonitorTests complete CollectData before any is
   called with ConstructComputedIntervals.
   1. CollectData with a beginning and end time.
      This function can reach out to the cluster or do whatever else is necessary to return a list of Intervals.
      Examples include doing things like streaming node or pod logs for particular messages or querying prometheus.
      Every Interval at this point is considered a source interval that cannot be recalculated.
   2. ConstructComputedIntervals with all the CollectedData, resources recorded during the TestSuite, and a beginning
      and end time.
      This function can then inspect all the Intervals and create derived Intervals.
      This is commonly done to take things like the instantaneous changes of pod or node state and convert them into
      a set of constructed intervals indicating time spans for particular activities.
   3. EvaluateTestsFromConstructedIntervals with the final set of Intervals and a beginning and end time
      The MonitorTest can process the Intervals and use them to decide whether an individual tests should succeed or fail.
      This is used to do things like know if there was too much disruption, too many pod restarts, etc.
   4. Cleanup
      This is where a MonitorTest must clean up the resources it created on the cluster.
      This is the first potentially multiple cleanup calls, since interrupts will also call cleanup.
      Be sure your invariant tolerates multiple parallel and serial calls to Cleanup.
8. After the Monitor is stopped, it will write data to disk by going to every MonitorTest and calling
   1. WriteContentToStorage with a storage directory.
      At this point MonitorTests may write data that will be preserved by the JobRun.
      Do not write Intervals, RecordedResources, or junit xml files.
      You can do things like write summary or metadat files for the run, like how many requests a particular client made. 

# My MonitorTest doesn't run on X.

This is a problem for each MonitorTest, not a larger piece of logic.
Do not try to add different subsets of MonitorTests that are going to run for different TestSuites.
The only distinction we have is for those TestSuites that take etcd offline and time-travel the cluster by restoring from backup.
Every other situation should be handled by logic inside the individual MonitorTest to know whether it can work or not.
If you cannot determine it with an admin kubeconfig, neither can a customer and this indicates a platform failure that needs correction.