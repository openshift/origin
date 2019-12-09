package encryption

import (
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/operator/v1"
)

type GetOperatorConditionsFuncType func(t testing.TB) ([]v1.OperatorCondition, error)

type PerfScenario struct {
	BasicScenario
	GetOperatorConditionsFunc GetOperatorConditionsFuncType

	DBLoaderFunc          DBLoaderFuncType
	AssertDBPopulatedFunc func(t testing.TB, errorStore map[string]int, statStore map[string]int)
	AssertMigrationTime   func(t testing.TB, migrationTime time.Duration)
	// DBLoaderWorker is the number of workers that will execute DBLoaderFunc
	DBLoaderWorkers int
}

func TestPerfEncryptionTypeAESCBC(t *testing.T, scenario PerfScenario) {
	e := NewE(t, PrintEventsOnFailure(scenario.OperatorNamespace))
	migrationStartedCh := make(chan time.Time, 1)

	populateDatabase(e, scenario.DBLoaderWorkers, scenario.DBLoaderFunc, scenario.AssertDBPopulatedFunc)
	watchForMigrationControllerProgressingConditionAsync(e, scenario.GetOperatorConditionsFunc, migrationStartedCh)
	endTimeStamp := runTestEncryptionTypeAESCBCScenario(t, scenario.BasicScenario)

	select {
	case migrationStarted := <-migrationStartedCh:
		scenario.AssertMigrationTime(e, endTimeStamp.Sub(migrationStarted))
	default:
		e.Error("unable to calculate the migration time, failed to observe when the migration has started")
	}
}

func runTestEncryptionTypeAESCBCScenario(tt *testing.T, scenario BasicScenario) time.Time {
	var ts time.Time
	TestEncryptionTypeAESCBC(tt, BasicScenario{
		Namespace:                       scenario.Namespace,
		LabelSelector:                   scenario.LabelSelector,
		EncryptionConfigSecretName:      scenario.EncryptionConfigSecretName,
		EncryptionConfigSecretNamespace: scenario.EncryptionConfigSecretNamespace,
		OperatorNamespace:               scenario.OperatorNamespace,
		TargetGRs:                       scenario.TargetGRs,
		AssertFunc: func(t testing.TB, clientSet ClientSet, expectedMode configv1.EncryptionType, namespace, labelSelector string) {
			// Note that AssertFunc is executed after an encryption secret has been annotated
			ts = time.Now()
			scenario.AssertFunc(t, clientSet, expectedMode, scenario.Namespace, scenario.LabelSelector)
			t.Logf("AssertFunc for TestEncryptionTypeAESCBC scenario took %v", time.Now().Sub(ts))
		},
	})
	return ts
}
