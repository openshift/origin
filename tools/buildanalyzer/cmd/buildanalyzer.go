package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildinternalapi "github.com/openshift/origin/pkg/build/apis/build"
	buildapi "github.com/openshift/origin/pkg/build/apis/build/v1"
)

type BuildAnalyzerOptions struct {
	// time of upgrade/triggering of new builds we care about
	TriggerTime *metav1.Time

	// ignore builds older than this
	StartTime *metav1.Time

	// whether to attempt to clone the build's git source
	TestClone bool

	// source file for the build objects
	BuildFile string

	// only analyze builds triggered by an imagechange
	ImageChangeOnly bool

	// dump push time data for each interesting build.
	PushTimes bool
}

func NewBuildAnalyzerCommand() *cobra.Command {
	o := &BuildAnalyzerOptions{}

	triggerTime := ""
	startTime := ""
	cmd := &cobra.Command{
		Use:   "Analyze an ObjectList of builds",
		Short: "Analyze builds",
		Run: func(cmd *cobra.Command, args []string) {
			if len(triggerTime) > 0 {
				t, e := time.Parse(time.RFC3339, triggerTime)
				if e != nil {
					fmt.Printf("unparseable interest-time: %v\n", e)
					return
				}
				t2 := metav1.NewTime(t)
				o.TriggerTime = &t2
			}
			if len(startTime) > 0 {
				t, e := time.Parse(time.RFC3339, startTime)
				if e != nil {
					fmt.Printf("unparseable start-time: %v\n", e)
					return
				}
				t2 := metav1.NewTime(t)
				o.StartTime = &t2
			}
			if e := o.Run(); e != nil {
				fmt.Printf("error analyzing builds: %v\n", e)
			}
		},
	}

	cmd.Flags().StringVar(&triggerTime, "trigger-time", "", "builds of interest completed after this time (format: 2007-01-02T15:04:05+00:00)")
	cmd.Flags().StringVar(&startTime, "start-time", "", "ignore builds completed before this time (format: 2007-01-02T15:04:05+00:00)")
	cmd.Flags().BoolVar(&o.TestClone, "test-clone", false, "if true, test cloning the build's repo if it failed due to a fetch source issue")
	cmd.Flags().StringVarP(&o.BuildFile, "file", "f", "builds.json", "file containing an ObjectList of builds")
	cmd.Flags().BoolVar(&o.ImageChangeOnly, "image-change-only", true, "if true(default), only analyze builds that were image change triggered")
	cmd.Flags().BoolVar(&o.PushTimes, "push-times", false, "if true, dump push times for successful builds")

	return cmd
}

func (o *BuildAnalyzerOptions) Run() error {

	raw, err := ioutil.ReadFile(o.BuildFile)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}
	var buildList buildapi.BuildList
	err = json.Unmarshal(raw, &buildList)
	if err != nil {
		fmt.Printf("Error processing build list: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Processing %d builds from file\n", len(buildList.Items))

	allBuildConfigs := map[string][]buildapi.Build{}
	totalBuilds := 0
	for _, build := range buildList.Items {
		totalBuilds += 1
		appendBuild(allBuildConfigs, build)
	}

	errcnt := 0
	failcnt := 0
	successcnt := 0
	cancelcnt := 0
	aftercnt := 0
	newcnt := 0
	pendingcnt := 0
	runningcnt := 0
	beforecnt := 0
	interestcnt := 0

	failedReasons := map[string]int{}
	errorReasons := map[string]int{}
	newReasons := map[string]int{}
	pendingReasons := map[string]int{}

	for _, builds := range allBuildConfigs {

		hasPriorSuccess := false
		// All builds for each buildconfig list are sorted from oldest to newest, so we will be processing them
		// in that order.  That means once we start seeing builds from "after" the trigger time, we know
		// we have already seen all the builds that ran before the trigger time(if any) and can rely on the
		// value of "hasPriorSuccess".
		for _, build := range builds {

			// ignore builds that completed before the earliest time we care about.
			if o.StartTime != nil {
				if build.Status.CompletionTimestamp != nil && build.Status.CompletionTimestamp.Before(*o.StartTime) {
					continue
				}
			}
			if o.TriggerTime != nil && build.Status.CompletionTimestamp != nil && build.Status.CompletionTimestamp.Before(*o.TriggerTime) {
				// the set of builds that completed(successfully or otherwise) before the new set of builds were triggered.
				beforecnt += 1
				if build.Status.Phase == buildapi.BuildPhaseComplete {
					hasPriorSuccess = true
				}
			} else {
				// the set of builds that completed after the new builds were triggered (or are still active)
				if o.ImageChangeOnly && (len(build.Spec.TriggeredBy) != 1 || build.Spec.TriggeredBy[0].ImageChangeBuild == nil) {
					// skip builds not triggered by the imagestream update
					continue
				}
				if o.PushTimes {
					for _, s := range build.Status.Stages {
						if s.Name == "PushImage" {
							fmt.Printf("Push for build %s/%s took %dms\n", build.Namespace, build.Name, s.DurationMilliseconds)
						}
					}
				}

				aftercnt += 1
				// if no triggertime was provided, treat all builds as if they had a prior success because there
				// will be no prior builds to look at.
				if o.TriggerTime == nil || hasPriorSuccess {
					interestcnt += 1
					switch build.Status.Phase {
					case buildapi.BuildPhaseComplete:
						successcnt += 1
					case buildapi.BuildPhaseFailed:
						failcnt += 1
						failedReasons[string(build.Status.Reason)+":"+string(build.Status.Message)] += 1
						if o.TestClone {
							if string(build.Status.Reason) == string(buildinternalapi.StatusReasonFetchSourceFailed) {
								//fmt.Printf("Attempting to clone %s\n", build.Spec.Source.Git.URI)
								err := exec.Command("/bin/sh", "-c", "GIT_TERMINAL_PROMPT=0 git clone -q "+build.Spec.Source.Git.URI).Run()
								if err == nil {
									// TODO: test git checkout of the source ref.  Many builds successfully cloned but
									// the ref was invalid, which failed the build.
									fmt.Printf("Successfully cloned %s but build %s/%s failed, ref is %s\n", build.Spec.Source.Git.URI, build.Namespace, build.Name, build.Spec.Source.Git.Ref)
								}
								//fmt.Println("Done.")
							}
						}
					case buildapi.BuildPhaseError:
						errcnt += 1
						errorReasons[string(build.Status.Reason)+":"+string(build.Status.Message)] += 1
					case buildapi.BuildPhaseCancelled:
						cancelcnt += 1
					case buildapi.BuildPhaseNew:
						newcnt += 1
						newReasons[string(build.Status.Reason)+":"+string(build.Status.Message)] += 1
					case buildapi.BuildPhasePending:
						pendingcnt += 1
						pendingReasons[string(build.Status.Reason)+":"+string(build.Status.Message)] += 1
					case buildapi.BuildPhaseRunning:
						runningcnt += 1
					}
				}
			}
		}
	}

	fmt.Printf("pre-trigger time builds: %d\npost-trigger time builds: %d\n", beforecnt, aftercnt)

	// interesting builds:
	// 1) occurred after the trigger time (if provided, otherwise true for all builds)
	// 2) were imagechangetriggered (unless --image-change-only=false, in which case all build qualify here)
	// 3) had a successful build prior to the trigger time (unless no trigger time is provided in which case
	//    all builds are treated as if they had a prior success)
	fmt.Printf("interesting builds: %d\nnew:%d\npending: %d\nrunning:%d\nsuccess: %d\nfail: %d\nerror: %d\ncanceled: %d\n", interestcnt, newcnt, pendingcnt, runningcnt, successcnt, failcnt, errcnt, cancelcnt)

	if len(newReasons) > 0 {
		fmt.Println("\n\nNew state reasons:")
		for r, c := range newReasons {
			fmt.Printf("Reason=%s Count=%d\n", r, c)
		}
	}

	if len(pendingReasons) > 0 {
		fmt.Println("\n\nPending state reasons:")
		for r, c := range pendingReasons {
			fmt.Printf("Reason=%s Count=%d\n", r, c)
		}
	}

	if len(failedReasons) > 0 {
		fmt.Println("\n\nFailed state reasons:")
		for r, c := range failedReasons {
			fmt.Printf("Reason=%s Count=%d\n", r, c)
		}
	}
	if len(errorReasons) > 0 {
		fmt.Println("\n\nError state reasons:")
		for r, c := range errorReasons {
			fmt.Printf("Reason=%s Count=%d\n", r, c)
		}
	}

	return nil
}

func appendBuild(bcs map[string][]buildapi.Build, build buildapi.Build) {
	bc := build.Annotations[buildinternalapi.BuildConfigAnnotation]
	if len(bc) == 0 {
		fmt.Printf("Skipping build with no buildconfig: %s\n", build.Name)
		return
	}
	if len(bcs[bc]) == 0 {
		bcs[bc] = append(bcs[bc], build)
		return
	}

	f := false
	for i, b := range bcs[bc] {
		if build.CreationTimestamp.Before(b.CreationTimestamp) {
			if i == 0 {
				bcs[bc] = append([]buildapi.Build{build}, bcs[bc]...)
				f = true
				break
			}
			bcs[bc] = append(bcs[bc][0:i], append([]buildapi.Build{build}, bcs[bc][i:]...)...)
			f = true
			break
		}
	}
	if !f {
		bcs[bc] = append(bcs[bc], build)
	}

}
