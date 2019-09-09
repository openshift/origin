package conv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/go-openapi/strfmt"
)

func TestDateTimeValue(t *testing.T) {
	assert.Equal(t, strfmt.DateTime{}, DateTimeValue(nil))
	time := strfmt.DateTime(time.Now())
	assert.Equal(t, time, DateTimeValue(&time))
}
