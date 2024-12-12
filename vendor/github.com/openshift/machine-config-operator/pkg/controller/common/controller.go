package common

// Controller is the common interface all controllers implement
type Controller interface {
	Run(workers int, stopCh <-chan struct{})
}
