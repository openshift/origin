package commatrix

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientutil "github.com/openshift-kni/commatrix/client"
	"github.com/openshift-kni/commatrix/consts"
	"github.com/openshift-kni/commatrix/debug"
	"github.com/openshift-kni/commatrix/ss"
	"github.com/openshift-kni/commatrix/types"
)

func GenerateSS(kubeconfig, customEntriesPath, customEntriesFormat, format string, env Env, deployment Deployment, destDir string) (ssMat *types.ComMatrix, ssOutTCP, ssOutUDP []byte, err error) {
	cs, err := clientutil.New(kubeconfig)
	if err != nil {
		return nil, nil, nil, err
	}

	nodesList, err := cs.CoreV1Interface.Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	nodesComDetails := []types.ComDetails{}
	err = debug.CreateNamespace(cs, consts.DefaultDebugNamespace)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() {
		err := debug.DeleteNamespace(cs, consts.DefaultDebugNamespace)
		if err != nil {
			fmt.Printf("failed to delete debug namespace: %v", err)
		}
	}()

	nLock := &sync.Mutex{}
	g := new(errgroup.Group)
	for _, n := range nodesList.Items {
		node := n
		g.Go(func() error {
			debugPod, err := debug.New(cs, node.Name, consts.DefaultDebugNamespace, consts.DefaultDebugPodImage)
			if err != nil {
				return err
			}
			defer func() {
				err := debugPod.Clean()
				if err != nil {
					fmt.Printf("failed cleaning debug pod %s: %v", debugPod, err)
				}
			}()

			cds, ssTCP, ssUDP, err := ss.CreateSSOutputFromNode(debugPod, &node)
			if err != nil {
				return err
			}
			nLock.Lock()
			ssTCPLine := fmt.Sprintf("node: %s\n%s\n", node.Name, string(ssTCP))
			ssUDPLine := fmt.Sprintf("node: %s\n%s\n", node.Name, string(ssUDP))

			nodesComDetails = append(nodesComDetails, cds...)
			ssOutTCP = append(ssOutTCP, []byte(ssTCPLine)...)
			ssOutUDP = append(ssOutUDP, []byte(ssUDPLine)...)
			nLock.Unlock()
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, nil, nil, err
	}

	cleanedComDetails := types.CleanComDetails(nodesComDetails)
	ssComMat := types.ComMatrix{Matrix: cleanedComDetails}
	return &ssComMat, ssOutTCP, ssOutUDP, nil
}

func getPrintFunction(format string) (func(m types.ComMatrix) ([]byte, error), error) {
	switch format {
	case "json":
		return types.ToJSON, nil
	case "csv":
		return types.ToCSV, nil
	case "yaml":
		return types.ToYAML, nil
	case "nft":
		return types.ToNFTables, nil
	default:
		return nil, fmt.Errorf("invalid format: %s. Please specify json, csv, yaml, or nft", format)
	}
}

func WriteSSRawFiles(destDir string, ssOutTCP, ssOutUDP []byte) error {
	tcpFile, err := os.OpenFile(path.Join(destDir, "raw-ss-tcp"), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	udpFile, err := os.OpenFile(path.Join(destDir, "raw-ss-udp"), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		tcpFile.Close()
		return err
	}

	defer tcpFile.Close()
	defer udpFile.Close()

	_, err = tcpFile.Write([]byte(string(ssOutTCP)))
	if err != nil {
		return fmt.Errorf("failed writing to file: %s", err)
	}

	_, err = udpFile.Write([]byte(string(ssOutUDP)))
	if err != nil {
		return fmt.Errorf("failed writing to file: %s", err)
	}

	return nil
}

func WriteMatrixToFileByType(mat types.ComMatrix, fileNamePrefix, format string, deployment Deployment, destDir string) error {
	printFn, err := getPrintFunction(format)
	if err != nil {
		return err
	}

	if format == types.FormatNFT {
		masterMatrix, workerMatrix := separateMatrixByRole(mat)
		err := writeMatrixToFile(masterMatrix, fileNamePrefix+"-master", format, printFn, destDir)
		if err != nil {
			return err
		}
		if deployment == MNO {
			err := writeMatrixToFile(workerMatrix, fileNamePrefix+"-worker", format, printFn, destDir)
			if err != nil {
				return err
			}
		}
	} else {
		err := writeMatrixToFile(mat, fileNamePrefix, format, printFn, destDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeMatrixToFile(matrix types.ComMatrix, fileName, format string, printFn func(m types.ComMatrix) ([]byte, error), destDir string) error {
	res, err := printFn(matrix)
	if err != nil {
		return err
	}

	comMatrixFileName := filepath.Join(destDir, fmt.Sprintf("%s.%s", fileName, format))
	return os.WriteFile(comMatrixFileName, res, 0644)
}

func GenerateMatrixDiff(mat1 types.ComMatrix, mat2 types.ComMatrix) string {
	diff := consts.CSVHeaders + "\n"
	for _, cd := range mat1.Matrix {
		if mat2.Contains(cd) {
			diff += fmt.Sprintf("%s\n", cd)
			continue
		}

		diff += fmt.Sprintf("+ %s\n", cd)
	}

	for _, cd := range mat2.Matrix {
		// Skip "rpc.statd" ports, these are randomly open ports on the node,
		// no need to mention them in the matrix diff
		if cd.Service == "rpc.statd" {
			continue
		}

		if !mat1.Contains(cd) {
			diff += fmt.Sprintf("- %s\n", cd)
		}
	}

	return diff
}

func separateMatrixByRole(matrix types.ComMatrix) (types.ComMatrix, types.ComMatrix) {
	var masterMatrix, workerMatrix types.ComMatrix
	for _, entry := range matrix.Matrix {
		if entry.NodeRole == "master" {
			masterMatrix.Matrix = append(masterMatrix.Matrix, entry)
		} else if entry.NodeRole == "worker" {
			workerMatrix.Matrix = append(workerMatrix.Matrix, entry)
		}
	}

	return masterMatrix, workerMatrix
}
