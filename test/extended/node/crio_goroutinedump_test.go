package node

import (
	"testing"
)

func TestFindStuckImagePulls(t *testing.T) {
	stuckGoroutine := `goroutine 64418 [IO wait, 189 minutes]:
internal/poll.runtime_pollWait(0x7ff92406ec00, 0x72)
	/usr/lib/golang/src/runtime/netpoll.go:351 +0x85
internal/poll.(*pollDesc).wait(0xc0006db180?, 0xc003d54000?, 0x0)
	/usr/lib/golang/src/internal/poll/fd_poll_runtime.go:84 +0x27
internal/poll.(*pollDesc).waitRead(...)
	/usr/lib/golang/src/internal/poll/fd_poll_runtime.go:89
internal/poll.(*FD).Read(0xc0006db180, {0xc003d54000, 0xa000, 0xa000})
	/usr/lib/golang/src/internal/poll/fd_unix.go:165 +0x279
net.(*netFD).Read(0xc0006db180, {0xc003d54000?, 0xc003d58f6a?, 0x5?})
	/usr/lib/golang/src/net/fd_posix.go:68 +0x25
net.(*conn).Read(0xc0021faec0, {0xc003d54000?, 0x7ff924940130?, 0x7ff97c78f5c0?})
	/usr/lib/golang/src/net/net.go:196 +0x45
crypto/tls.(*atLeastReader).Read(0xc003835698, {0xc003d54000?, 0x5091?, 0x55a992a076bc?})
	/usr/lib/golang/src/crypto/tls/conn.go:819 +0x3b
bytes.(*Buffer).ReadFrom(0xc001cfe628, {0x55a994d32820, 0xc003835698})
	/usr/lib/golang/src/bytes/buffer.go:217 +0x98
crypto/tls.(*Conn).readFromUntil(0xc001cfe388, {0x55a994d32240, 0xc0021faec0}, 0xc0028e5228?)
	/usr/lib/golang/src/crypto/tls/conn.go:841 +0xde
crypto/tls.(*Conn).readRecordOrCCS(0xc001cfe388, 0x0)
	/usr/lib/golang/src/crypto/tls/conn.go:630 +0x3db
crypto/tls.(*Conn).readRecord(...)
	/usr/lib/golang/src/crypto/tls/conn.go:592
crypto/tls.(*Conn).Read(0xc001cfe388, {0xc003bc8000, 0x8000, 0x7ff97c78f108?})
	/usr/lib/golang/src/crypto/tls/conn.go:1397 +0x145
net/http.(*persistConn).Read(0xc002974ea0, {0xc003bc8000?, 0x55a99287cdc5?, 0x0?})
	/usr/lib/golang/src/net/http/transport.go:2125 +0x47
bufio.(*Reader).Read(0xc002366ba0, {0xc003bc8000, 0x8000, 0xc0021ce210?})
	/usr/lib/golang/src/bufio/bufio.go:231 +0xe2
io.(*LimitedReader).Read(0xc003b05050, {0xc003bc8000?, 0x55a994d2f788?, 0x55a994d58100?})
	/usr/lib/golang/src/io/io.go:479 +0x43
net/http.(*body).readLocked(0xc004224ac0, {0xc003bc8000?, 0x55a99287c394?, 0xc0028e5628?})
	/usr/lib/golang/src/net/http/transfer.go:845 +0x3b
net/http.(*body).Read(0x55a9942d3237?, {0xc003bc8000?, 0x2?, 0x0?})
	/usr/lib/golang/src/net/http/transfer.go:837 +0xff
net/http.(*bodyEOFSignal).Read(0xc004224b00, {0xc003bc8000, 0x8000, 0x8000})
	/usr/lib/golang/src/net/http/transport.go:3000 +0x13e
github.com/cri-o/cri-o/vendor/go.podman.io/image/v5/docker.(*bodyReader).Read(0xc003f3a0a0, {0xc003bc8000?, 0x0?, 0x0?})
	/builddir/build/BUILD/cri-o-804ec103e65c2d3766fbd81b2d215fbc00350768/_output/src/github.com/cri-o/cri-o/vendor/go.podman.io/image/v5/docker/body_reader.go:143 +0x67
github.com/cri-o/cri-o/vendor/go.podman.io/image/v5/copy.(*digestingReader).Read(0xc0032e7b80, {0xc003bc8000, 0xc0028e5b00?, 0x8000})
	/builddir/build/BUILD/cri-o-804ec103e65c2d3766fbd81b2d215fbc00350768/_output/src/github.com/cri-o/cri-o/vendor/go.podman.io/image/v5/copy/digesting_reader.go:44 +0x3f
github.com/cri-o/cri-o/vendor/github.com/vbauerster/mpb/v8.ewmaProxyReader.Read({{0x55a994d3b728?, 0xc001c1d200?}, 0xc003262ff0?}, {0xc003bc8000, 0x8000, 0x8000})
	/builddir/build/BUILD/cri-o-804ec103e65c2d3766fbd81b2d215fbc00350768/_output/src/github.com/cri-o/cri-o/vendor/github.com/vbauerster/mpb/v8/proxyreader.go:36 +0x8d
io.(*multiReader).Read(0xc003b051a0, {0xc003bc8000, 0x8000, 0x8000})
	/usr/lib/golang/src/io/multi.go:26 +0x93
github.com/cri-o/cri-o/vendor/go.podman.io/image/v5/copy.errorAnnotationReader.Read({{0x55a994d301e0?, 0xc003b051a0?}}, {0xc003bc8000?, 0x8000?, 0x0?})
	/builddir/build/BUILD/cri-o-804ec103e65c2d3766fbd81b2d215fbc00350768/_output/src/github.com/cri-o/cri-o/vendor/go.podman.io/image/v5/copy/blob.go:165 +0x33`

	normalRunning := `goroutine 1 [running]:
main.main()
	/src/main.go:10 +0x25`

	ioWaitShort := `goroutine 100 [IO wait, 5 minutes]:
net.(*conn).Read(0xc000a1c010, {0xc002000000, 0x1000, 0x1000})
	net/net.go:179 +0x45
github.com/containers/image/v5/docker.(*bodyReader).Read(0xc000b1e5c0, {0xc002000000, 0x1000, 0x1000})
	github.com/containers/image/v5/docker/body_reader.go:52 +0x78`

	ioWaitNoBodyReader := `goroutine 200 [IO wait, 60 minutes]:
net.(*conn).Read(0xc000a1c010, {0xc002000000, 0x1000, 0x1000})
	net/net.go:179 +0x45
bufio.(*Reader).Read(0xc000b1e5c0, {0xc002000000, 0x1000, 0x1000})
	bufio/bufio.go:237 +0x78`

	ioWaitNoConnRead := `goroutine 300 [IO wait, 60 minutes]:
github.com/containers/image/v5/docker.(*bodyReader).Read(0xc000b1e5c0, {0xc002000000, 0x1000, 0x1000})
	github.com/containers/image/v5/docker/body_reader.go:52 +0x78`

	// v6 version of containers/image — should still match with version-agnostic pattern
	stuckV6 := `goroutine 500 [IO wait, 45 minutes]:
net.(*conn).Read(0xc000a1c010, {0xc002000000, 0x1000, 0x1000})
	net/net.go:179 +0x45
github.com/containers/image/v6/docker.(*bodyReader).Read(0xc000b1e5c0, {0xc002000000, 0x1000, 0x1000})
	github.com/containers/image/v6/docker/body_reader.go:52 +0x78`

	tests := []struct {
		name      string
		dump      string
		wantCount int
	}{
		{
			name:      "single stuck goroutine",
			dump:      stuckGoroutine,
			wantCount: 1,
		},
		{
			name:      "no stuck goroutines in running state",
			dump:      normalRunning,
			wantCount: 0,
		},
		{
			name:      "IO wait under 30 minutes is not stuck",
			dump:      ioWaitShort,
			wantCount: 0,
		},
		{
			name:      "IO wait without bodyReader is not stuck",
			dump:      ioWaitNoBodyReader,
			wantCount: 0,
		},
		{
			name:      "IO wait without conn.Read is not stuck",
			dump:      ioWaitNoConnRead,
			wantCount: 0,
		},
		{
			name:      "version-agnostic match works with v6",
			dump:      stuckV6,
			wantCount: 1,
		},
		{
			name:      "mixed dump returns only stuck goroutines",
			dump:      stuckGoroutine + "\n\n" + normalRunning + "\n\n" + ioWaitShort + "\n\n" + stuckV6,
			wantCount: 2,
		},
		{
			name:      "empty dump",
			dump:      "",
			wantCount: 0,
		},
		{
			name:      "exactly 30 minutes is not stuck",
			dump:      "goroutine 400 [IO wait, 30 minutes]:\nnet.(*conn).Read(0xc0, {0xc0, 0x1})\n\tnet/net.go:179 +0x45\ngithub.com/containers/image/v5/docker.(*bodyReader).Read(0xc0, {0xc0, 0x1})\n\tbody_reader.go:52 +0x78",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findStuckImagePulls(tt.dump)
			if len(got) != tt.wantCount {
				t.Errorf("findStuckImagePulls() returned %d results, want %d.\nGot: %v", len(got), tt.wantCount, got)
			}
		})
	}
}
