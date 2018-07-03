package x509

import (
	"testing"
)

func TestTemplateIDs(t *testing.T) {
	for id, template := range idToError {
		if template.ID != id {
			t.Errorf("idToError[%v].id=%v; want %v", id, template.ID, id)
		}
	}
}
