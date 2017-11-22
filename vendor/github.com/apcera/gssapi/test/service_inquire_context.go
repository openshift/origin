// Copyright 2013-2015 Apcera Inc. All rights reserved.

package test

import (
	"fmt"
	"net/http"
)

// HandleInquireContext accepts the context, unwraps, and then outputs its
// parameters obtained with InquireContext
func HandleInquireContext(
	c *Context, w http.ResponseWriter, r *http.Request) (
	code int, message string) {

	ctx, code, message := allowed(c, w, r)
	if ctx == nil {
		return code, message
	}

	srcName, targetName, lifetimeRec, mechType, ctxFlags,
		locallyInitiated, open, err := ctx.InquireContext()
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	defer srcName.Release()
	defer targetName.Release()

	body := fmt.Sprintf("%q %q %v %q %x %v %v",
		srcName, targetName, lifetimeRec, mechType.DebugString(), ctxFlags,
		locallyInitiated, open)

	w.Write([]byte(body))
	return http.StatusOK, "OK"
}
