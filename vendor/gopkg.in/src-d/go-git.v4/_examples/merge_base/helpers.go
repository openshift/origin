package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func checkIfError(err error, code exitCode, mainReason string, v ...interface{}) {
	if err == nil {
		return
	}

	printErr(wrappErr(err, mainReason, v...))
	os.Exit(int(code))
}

func helpAndExit(s string, helpMsg string, code exitCode) {
	if code == exitCodeSuccess {
		printMsg("%s", s)
	} else {
		printErr(fmt.Errorf(s))
	}

	fmt.Println(strings.Replace(helpMsg, "%_COMMAND_NAME_%", os.Args[0], -1))

	os.Exit(int(code))
}

func printErr(err error) {
	fmt.Printf("\x1b[31;1m%s\x1b[0m\n", fmt.Sprintf("error: %s", err))
}

func printMsg(format string, args ...interface{}) {
	fmt.Printf("%s\n", fmt.Sprintf(format, args...))
}

func printCommits(commits []*object.Commit) {
	for _, commit := range commits {
		if os.Getenv("LOG_LEVEL") == "verbose" {
			fmt.Printf(
				"\x1b[36;1m%s \x1b[90;21m%s\x1b[0m %s\n",
				commit.Hash.String()[:7],
				commit.Hash.String(),
				strings.Split(commit.Message, "\n")[0],
			)
		} else {
			fmt.Println(commit.Hash.String())
		}
	}
}

func wrappErr(err error, s string, v ...interface{}) error {
	if err != nil {
		return fmt.Errorf("%s\n  %s", fmt.Sprintf(s, v...), err)
	}

	return nil
}
