To be expanded later

old Flow
	run upgrade
	before the upgrade happens the primary process would start (but never stop) a bunch of availability check, but NOT SLB and image-registry
	then it would launch another process to run the upgrade itself
	the upgrade (separate process) woudl start the SLB and image-registry a separate process was spawned
	the upgrade process would start terminating
	the SLB and image-registry would cleanup
	that cleanup writes a file to AdditionalEvents__ that contains the disruption events only
	and then the primary process begin to stop
	the primary process reads all AdditionalEvents__
	the primary process analyzes all of it.
		
New flow
	this changes into
	run upgrade is just as twisty as before (so far)
	the primary process starts *all* the availability checks
	the upgrade (separate process) does upgradey things
	we do not write any AdditionalEvents__
	we do not read any additionalEvents__
	the primary process starts shutting down
	the monitor.Stop calls the CollectData, ConstructComputedIntervals, EvaluateTestsFromConstructedIntervals, WriteToStorage, and Cleanup for all invariants
	