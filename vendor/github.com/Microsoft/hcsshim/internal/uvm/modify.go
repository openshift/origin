package uvm

import (
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/sirupsen/logrus"
)

// Modify modifies the compute system by sending a request to HCS.
func (uvm *UtilityVM) Modify(hcsModificationDocument interface{}) (err error) {
	op := "uvm::Modify"
	log := logrus.WithFields(logrus.Fields{
		logfields.UVMID: uvm.id,
	})
	log.Debug(op + " - Begin Operation")
	defer func() {
		if err != nil {
			log.Data[logrus.ErrorKey] = err
			log.Error(op + " - End Operation - Error")
		} else {
			log.Debug(op + " - End Operation - Success")
		}
	}()

	return uvm.hcsSystem.Modify(hcsModificationDocument)
}
