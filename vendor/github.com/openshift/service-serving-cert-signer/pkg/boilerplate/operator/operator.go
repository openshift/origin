package operator

import "github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controller"

type Runner interface {
	Run(stopCh <-chan struct{})
}

func New(name string, sync KeySyncer, opts ...Option) Runner {
	o := &operator{
		name: name,
		sync: &wrapper{KeySyncer: sync},
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

type operator struct {
	name string
	sync controller.KeySyncer

	opts []controller.Option
}

func (o *operator) Run(stopCh <-chan struct{}) {
	runner := controller.New(o.name, o.sync, o.opts...)
	// only start one worker because we only have one key in our queue (see WithInformer)
	// since this operator works on a singleton, it does not make sense to ever run more than one worker
	runner.Run(1, stopCh)
}
