package scheduler

import (
	"github.com/mesos/mesos-go/api/v0/auth/callback"
	mesos "github.com/mesos/mesos-go/api/v0/mesosproto"
	"github.com/mesos/mesos-go/api/v0/upid"
)

type CredentialHandler struct {
	pid        *upid.UPID // the process to authenticate against (master)
	client     *upid.UPID // the process to be authenticated (slave / framework)
	credential *mesos.Credential
}

func (h *CredentialHandler) Handle(callbacks ...callback.Interface) error {
	for _, cb := range callbacks {
		switch cb := cb.(type) {
		case *callback.Name:
			cb.Set(h.credential.GetPrincipal())
		case *callback.Password:
			cb.Set(([]byte)(h.credential.GetSecret()))
		case *callback.Interprocess:
			cb.Set(*(h.pid), *(h.client))
		default:
			return &callback.Unsupported{Callback: cb}
		}
	}
	return nil
}
