// +build windows

package openshift

import (
	"fmt"
)

func CheckSocat() error {
	return fmt.Errorf("socat tunnel not supported on windows")
}

func KillExistingSocat() error {
	return nil
}

func SaveSocatPid(pid int) error {
	return nil
}

func (h *Helper) StartSocatTunnel(string) error {
	return fmt.Errorf("socat tunnel not supported on windows")
}
