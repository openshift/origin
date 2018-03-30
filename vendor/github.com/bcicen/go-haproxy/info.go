package haproxy

import (
	"fmt"

	"github.com/bcicen/go-haproxy/kvcodec"
)

// Response from HAProxy "show info" command.
type Info struct {
	Name                       string `kv:"Name"`
	Version                    string `kv:"Version"`
	ReleaseDate                string `kv:"Release_date"`
	Nbproc                     uint64 `kv:"Nbproc"`
	ProcessNum                 uint64 `kv:"Process_num"`
	Pid                        uint64 `kv:"Pid"`
	Uptime                     string `kv:"Uptime"`
	UptimeSec                  uint64 `kv:"Uptime_sec"`
	MemMaxMB                   uint64 `kv:"Memmax_MB"`
	UlimitN                    uint64 `kv:"Ulimit-n"`
	Maxsock                    uint64 `kv:"Maxsock"`
	Maxconn                    uint64 `kv:"Maxconn"`
	HardMaxconn                uint64 `kv:"Hard_maxconn"`
	CurrConns                  uint64 `kv:"CurrConns"`
	CumConns                   uint64 `kv:"CumConns"`
	CumReq                     uint64 `kv:"CumReq"`
	MaxSslConns                uint64 `kv:"MaxSslConns"`
	CurrSslConns               uint64 `kv:"CurrSslConns"`
	CumSslConns                uint64 `kv:"CumSslConns"`
	Maxpipes                   uint64 `kv:"Maxpipes"`
	PipesUsed                  uint64 `kv:"PipesUsed"`
	PipesFree                  uint64 `kv:"PipesFree"`
	ConnRate                   uint64 `kv:"ConnRate"`
	ConnRateLimit              uint64 `kv:"ConnRateLimit"`
	MaxConnRate                uint64 `kv:"MaxConnRate"`
	SessRate                   uint64 `kv:"SessRate"`
	SessRateLimit              uint64 `kv:"SessRateLimit"`
	MaxSessRate                uint64 `kv:"MaxSessRate"`
	SslRate                    uint64 `kv:"SslRate"`
	SslRateLimit               uint64 `kv:"SslRateLimit"`
	MaxSslRate                 uint64 `kv:"MaxSslRate"`
	SslFrontendKeyRate         uint64 `kv:"SslFrontendKeyRate"`
	SslFrontendMaxKeyRate      uint64 `kv:"SslFrontendMaxKeyRate"`
	SslFrontendSessionReusePct uint64 `kv:"SslFrontendSessionReuse_pct"`
	SslBackendKeyRate          uint64 `kv:"SslBackendKeyRate"`
	SslBackendMaxKeyRate       uint64 `kv:"SslBackendMaxKeyRate"`
	SslCacheLookups            uint64 `kv:"SslCacheLookups"`
	SslCacheMisses             uint64 `kv:"SslCacheMisses"`
	CompressBpsIn              uint64 `kv:"CompressBpsIn"`
	CompressBpsOut             uint64 `kv:"CompressBpsOut"`
	CompressBpsRateLim         uint64 `kv:"CompressBpsRateLim"`
	ZlibMemUsage               uint64 `kv:"ZlibMemUsage"`
	MaxZlibMemUsage            uint64 `kv:"MaxZlibMemUsage"`
	Tasks                      uint64 `kv:"Tasks"`
	RunQueue                   uint64 `kv:"Run_queue"`
	IdlePct                    uint64 `kv:"Idle_pct"`
	Node                       string `kv:"node"`
	Description                string `kv:"description"`
}

// Equivalent to HAProxy "show info" command.
func (h *HAProxyClient) Info() (*Info, error) {
	res, err := h.RunCommand("show info")
	if err != nil {
		return nil, err
	}
	info := &Info{}
	err = kvcodec.Unmarshal(res, info)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %s", err)
	}
	return info, nil
}
