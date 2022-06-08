package nodedetails

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"k8s.io/client-go/kubernetes"
)

// GetNodeLog returns logs for a particular systemd service on a given node.
// We're count on these logs to fit into some reasonable memory size.
func GetNodeLog(ctx context.Context, client kubernetes.Interface, nodeName, systemdServiceName string) ([]byte, error) {
	path := client.CoreV1().RESTClient().Get().
		Namespace("").Name(nodeName).
		Resource("nodes").SubResource("proxy", "logs").Suffix("journal").URL().Path

	req := client.CoreV1().RESTClient().Get().RequestURI(path).
		SetHeader("Accept", "text/plain, */*")
		//SetHeader("Accept-Encoding", "gzip")
	req.Param("since", "-7d")
	req.Param("unit", systemdServiceName)

	fmt.Printf("path: %v\n", path)

	//decompressedReader, decompressedWriter := io.Pipe()
	//defer decompressedWriter.Close()
	//defer decompressedReader.Close()

	in, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	//err = optionallyDecompress(decompressedWriter, in)
	//if err != nil {
	//	return nil, err
	//}

	return ioutil.ReadAll(in)
}

func optionallyDecompress(out io.Writer, in io.Reader) error {
	bufferSize := 4096
	buf := bufio.NewReaderSize(in, bufferSize)
	head, err := buf.Peek(1024)
	if err != nil && err != io.EOF {
		return err
	}
	if _, err := gzip.NewReader(bytes.NewBuffer(head)); err != nil {
		// not a gzipped stream
		_, err = io.Copy(out, buf)
		return err
	}
	r, err := gzip.NewReader(buf)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	return err
}
