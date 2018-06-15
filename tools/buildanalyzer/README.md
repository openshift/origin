# Build Analyzer

A tool for analyzing the failure causes of builds in a cluster.

## How to build

    $ go build .

## Basic Usage

This tool takes a json file input containing an ObjectList of builds, such as retrieved via:

    $ oc get builds --all-namespaces -o json > builds.json

The file is passed with the `-f` argument:

    $ buildanalyzer -f builds.json

By default it will dump some analysis about the number of builds in each state, including success and failure.  It will
also dump some information about the different reasons for failure and the number of builds that failed with each reason.

The tool will filter the list of builds to determine which builds are "interesting" and only perform analysis on those builds.
See the flags section for details on how the "interesting" builds are determined.

## Optional Flags

* `--trigger-time` accepts a time formatted such as 2017-01-02T15:04:05+00:00.  This time is used to divide builds into "pre-upgrade" and "post-upgrade".  Builds before this time are used to check if a particular BuildConfig ever succeeded previously.  Builds after this time are inspected for failure.  Builds which failed after this time and succeeded at least once before this time, are treated as "interesting" and subject to analysis by the tool.  If no time is provided, all builds are considered "interesting" (subject to other flag settings).

* `--image-change-only` will only consider builds "interesting" if they were triggered by an imagechange.  The intent is to limit the analysis to builds kicked off by an imagestream update/import as might occur during a cluster upgrade.  Builds prior to the trigger-time (if any) are still used to determine if the BuildConfig ever succeeded, regardless of their trigger cause.  Defaults to true.

* `--start-time` accepts a time formatted such as 2017-01-02T15:04:05+00:00.  Builds before this time will be ignored/filtered, including for purposes of determining if a particular BuildConfig ever succeeded.

* `--push-times` will dump how long each "interesting" and successful build took to push.  Defaults to false.

* `--test-clone` will attempt to git clone the git source url for any "interesting" build which failed while fetching source.  Builds for which the clone is successful will be reported (as this indicates the fetch source failure was not due to an inaccessible repository, so the failure should be investigated more thoroughly).  Defaults to false.
