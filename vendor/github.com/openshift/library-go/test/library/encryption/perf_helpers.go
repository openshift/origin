package encryption

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	operatorv1 "github.com/openshift/api/operator/v1"
	test "github.com/openshift/library-go/test/library"
)

func watchForMigrationControllerProgressingConditionAsync(t testing.TB, getOperatorCondFn GetOperatorConditionsFuncType, migrationStartedCh chan time.Time) {
	t.Helper()
	go watchForMigrationControllerProgressingCondition(t, getOperatorCondFn, migrationStartedCh)
}

func watchForMigrationControllerProgressingCondition(t testing.TB, getOperatorConditionsFn GetOperatorConditionsFuncType, migrationStartedCh chan time.Time) {
	t.Helper()

	t.Logf("Waiting up to %s for the condition %q with the reason %q to be set to true", waitPollTimeout.String(), "EncryptionMigrationControllerProgressing", "Migrating")
	err := wait.Poll(waitPollInterval, waitPollTimeout, func() (bool, error) {
		conditions, err := getOperatorConditionsFn(t)
		if err != nil {
			return false, err
		}
		for _, cond := range conditions {
			if cond.Type == "EncryptionMigrationControllerProgressing" && cond.Status == operatorv1.ConditionTrue {
				t.Logf("EncryptionMigrationControllerProgressing condition observed at %v", cond.LastTransitionTime)
				migrationStartedCh <- cond.LastTransitionTime.Time
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Logf("failed waiting for the condition %q with the reason %q to be set to true", "EncryptionMigrationControllerProgressing", "Migrating")
	}
}

func populateDatabase(t testing.TB, workers int, dbLoaderFun DBLoaderFuncType, assertDBPopulatedFunc func(t testing.TB, errorStore map[string]int, statStore map[string]int)) {
	t.Helper()
	start := time.Now()
	defer func() {
		end := time.Now()
		t.Logf("Populating etcd took %v", end.Sub(start))
	}()

	r := newRunner()

	// run executes loaderFunc for each worker
	r.run(t, workers, dbLoaderFun)

	assertDBPopulatedFunc(t, r.errorStore, r.statsStore)
}

type DBLoaderFuncType func(kubernetes.Interface, string, func(error) /*error collector*/, func(string) /*stats collector*/) error

type runner struct {
	errorStore map[string]int
	lock       *sync.Mutex

	statsStore map[string]int
	lockStats  *sync.Mutex
	wg         *sync.WaitGroup
}

func newRunner() *runner {
	r := &runner{}

	r.errorStore = map[string]int{}
	r.lock = &sync.Mutex{}
	r.statsStore = map[string]int{}
	r.lockStats = &sync.Mutex{}

	r.wg = &sync.WaitGroup{}

	return r
}

func (r *runner) run(t testing.TB, workers int, workFunc ...DBLoaderFuncType) {
	t.Logf("Executing provided load function for %d workers", workers)
	for i := 0; i < workers; i++ {
		wrapper := func(wg *sync.WaitGroup) {
			defer wg.Done()
			kubeClient, err := newKubeClient(300, 600)
			if err != nil {
				t.Errorf("Unable to create a kube client for a worker due to %v", err)
				r.collectError(err)
				return
			}
			_ = runWorkFunctions(kubeClient, "", r.collectError, r.collectStat, workFunc...)
		}
		r.wg.Add(1)
		go wrapper(r.wg)
	}
	r.wg.Wait()
	t.Log("All workers completed successfully")
}

func (r *runner) collectError(err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	errCount, ok := r.errorStore[err.Error()]
	if !ok {
		r.errorStore[err.Error()] = 1
		return
	}
	errCount += 1
	r.errorStore[err.Error()] = errCount
}

func (r *runner) collectStat(stat string) {
	r.lockStats.Lock()
	defer r.lockStats.Unlock()
	statCount, ok := r.statsStore[stat]
	if !ok {
		r.statsStore[stat] = 1
		return
	}
	statCount += 1
	r.statsStore[stat] = statCount
}

func runWorkFunctions(kubeClient kubernetes.Interface, namespace string, errorCollector func(error), statsCollector func(string), workFunc ...DBLoaderFuncType) error {
	if len(namespace) == 0 {
		namespace = createNamespaceName()
	}
	for _, work := range workFunc {
		err := work(kubeClient, namespace, errorCollector, statsCollector)
		if err != nil {
			errorCollector(err)
			return err
		}
	}
	return nil
}

func DBLoaderRepeat(times int, genNamespaceName bool, workToRepeatFunc ...DBLoaderFuncType) DBLoaderFuncType {
	return DBLoaderRepeatParallel(times, 1, genNamespaceName, workToRepeatFunc...)
}

func DBLoaderRepeatParallel(times int, workers int, genNamespaceName bool, workToRepeatFunc ...DBLoaderFuncType) DBLoaderFuncType {
	return func(kubeClient kubernetes.Interface, namespace string, errorCollector func(error), statsCollector func(string)) error {
		if times < workers {
			panic("DBLoaderRepeat cannot be < workers")
		}
		wg := sync.WaitGroup{}
		workPerWorker := times / workers
		for w := 0; w < workers; w++ {
			work := func() {
				defer wg.Done()
				for i := 0; i < workPerWorker; i++ {
					if genNamespaceName {
						namespace = createNamespaceName()
					}
					if err := runWorkFunctions(kubeClient, namespace, errorCollector, statsCollector, workToRepeatFunc...); err != nil {
						errorCollector(err)
					}
				}
			}
			wg.Add(1)
			go work()
		}
		wg.Wait()
		return nil
	}
}

func createNamespaceName() string {
	return fmt.Sprintf("encryption-%s", rand.String(10))
}

func newKubeClient(qps float32, burst int) (kubernetes.Interface, error) {
	kubeConfig, err := test.NewClientConfigForTest()
	if err != nil {
		return nil, err
	}

	kubeConfig.QPS = qps
	kubeConfig.Burst = burst

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}
