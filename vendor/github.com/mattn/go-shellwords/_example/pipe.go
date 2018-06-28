// +build ignore

package main

import (
	"fmt"
	"log"

	"github.com/mattn/go-shellwords"
)

func isSpace(r byte) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

func main() {
	line := `
	/usr/bin/ls -la | sort 2>&1 | tee files.log
	`
	parser := shellwords.NewParser()

	for {
		args, err := parser.Parse(line)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(args)
		if parser.Position < 0 {
			break
		}
		i := parser.Position
		for ; i < len(line); i++ {
			if isSpace(line[i]) {
				break
			}
		}
		fmt.Println(string([]rune(line)[parser.Position:i]))
		line = string([]rune(line)[i+1:])
	}
}
