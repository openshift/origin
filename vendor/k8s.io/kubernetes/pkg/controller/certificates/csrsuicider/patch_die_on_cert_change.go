package csrsuicider

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/controller/fileobserver"
)

// this is a global side-effect because we have the file content in package A and the stopChannel/run signal in
// package B, so package A creates this and package B starts it
var csrObserver fileobserver.Observer

type FileAndContent struct {
	filename string
	content  []byte
}

func NewFile(filename string, content []byte) FileAndContent {
	return FileAndContent{filename: filename, content: content}
}

// StartingCertValues sets the global observer
func StartingCertValues(cert, key FileAndContent) {
	fileCheckFrequence := 1 * time.Minute
	observer, err := fileobserver.NewObserver(fileCheckFrequence)
	if err != nil {
		panic(err)
	}

	var once sync.Once
	restartFn := func(filename string, action fileobserver.ActionType) error {
		once.Do(func() {
			klog.Warning(fmt.Sprintf("Restart triggered because of %s", action.String(filename)))
			// no graceful shutdown for a KCM
			klog.Fatalf("Restart triggered because of %s", action.String(filename))
		})
		return nil
	}

	fileContent := map[string][]byte{
		cert.filename: cert.content,
		key.filename:  key.content,
	}

	observer.AddReactor(restartFn, fileContent, sets.StringKeySet(fileContent).List()...)

	csrObserver = observer
}

func DieOnCertChange(stopCh <-chan struct{}) {
	if csrObserver == nil {
		return
	}

	go csrObserver.Run(stopCh)
}
