package compat_otp

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/formatter"
	"github.com/onsi/ginkgo/v2/types"
)

// when 4.14 synch with k1.27, there is gingkgo upgrade from 2.4 to 26
// By method changes and it does not print "STEP:" information. some tester want to use it. so, make this wrapper to print
// if you want to get "STEP:", you need to change g.By to exutil.By
// text is the string you want to describe the step.
func By(text string) {

	formatter := formatter.NewWithNoColorBool(true)
	fmt.Println(formatter.F("{{bold}}  STEP:{{/}} %s {{gray}}%s{{/}}", text, time.Now().Format(types.GINKGO_TIME_FORMAT)))
	g.By(text)

}
