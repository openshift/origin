package lockfile

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/containers/storage/pkg/reexec"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Warning: this is not an exhaustive set of tests.

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

// subTouchMain is a child process which opens the lock file, closes stdout to
// indicate that it has acquired the lock, waits for stdin to get closed,
// updates the last-writer for the lockfile, and then unlocks the file.
func subTouchMain() {
	if len(os.Args) != 2 {
		logrus.Fatalf("expected two args, got %d", len(os.Args))
	}
	tf, err := GetLockfile(os.Args[1])
	if err != nil {
		logrus.Fatalf("error opening lock file %q: %v", os.Args[1], err)
	}
	tf.Lock()
	os.Stdout.Close()
	io.Copy(ioutil.Discard, os.Stdin)
	tf.Touch()
	tf.Unlock()
}

// subTouch starts a child process.  If it doesn't return an error, the caller
// should wait for the first ReadCloser by reading it until it receives an EOF.
// At that point, the child will have acquired the lock.  It can then signal
// that the child should Touch() the lock by closing the WriteCloser.  The
// second ReadCloser will be closed when the child has finished.
func subTouch(l *namedLocker) (io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	cmd := reexec.Command("subTouch", l.name)
	wc, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	rc, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	ec, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	go func() {
		if err = cmd.Run(); err != nil {
			logrus.Errorf("error running subTouch: %v", err)
		}
	}()
	return wc, rc, ec, nil
}

// subLockMain is a child process which opens the lock file, closes stdout to
// indicate that it has acquired the lock, waits for stdin to get closed, and
// then unlocks the file.
func subLockMain() {
	if len(os.Args) != 2 {
		logrus.Fatalf("expected two args, got %d", len(os.Args))
	}
	tf, err := GetLockfile(os.Args[1])
	if err != nil {
		logrus.Fatalf("error opening lock file %q: %v", os.Args[1], err)
	}
	tf.Lock()
	os.Stdout.Close()
	io.Copy(ioutil.Discard, os.Stdin)
	tf.Unlock()
}

// subLock starts a child process.  If it doesn't return an error, the caller
// should wait for the first ReadCloser by reading it until it receives an EOF.
// At that point, the child will have acquired the lock.  It can then signal
// that the child should release the lock by closing the WriteCloser.
func subLock(l *namedLocker) (io.WriteCloser, io.ReadCloser, error) {
	cmd := reexec.Command("subLock", l.name)
	wc, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	rc, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	go func() {
		if err = cmd.Run(); err != nil {
			logrus.Errorf("error running subLock: %v", err)
		}
	}()
	return wc, rc, nil
}

// subRecursiveLockMain is a child process which opens the lock file, closes
// stdout to indicate that it has acquired the lock, waits for stdin to get
// closed, and then unlocks the file.
func subRecursiveLockMain() {
	if len(os.Args) != 2 {
		logrus.Fatalf("expected two args, got %d", len(os.Args))
	}
	tf, err := GetLockfile(os.Args[1])
	if err != nil {
		logrus.Fatalf("error opening lock file %q: %v", os.Args[1], err)
	}
	tf.RecursiveLock()
	os.Stdout.Close()
	io.Copy(ioutil.Discard, os.Stdin)
	tf.Unlock()
}

// subRecursiveLock starts a child process.  If it doesn't return an error, the
// caller should wait for the first ReadCloser by reading it until it receives
// an EOF. At that point, the child will have acquired the lock.  It can then
// signal that the child should release the lock by closing the WriteCloser.
func subRecursiveLock(l *namedLocker) (io.WriteCloser, io.ReadCloser, error) {
	cmd := reexec.Command("subRecursiveLock", l.name)
	wc, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	rc, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	go func() {
		if err = cmd.Run(); err != nil {
			logrus.Errorf("error running subLock: %v", err)
		}
	}()
	return wc, rc, nil
}

// subRLockMain is a child process which opens the lock file, closes stdout to
// indicate that it has acquired the read lock, waits for stdin to get closed,
// and then unlocks the file.
func subRLockMain() {
	if len(os.Args) != 2 {
		logrus.Fatalf("expected two args, got %d", len(os.Args))
	}
	tf, err := GetLockfile(os.Args[1])
	if err != nil {
		logrus.Fatalf("error opening lock file %q: %v", os.Args[1], err)
	}
	tf.RLock()
	os.Stdout.Close()
	io.Copy(ioutil.Discard, os.Stdin)
	tf.Unlock()
}

// subRLock starts a child process.  If it doesn't return an error, the caller
// should wait for the first ReadCloser by reading it until it receives an EOF.
// At that point, the child will have acquired a read lock.  It can then signal
// that the child should release the lock by closing the WriteCloser.
func subRLock(l *namedLocker) (io.WriteCloser, io.ReadCloser, error) {
	cmd := reexec.Command("subRLock", l.name)
	wc, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	rc, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	go func() {
		if err = cmd.Run(); err != nil {
			logrus.Errorf("error running subRLock: %v", err)
		}
	}()
	return wc, rc, nil
}

func init() {
	reexec.Register("subTouch", subTouchMain)
	reexec.Register("subRLock", subRLockMain)
	reexec.Register("subRecursiveLock", subRecursiveLockMain)
	reexec.Register("subLock", subLockMain)
}

type namedLocker struct {
	Locker
	name string
}

func getNamedLocker(ro bool) (*namedLocker, error) {
	var l Locker
	tf, err := ioutil.TempFile("", "lockfile")
	if err != nil {
		return nil, err
	}
	name := tf.Name()
	tf.Close()
	if ro {
		l, err = GetROLockfile(name)
	} else {
		l, err = GetLockfile(name)
	}
	if err != nil {
		return nil, err
	}
	return &namedLocker{Locker: l, name: name}, nil
}

func getTempLockfile() (*namedLocker, error) {
	return getNamedLocker(false)
}

func getTempROLockfile() (*namedLocker, error) {
	return getNamedLocker(true)
}

func TestLockfileName(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	assert.NotEmpty(t, l.name, "lockfile name should be recorded correctly")

	assert.False(t, l.Locked(), "Locked() said we have a write lock")

	l.RLock()
	assert.False(t, l.Locked(), "Locked() said we have a write lock")
	l.Unlock()

	assert.NotEmpty(t, l.name, "lockfile name should be recorded correctly")

	l.Lock()
	assert.True(t, l.Locked(), "Locked() said we didn't have a write lock")
	l.Unlock()

	assert.NotEmpty(t, l.name, "lockfile name should be recorded correctly")
}

func TestLockfileRead(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	l.RLock()
	assert.False(t, l.Locked(), "Locked() said we have a write lock")
	l.Unlock()
}

func TestROLockfileRead(t *testing.T) {
	l, err := getTempROLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	l.RLock()
	assert.False(t, l.Locked(), "Locked() said we have a write lock")
	l.Unlock()
}

func TestLockfileWrite(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	l.Lock()
	assert.True(t, l.Locked(), "Locked() said we didn't have a write lock")
	l.Unlock()
}

func TestRecursiveLockfileWrite(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	l.RecursiveLock()
	assert.True(t, l.Locked(), "Locked() said we didn't have a write lock")
	l.RecursiveLock()
	l.Unlock()
	l.Unlock()
}

func TestROLockfileWrite(t *testing.T) {
	l, err := getTempROLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	defer func() {
		assert.NotNil(t, recover(), "Should have panicked trying to take a write lock using a read lock")
	}()
	l.Lock()
	assert.False(t, l.Locked(), "Locked() said we have a write lock")
	l.Unlock()
}

func TestLockfileTouch(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	l.Lock()
	m, err := l.Modified()
	require.Nil(t, err, "got an error from Modified()")
	assert.True(t, m, "new lock file does not appear to have changed")

	now := time.Now()
	assert.False(t, l.TouchedSince(now), "file timestamp was updated for no reason")

	time.Sleep(2 * time.Second)
	err = l.Touch()
	require.Nil(t, err, "got an error from Touch()")
	assert.True(t, l.TouchedSince(now), "file timestamp was not updated by Touch()")

	m, err = l.Modified()
	require.Nil(t, err, "got an error from Modified()")
	assert.False(t, m, "lock file mistakenly indicated that someone else has modified it")

	stdin, stdout, stderr, err := subTouch(l)
	require.Nil(t, err, "got an error starting a subprocess to touch the lockfile")
	l.Unlock()
	io.Copy(ioutil.Discard, stdout)
	stdin.Close()
	io.Copy(ioutil.Discard, stderr)
	l.Lock()
	m, err = l.Modified()
	l.Unlock()
	require.Nil(t, err, "got an error from Modified()")
	assert.True(t, m, "lock file failed to notice that someone else modified it")
}

func TestLockfileWriteConcurrent(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)
	var wg sync.WaitGroup
	var highestMutex sync.Mutex
	var counter, highest int64
	for i := 0; i < 100000; i++ {
		wg.Add(1)
		go func() {
			l.Lock()
			tmp := atomic.AddInt64(&counter, 1)
			assert.True(t, tmp >= 0, "counter should never be less than zero")
			highestMutex.Lock()
			if tmp > highest {
				// multiple writers should not be able to hold
				// this lock at the same time, so there should
				// be no point at which two goroutines are
				// between the AddInt64() above and the one
				// below
				highest = tmp
			}
			highestMutex.Unlock()
			atomic.AddInt64(&counter, -1)
			l.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
	assert.True(t, highest == 1, "counter should never have gone above 1, got to %d", highest)
}

func TestLockfileReadConcurrent(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	// the test below is inspired by the stdlib's rwmutex tests
	numReaders := 1000
	locked := make(chan bool)
	unlocked := make(chan bool)
	done := make(chan bool)

	for i := 0; i < numReaders; i++ {
		go func() {
			l.RLock()
			locked <- true
			<-unlocked
			l.Unlock()
			done <- true
		}()
	}

	// Wait for all parallel locks to succeed
	for i := 0; i < numReaders; i++ {
		<-locked
	}
	// Instruct all parallel locks to unlock
	for i := 0; i < numReaders; i++ {
		unlocked <- true
	}
	// Wait for all parallel locks to be unlocked
	for i := 0; i < numReaders; i++ {
		<-done
	}
}

func TestLockfileRecursiveWrite(t *testing.T) {
	// NOTE: given we're in the same process space, it's effectively the same as
	// reader lock.

	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	// the test below is inspired by the stdlib's rwmutex tests
	numReaders := 1000
	locked := make(chan bool)
	unlocked := make(chan bool)
	done := make(chan bool)

	for i := 0; i < numReaders; i++ {
		go func() {
			l.RecursiveLock()
			locked <- true
			<-unlocked
			l.Unlock()
			done <- true
		}()
	}

	// Wait for all parallel locks to succeed
	for i := 0; i < numReaders; i++ {
		<-locked
	}
	// Instruct all parallel locks to unlock
	for i := 0; i < numReaders; i++ {
		unlocked <- true
	}
	// Wait for all parallel locks to be unlocked
	for i := 0; i < numReaders; i++ {
		<-done
	}
}

func TestLockfileMixedConcurrent(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	counter := int32(0)
	diff := int32(10000)
	numIterations := 10
	numReaders := 100
	numWriters := 50

	done := make(chan bool)

	// A writer always adds `diff` to the counter. Hence, `diff` is the
	// only valid value in the critical section.
	writer := func(c *int32) {
		for i := 0; i < numIterations; i++ {
			l.Lock()
			tmp := atomic.AddInt32(c, diff)
			assert.True(t, tmp == diff, "counter should be %d but instead is %d", diff, tmp)
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt32(c, diff*(-1))
			l.Unlock()
		}
		done <- true
	}

	// A reader always adds `1` to the counter. Hence,
	// [1,`numReaders*numIterations`] are valid values.
	reader := func(c *int32) {
		for i := 0; i < numIterations; i++ {
			l.RLock()
			tmp := atomic.AddInt32(c, 1)
			assert.True(t, tmp >= 1 && tmp < diff)
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt32(c, -1)
			l.Unlock()
		}
		done <- true
	}

	for i := 0; i < numReaders; i++ {
		go reader(&counter)
		// schedule a writer every 2nd iteration
		if i%2 == 1 {
			go writer(&counter)
		}
	}

	for i := 0; i < numReaders+numWriters; i++ {
		<-done
	}
}

func TestLockfileMixedConcurrentRecursiveWriters(t *testing.T) {
	// It's effectively the same tests as with mixed readers & writers but calling
	// RecursiveLocks() instead.

	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)

	counter := int32(0)
	diff := int32(10000)
	numIterations := 10
	numReaders := 100
	numWriters := 50

	done := make(chan bool)

	// A writer always adds `diff` to the counter. Hence, `diff` is the
	// only valid value in the critical section.
	writer := func(c *int32) {
		for i := 0; i < numIterations; i++ {
			l.Lock()
			tmp := atomic.AddInt32(c, diff)
			assert.True(t, tmp == diff, "counter should be %d but instead is %d", diff, tmp)
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt32(c, diff*(-1))
			l.Unlock()
		}
		done <- true
	}

	// A reader always adds `1` to the counter. Hence,
	// [1,`numReaders*numIterations`] are valid values.
	reader := func(c *int32) {
		for i := 0; i < numIterations; i++ {
			l.RecursiveLock()
			tmp := atomic.AddInt32(c, 1)
			assert.True(t, tmp >= 1 && tmp < diff)
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt32(c, -1)
			l.Unlock()
		}
		done <- true
	}

	for i := 0; i < numReaders; i++ {
		go reader(&counter)
		// schedule a writer every 2nd iteration
		if i%2 == 1 {
			go writer(&counter)
		}
	}

	for i := 0; i < numReaders+numWriters; i++ {
		<-done
	}
}

func TestLockfileMultiprocessRead(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)
	var wg sync.WaitGroup
	var rcounter, rhighest int64
	var highestMutex sync.Mutex
	subs := make([]struct {
		stdin  io.WriteCloser
		stdout io.ReadCloser
	}, 100)
	for i := range subs {
		stdin, stdout, err := subRLock(l)
		require.Nil(t, err, "error starting subprocess %d to take a read lock", i+1)
		subs[i].stdin = stdin
		subs[i].stdout = stdout
	}
	for i := range subs {
		wg.Add(1)
		go func(i int) {
			io.Copy(ioutil.Discard, subs[i].stdout)
			if testing.Verbose() {
				fmt.Printf("\tchild %4d acquired the read lock\n", i+1)
			}
			atomic.AddInt64(&rcounter, 1)
			highestMutex.Lock()
			if rcounter > rhighest {
				rhighest = rcounter
			}
			highestMutex.Unlock()
			time.Sleep(1 * time.Second)
			atomic.AddInt64(&rcounter, -1)
			if testing.Verbose() {
				fmt.Printf("\ttelling child %4d to release the read lock\n", i+1)
			}
			subs[i].stdin.Close()
			wg.Done()
		}(i)
	}
	wg.Wait()
	assert.True(t, rhighest > 1, "expected to have multiple reader locks at least once, only had %d", rhighest)
}

func TestLockfileMultiprocessWrite(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)
	var wg sync.WaitGroup
	var wcounter, whighest int64
	var highestMutex sync.Mutex
	subs := make([]struct {
		stdin  io.WriteCloser
		stdout io.ReadCloser
	}, 10)
	for i := range subs {
		stdin, stdout, err := subLock(l)
		require.Nil(t, err, "error starting subprocess %d to take a write lock", i+1)
		subs[i].stdin = stdin
		subs[i].stdout = stdout
	}
	for i := range subs {
		wg.Add(1)
		go func(i int) {
			io.Copy(ioutil.Discard, subs[i].stdout)
			if testing.Verbose() {
				fmt.Printf("\tchild %4d acquired the write lock\n", i+1)
			}
			atomic.AddInt64(&wcounter, 1)
			highestMutex.Lock()
			if wcounter > whighest {
				whighest = wcounter
			}
			highestMutex.Unlock()
			time.Sleep(1 * time.Second)
			atomic.AddInt64(&wcounter, -1)
			if testing.Verbose() {
				fmt.Printf("\ttelling child %4d to release the write lock\n", i+1)
			}
			subs[i].stdin.Close()
			wg.Done()
		}(i)
	}
	wg.Wait()
	assert.True(t, whighest == 1, "expected to have no more than one writer lock active at a time, had %d", whighest)
}

func TestLockfileMultiprocessRecursiveWrite(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)
	var wg sync.WaitGroup
	var wcounter, whighest int64
	var highestMutex sync.Mutex
	subs := make([]struct {
		stdin  io.WriteCloser
		stdout io.ReadCloser
	}, 10)
	for i := range subs {
		stdin, stdout, err := subRecursiveLock(l)
		require.Nil(t, err, "error starting subprocess %d to take a write lock", i+1)
		subs[i].stdin = stdin
		subs[i].stdout = stdout
	}
	for i := range subs {
		wg.Add(1)
		go func(i int) {
			io.Copy(ioutil.Discard, subs[i].stdout)
			if testing.Verbose() {
				fmt.Printf("\tchild %4d acquired the recursive write lock\n", i+1)
			}
			atomic.AddInt64(&wcounter, 1)
			highestMutex.Lock()
			if wcounter > whighest {
				whighest = wcounter
			}
			highestMutex.Unlock()
			time.Sleep(1 * time.Second)
			atomic.AddInt64(&wcounter, -1)
			if testing.Verbose() {
				fmt.Printf("\ttelling child %4d to release the recursive write lock\n", i+1)
			}
			subs[i].stdin.Close()
			wg.Done()
		}(i)
	}
	wg.Wait()
	assert.True(t, whighest == 1, "expected to have no more than one writer lock active at a time, had %d", whighest)
}

func TestLockfileMultiprocessMixed(t *testing.T) {
	l, err := getTempLockfile()
	require.Nil(t, err, "error getting temporary lock file")
	defer os.Remove(l.name)
	var wg sync.WaitGroup
	var rcounter, wcounter, rhighest, whighest int64
	var rhighestMutex, whighestMutex sync.Mutex
	bias_p := 1
	bias_q := 10
	groups := 15
	writer := func(i int) bool { return (i % bias_q) < bias_p }
	subs := make([]struct {
		stdin  io.WriteCloser
		stdout io.ReadCloser
	}, bias_q*groups)
	for i := range subs {
		var stdin io.WriteCloser
		var stdout io.ReadCloser
		if writer(i) {
			stdin, stdout, err = subLock(l)
			require.Nil(t, err, "error starting subprocess %d to take a write lock", i+1)
		} else {
			stdin, stdout, err = subRLock(l)
			require.Nil(t, err, "error starting subprocess %d to take a read lock", i+1)
		}
		subs[i].stdin = stdin
		subs[i].stdout = stdout
	}
	for i := range subs {
		wg.Add(1)
		go func(i int) {
			// wait for the child to acquire whatever lock it wants
			io.Copy(ioutil.Discard, subs[i].stdout)
			if writer(i) {
				// child acquired a write lock
				if testing.Verbose() {
					fmt.Printf("\tchild %4d acquired the write lock\n", i+1)
				}
				atomic.AddInt64(&wcounter, 1)
				whighestMutex.Lock()
				if wcounter > whighest {
					whighest = wcounter
				}
				require.Zero(t, rcounter, "acquired a write lock while we appear to have read locks")
				whighestMutex.Unlock()
			} else {
				// child acquired a read lock
				if testing.Verbose() {
					fmt.Printf("\tchild %4d acquired the read lock\n", i+1)
				}
				atomic.AddInt64(&rcounter, 1)
				rhighestMutex.Lock()
				if rcounter > rhighest {
					rhighest = rcounter
				}
				require.Zero(t, wcounter, "acquired a read lock while we appear to have write locks")
				rhighestMutex.Unlock()
			}
			time.Sleep(1 * time.Second)
			if writer(i) {
				atomic.AddInt64(&wcounter, -1)
				if testing.Verbose() {
					fmt.Printf("\ttelling child %4d to release the write lock\n", i+1)
				}
			} else {
				atomic.AddInt64(&rcounter, -1)
				if testing.Verbose() {
					fmt.Printf("\ttelling child %4d to release the read lock\n", i+1)
				}
			}
			subs[i].stdin.Close()
			wg.Done()
		}(i)
	}
	wg.Wait()
	assert.True(t, rhighest > 1, "expected to have more than one reader lock active at a time at least once, only had %d", rhighest)
	assert.True(t, whighest == 1, "expected to have no more than one writer lock active at a time, had %d", whighest)
}
