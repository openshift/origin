package watch

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

func makeSecret() *api.Secret {
	return &api.Secret{
		ObjectMeta: api.ObjectMeta{
			ResourceVersion: "0",
			Name:            "test",
		},
	}
}

func modifySecret(secret *api.Secret, data map[string][]byte) *api.Secret {
	newSecret := *secret
	rv, _ := strconv.Atoi(newSecret.ObjectMeta.ResourceVersion)
	rv++
	newSecret.ObjectMeta.ResourceVersion = strconv.Itoa(rv)
	newSecret.Data = data
	*secret = newSecret
	return &newSecret
}

func TestRetryWatchUntilSimple(t *testing.T) {
	secret := makeSecret()
	fakeWatcher := watch.NewFake()
	go func() {
		for i := 0; i < 100; i++ {
			fakeWatcher.Modify(modifySecret(secret, nil))
		}
		fakeWatcher.Modify(modifySecret(secret, map[string][]byte{"foo": []byte("1")}))
		fakeWatcher.Modify(modifySecret(secret, nil))
	}()

	ok, err := RetryWatchUntil(
		func(rv string) (watch.Interface, error) {
			return fakeWatcher, nil
		},
		func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Modified:
				token, _ := event.Object.(*api.Secret)
				if len(token.Data["foo"]) > 0 {
					return true, nil
				}
			}
			return false, nil
		},
		10*time.Second,
	)
	if !ok {
		t.Errorf("expected condition function to succeed")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRetryWatchUntilReportError(t *testing.T) {
	secret := makeSecret()
	fakeWatcher := watch.NewFake()
	go func() {
		for i := 0; i < 100; i++ {
			fakeWatcher.Modify(modifySecret(secret, nil))
		}
		fakeWatcher.Modify(modifySecret(secret, map[string][]byte{"foo": []byte("1")}))
		fakeWatcher.Modify(modifySecret(secret, nil))
	}()
	expectErr := errors.New("error")

	_, err := RetryWatchUntil(
		func(rv string) (watch.Interface, error) {
			return fakeWatcher, nil
		},
		func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Modified:
				token, _ := event.Object.(*api.Secret)
				if len(token.Data["foo"]) > 0 {
					return false, expectErr
				}
			}
			return false, nil
		},
		10*time.Second,
	)
	if err == nil {
		t.Errorf("expected error to be reported")
	}
	if err != expectErr {
		t.Errorf("expected error %v, got: %v", expectErr, err)
	}
}

func TestRetryWatchUntilTimeout(t *testing.T) {
	secret := makeSecret()
	fakeWatcher := watch.NewFake()
	go func() {
		for i := 0; i < 100; i++ {
			fakeWatcher.Modify(modifySecret(secret, nil))
		}
	}()
	_, err := RetryWatchUntil(
		func(rv string) (watch.Interface, error) {
			return fakeWatcher, nil
		},
		func(event watch.Event) (bool, error) {
			return false, nil
		},
		2*time.Second,
	)
	if err == nil {
		t.Errorf("expected timeout error")
	}
	if err != wait.ErrWaitTimeout {
		t.Errorf("expected error %v, got: %v", wait.ErrWaitTimeout, err)
	}
}

func TestRetryWatchUntilWillRetry(t *testing.T) {
	secret := makeSecret()
	numWatcherStarted := 0
	var lastResourceVersion string

	// In this test, we start a watcher and fire up 100 modify events. The we stop the
	// watcher and return which should signal the RetryWatchUntil function to re-open the
	// watcher (re-call the watcher function).
	ok, err := RetryWatchUntil(
		func(rv string) (watch.Interface, error) {
			fakeWatcher := watch.NewFake()
			go func() {
				for i := 0; i < 100; i++ {
					fakeWatcher.Modify(modifySecret(secret, nil))
				}
				lastResourceVersion = secret.ObjectMeta.ResourceVersion
				rv, _ := strconv.Atoi(lastResourceVersion)
				if rv == 100 {
					fakeWatcher.Stop()
					return
				}
				if rv > 150 {
					fakeWatcher.Modify(modifySecret(secret, map[string][]byte{"foo": []byte("1")}))
				}
			}()
			numWatcherStarted++
			return fakeWatcher, nil
		},
		func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Modified:
				token, _ := event.Object.(*api.Secret)
				if len(token.Data["foo"]) > 0 {
					return true, nil
				}
			}
			return false, nil
		},
		10*time.Second,
	)
	if numWatcherStarted != 2 {
		t.Errorf("the watch was not re-opened properly")
	}
	if !ok {
		t.Errorf("expected condition function to succeed")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
