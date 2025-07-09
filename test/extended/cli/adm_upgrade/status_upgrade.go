package adm_upgrade

import (
	"context"
	"fmt"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo/v2"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

type ingestingOcAdmUpgradeStatusTest interface {
	name() string
	ingest(output string) error
}

type controlPlaneAssessmentState string

const (
	S01_Initial              controlPlaneAssessmentState = "INITIAL"
	S02_Before               controlPlaneAssessmentState = "BEFORE UPDATE"
	S03_Updating             controlPlaneAssessmentState = "UPDATING"
	S04_ControlPlaneComplete controlPlaneAssessmentState = "CONTROL PLANE DONE"
	S05_Complete             controlPlaneAssessmentState = "DONE"
)

type controlPlaneAssessmentObservation string

const (
	O01_NotUpdating          controlPlaneAssessmentObservation = "NOT_UPDATING"
	O02_Updating             controlPlaneAssessmentObservation = "UPDATING"
	O03_ControlPlaneComplete controlPlaneAssessmentObservation = "CONTROL_PLANE_COMPLETE"

	O05_UnknownAssessment controlPlaneAssessmentObservation = "UNKNOWN_ASSESSMENT"
	O06_Unknown           controlPlaneAssessmentObservation = "UNKNOWN"
)

type controlPlaneAssessmentLifecycleTest struct {
	current controlPlaneAssessmentState
	allowed map[controlPlaneAssessmentState]map[controlPlaneAssessmentObservation]controlPlaneAssessmentState
}

func NewControlPlaneLifecycleAssessmentTest() *controlPlaneAssessmentLifecycleTest {
	allowed := map[controlPlaneAssessmentState]map[controlPlaneAssessmentObservation]controlPlaneAssessmentState{
		S01_Initial: {
			O01_NotUpdating:          S02_Before,
			O02_Updating:             S03_Updating,
			O03_ControlPlaneComplete: S04_ControlPlaneComplete,
		},
		S02_Before: {
			O01_NotUpdating:          S02_Before,
			O02_Updating:             S03_Updating,
			O03_ControlPlaneComplete: S04_ControlPlaneComplete,
		},
		S03_Updating: {
			O01_NotUpdating:          S05_Complete,
			O02_Updating:             S03_Updating,
			O03_ControlPlaneComplete: S04_ControlPlaneComplete,
		},
		S04_ControlPlaneComplete: {
			O01_NotUpdating:          S05_Complete,
			O03_ControlPlaneComplete: S04_ControlPlaneComplete,
		},
		S05_Complete: {
			O01_NotUpdating: S05_Complete,
		},
	}
	return &controlPlaneAssessmentLifecycleTest{
		current: S01_Initial,
		allowed: allowed,
	}
}

func (t *controlPlaneAssessmentLifecycleTest) name() string {
	return "control plane assessment lifecycle"
}

var (
	reNotUpdating = regexp.MustCompile(`The cluster is not updating.`)
	// TODO(muller): Key on = Control Plane = header with multi-line regex
	reUpdating             = regexp.MustCompile(`^Assessment:\w+ (Progressing|Progressing - Slow|Degraded)`)
	reUnknownAssessment    = regexp.MustCompile(`^Assessment:\w+`) // Everything else
	reControlPlaneComplete = regexp.MustCompile(`^Update to \W+ successfully completed at \W+ \(duration: \W+\)$`)
)

func (t *controlPlaneAssessmentLifecycleTest) controlPlaneObservation(output string) (controlPlaneAssessmentObservation, string) {
	if match := reNotUpdating.FindString(output); match != "" {
		return O01_NotUpdating, match
	}
	if match := reUpdating.FindString(output); match != "" {
		return O02_Updating, match
	}
	if match := reControlPlaneComplete.FindString(output); match != "" {
		return O03_ControlPlaneComplete, match
	}
	if match := reUnknownAssessment.FindString(output); match != "" {
		return O05_UnknownAssessment, match
	}

	return O06_Unknown, output
}

func (t *controlPlaneAssessmentLifecycleTest) ingest(output string) error {
	paths := t.allowed[t.current]
	observation, witness := t.controlPlaneObservation(output)
	next, allowed := paths[observation]
	if !allowed {
		// TODO(muller): resets
		return fmt.Errorf("unexpected observed output %s after earlier state %s, witness output:\n %s", observation, t.current, witness)
	}
	t.current = next
	return nil
}

type pollOcUpgradeStatusAndIngest struct {
	interval time.Duration
	test     ingestingOcAdmUpgradeStatusTest

	oc *exutil.CLI
}

func (t *pollOcUpgradeStatusAndIngest) Test(ctx context.Context, f *e2e.Framework) {
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		g.By("Running `oc adm upgrade status` command")
		cmd := t.oc.Run("adm", "upgrade", "status").EnvVar("OC_ENABLE_CMD_UPGRADE_STATUS", "true")
		out, err := cmd.Output()
		e2e.ExpectNoError(err)

		err = t.test.ingest(out)
		e2e.ExpectNoError(err, "%s:`oc adm upgrade status`", t.test.name())
	}, t.interval)

	g.By("Running `oc adm upgrade status` once after update is complete")
	cmd := t.oc.Run("adm", "upgrade", "status").EnvVar("OC_ENABLE_CMD_UPGRADE_STATUS", "true")
	out, err := cmd.Output()
	e2e.ExpectNoError(err)

	err = t.test.ingest(out)
	e2e.ExpectNoError(err, "failed to ingest output from `oc adm upgrade status`")
}

type OcAdmUpgradeStatusTest struct {
	oc *exutil.CLI
}

func (t *OcAdmUpgradeStatusTest) Name() string {
	return "oc-adm-upgrade-status"
}

func (t *OcAdmUpgradeStatusTest) DisplayName() string {
	return "[sig-cli] oc-adm-upgrade-status"
}

func (t *OcAdmUpgradeStatusTest) Setup(_ context.Context, f *e2e.Framework) {
	g.By("Setting up `oc adm upgrade status` test")

	t.oc = exutil.NewCLIWithFramework(f)

	e2e.Logf("Setup for `oc adm upgrade status` complete")
}

func (t *OcAdmUpgradeStatusTest) Test(ctx context.Context, f *e2e.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-done
		cancel()
	}()

	test := &pollOcUpgradeStatusAndIngest{
		interval: 5 * time.Minute,
		test:     NewControlPlaneLifecycleAssessmentTest(),
		oc:       t.oc,
	}
	test.Test(ctx, f)
}

func (t *OcAdmUpgradeStatusTest) Teardown(_ context.Context, _ *e2e.Framework) {
	// Nothing to clean up; everything in `oc adm upgrade status` is CLI-side and read-only
}
