package templaterouter

import "time"

// NewFakeTemplateRouter provides an empty template router with a simple certificate manager
// backed by a fake cert writer for testing
func NewFakeTemplateRouter() *templateRouter {
	fakeCertManager, _ := newSimpleCertificateManager(newFakeCertificateManagerConfig(), &fakeCertWriter{})
	return &templateRouter{
		state:        map[string]ServiceAliasConfig{},
		serviceUnits: make(map[string]ServiceUnit),
		certManager:  fakeCertManager,
	}
}

// FakeReloadHandler implements the minimal changes needed to make the locking behavior work
// This MUST match the behavior with the object updates of commitAndReload() in router.go
func (r *templateRouter) FakeReloadHandler() {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.stateChanged = false
	r.lastReloadStart = time.Now()
	r.lastReloadEnd = time.Now()

	return
}

// fakeCertWriter is a certificate writer that records actions but is a no-op
type fakeCertWriter struct {
	addedCerts   []string
	deletedCerts []string
}

// clear clears the fake cert writer for test case resets
func (fcw *fakeCertWriter) clear() {
	fcw.addedCerts = make([]string, 0)
	fcw.deletedCerts = make([]string, 0)
}

func (fcw *fakeCertWriter) WriteCertificate(directory string, id string, cert []byte) error {
	fcw.addedCerts = append(fcw.addedCerts, directory+id)
	return nil
}

func (fcw *fakeCertWriter) DeleteCertificate(directory, id string) error {
	fcw.deletedCerts = append(fcw.deletedCerts, directory+id)
	return nil
}

func newFakeCertificateManagerConfig() *certificateManagerConfig {
	return &certificateManagerConfig{
		certKeyFunc:     generateCertKey,
		caCertKeyFunc:   generateCACertKey,
		destCertKeyFunc: generateDestCertKey,
		certDir:         certDir,
		caCertDir:       caCertDir,
	}
}
