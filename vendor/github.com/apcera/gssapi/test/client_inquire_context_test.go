// Copyright 2013-2015 Apcera Inc. All rights reserved.

// +build clienttest

package test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func verifyInquireContextResult(t *testing.T, result string, regexps []string) {
	rr := strings.Split(result, " ")
	if len(rr) != len(regexps) {
		t.Fatalf("got %v fragments, expected %v (%s)", len(rr), len(regexps), result)
	}

	for i, r := range rr {
		rx := regexp.MustCompile(regexps[i])
		if !rx.MatchString(r) {
			t.Errorf("%s does not match %s", r, regexps[i])
		}
	}
}

func TestClientInquireContext(t *testing.T) {
	ctx, r := initClientContext(t, "GET", "/inquire_context/", nil)
	defer ctx.Release()

	srcName, targetName, lifetimeRec, mechType, ctxFlags,
		locallyInitiated, open, err := ctx.InquireContext()
	if err != nil {
		t.Fatal(err)
	}
	defer srcName.Release()
	defer targetName.Release()

	verifyInquireContextResult(t,
		fmt.Sprintf("%q %q %v %q %x %v %v",
			srcName, targetName, lifetimeRec, mechType.DebugString(), ctxFlags,
			locallyInitiated, open),
		[]string{
			`"[a-zA-Z_-]+@[[:graph:]]+"`,
			`"HTTP/[[:graph:]]+@[[:graph:]]+"`,
			`[0-9a-z]+`,
			`[A-Z]+`,
			"1b0",
			"true",
			"true",
		})

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %v", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	verifyInquireContextResult(t, string(body),
		[]string{
			`"[a-zA-Z_-]+@[[:graph:]]+"`,
			`"HTTP/[[:graph:]]+@[[:graph:]]+"`,
			`[0-9a-z]+`,
			`[A-Z]+`,
			"130",
			"false",
			"true",
		})
}
