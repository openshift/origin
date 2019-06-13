package file

import (
	"bufio"
	"io/ioutil"
	"os"
)

// ReadLines reads the content of the given file into a string slice
func ReadLines(fileName string) ([]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// LoadData reads the specified file and returns it as a bytes slice.
func LoadData(file string) ([]byte, error) {
	if len(file) == 0 {
		return []byte{}, nil
	}

	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}
