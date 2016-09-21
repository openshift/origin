package app

import (
	"bufio"
	"os"
)

// isFile returns true if the passed-in argument is a file in the filesystem
func isFile(name string) bool {
	info, err := os.Stat(name)
	return err == nil && !info.IsDir()
}

// isDirectory returns true if the passed-in argument is a directory in the filesystem
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}

// ReadLinesFromFile reads a whole file into memory and returns string
// array after reading it line by line.
func ReadLinesFromFile(path string) ([]string, error) {
	envFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer envFile.Close()
	var lines []string
	scanner := bufio.NewScanner(envFile)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}
