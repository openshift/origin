package nodeaccess

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"

	"k8s.io/client-go/kubernetes"
)

// this is copy/pasted from the oc node logs impl
func GetDirectoryListing(in io.Reader) ([]string, error) {
	filenames := []string{}
	bufferSize := 4096
	buf := bufio.NewReaderSize(in, bufferSize)

	// turn href links into lines of output
	content, _ := buf.Peek(bufferSize)
	if bytes.HasPrefix(content, []byte("<pre>")) {
		reLink := regexp.MustCompile(`href="([^"]+)"`)
		s := bufio.NewScanner(buf)
		s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			matches := reLink.FindSubmatchIndex(data)
			if matches == nil {
				advance = bytes.LastIndex(data, []byte("\n"))
				if advance == -1 {
					advance = 0
				}
				return advance, nil, nil
			}
			advance = matches[1]
			token = data[matches[2]:matches[3]]
			return advance, token, nil
		})
		for s.Scan() {
			filename := s.Text()
			filenames = append(filenames, filename)
		}
		return filenames, s.Err()
	}

	return nil, fmt.Errorf("not a directory listing")
}

func StreamNodeLogFile(ctx context.Context, client kubernetes.Interface, nodeName, filename string) (io.ReadCloser, error) {
	path := client.CoreV1().RESTClient().Get().
		Namespace("").Name(nodeName).
		Resource("nodes").SubResource("proxy", "logs").Suffix(filename).URL().Path

	req := client.CoreV1().RESTClient().Get().RequestURI(path).
		SetHeader("Accept", "text/plain, */*")

	return req.Stream(ctx)
}

func GetNodeLogFile(ctx context.Context, client kubernetes.Interface, nodeName, filename string) ([]byte, error) {
	in, err := StreamNodeLogFile(ctx, client, nodeName, filename)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	return ioutil.ReadAll(in)
}
