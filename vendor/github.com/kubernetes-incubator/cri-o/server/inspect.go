package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	cimage "github.com/containers/image/types"
	"github.com/go-zoo/bone"
	"github.com/kubernetes-incubator/cri-o/lib/sandbox"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/kubernetes-incubator/cri-o/types"
	"github.com/sirupsen/logrus"
)

func (s *Server) getInfo() types.CrioInfo {
	return types.CrioInfo{
		StorageDriver: s.config.Config.Storage,
		StorageRoot:   s.config.Config.Root,
		CgroupDriver:  s.config.Config.CgroupManager,
	}
}

var (
	errCtrNotFound     = errors.New("container not found")
	errCtrStateNil     = errors.New("container state is nil")
	errSandboxNotFound = errors.New("sandbox for container not found")
)

func (s *Server) getContainerInfo(id string, getContainerFunc func(id string) *oci.Container, getInfraContainerFunc func(id string) *oci.Container, getSandboxFunc func(id string) *sandbox.Sandbox) (types.ContainerInfo, error) {
	ctr := getContainerFunc(id)
	if ctr == nil {
		ctr = getInfraContainerFunc(id)
		if ctr == nil {
			return types.ContainerInfo{}, errCtrNotFound
		}
	}
	// TODO(mrunalp): should we call UpdateStatus()?
	ctrState := ctr.State()
	if ctrState == nil {
		return types.ContainerInfo{}, errCtrStateNil
	}
	sb := getSandboxFunc(ctr.Sandbox())
	if sb == nil {
		logrus.Debugf("can't find sandbox %s for container %s", ctr.Sandbox(), id)
		return types.ContainerInfo{}, errSandboxNotFound
	}
	image := ctr.Image()
	if s.ContainerServer != nil && s.ContainerServer.StorageImageServer() != nil {
		if status, err := s.ContainerServer.StorageImageServer().ImageStatus(&cimage.SystemContext{}, ctr.ImageRef()); err == nil {
			image = status.Name
		}
	}
	return types.ContainerInfo{
		Name:            ctr.Name(),
		Pid:             ctrState.Pid,
		Image:           image,
		ImageRef:        ctr.ImageRef(),
		CreatedTime:     ctrState.Created.UnixNano(),
		Labels:          ctr.Labels(),
		Annotations:     ctr.Annotations(),
		CrioAnnotations: ctr.CrioAnnotations(),
		Root:            ctr.MountPoint(),
		LogPath:         ctr.LogPath(),
		Sandbox:         ctr.Sandbox(),
		IP:              sb.IP(),
	}, nil

}

// GetInfoMux returns the mux used to serve info requests
func (s *Server) GetInfoMux() *bone.Mux {
	mux := bone.New()

	mux.Get("/info", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ci := s.getInfo()
		js, err := json.Marshal(ci)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}))

	mux.Get("/containers/:id", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		containerID := bone.GetValue(req, "id")
		ci, err := s.getContainerInfo(containerID, s.GetContainer, s.getInfraContainer, s.getSandbox)
		if err != nil {
			switch err {
			case errCtrNotFound:
				http.Error(w, fmt.Sprintf("can't find the container with id %s", containerID), http.StatusNotFound)
			case errCtrStateNil:
				http.Error(w, fmt.Sprintf("can't find container state for container with id %s", containerID), http.StatusInternalServerError)
			case errSandboxNotFound:
				http.Error(w, fmt.Sprintf("can't find the sandbox for container id %s", containerID), http.StatusNotFound)
			default:
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		js, err := json.Marshal(ci)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}))

	return mux
}
