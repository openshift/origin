package phaul

import (
	"fmt"
	"os"

	"github.com/checkpoint-restore/go-criu"
	"github.com/checkpoint-restore/go-criu/rpc"
	"github.com/golang/protobuf/proto"
	"path/filepath"
)

// Server struct
type Server struct {
	cfg     Config
	imgs    *images
	cr      *criu.Criu
	process *os.Process
}

// MakePhaulServer function
// Main entry point. Make the server with comm and call PhaulRemote
// methods on it upon client requests.
func MakePhaulServer(c Config) (*Server, error) {
	img, err := preparePhaulImages(c.Wdir)
	if err != nil {
		return nil, err
	}

	cr := criu.MakeCriu()

	return &Server{imgs: img, cfg: c, cr: cr}, nil
}

//
// StartIter phaul.Remote methods
func (s *Server) StartIter() error {
	fmt.Printf("S: start iter\n")
	psi := rpc.CriuPageServerInfo{
		Fd: proto.Int32(int32(s.cfg.Memfd)),
	}
	opts := rpc.CriuOpts{
		LogLevel: proto.Int32(4),
		LogFile:  proto.String("ps.log"),
		Ps:       &psi,
	}

	prevP := s.imgs.lastImagesDir()
	imgDir, err := s.imgs.openNextDir()
	if err != nil {
		return err
	}
	defer imgDir.Close()

	opts.ImagesDirFd = proto.Int32(int32(imgDir.Fd()))
	if prevP != "" {
		p, err := filepath.Abs(imgDir.Name())
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(p, prevP)
		if err != nil {
			return err
		}
		opts.ParentImg = proto.String(rel)
	}

	pid, _, err := s.cr.StartPageServerChld(opts)
	if err != nil {
		return err
	}

	s.process, err = os.FindProcess(pid)
	if err != nil {
		return err
	}

	return nil
}

// StopIter function
func (s *Server) StopIter() error {
	state, err := s.process.Wait()
	if err != nil {
		return nil
	}

	if !state.Success() {
		return fmt.Errorf("page-server failed: %s", s)
	}
	return nil
}

// Server-local methods

// LastImagesDir function
func (s *Server) LastImagesDir() string {
	return s.imgs.lastImagesDir()
}

// GetCriu function
func (s *Server) GetCriu() *criu.Criu {
	return s.cr
}
