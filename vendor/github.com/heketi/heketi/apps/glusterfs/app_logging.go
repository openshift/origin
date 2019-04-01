//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/logging"
	"github.com/heketi/heketi/pkg/utils"
)

func (a *App) logLevelName() string {
	switch logger.Level() {
	case logging.LEVEL_NOLOG:
		return "none"
	case logging.LEVEL_CRITICAL:
		return "critical"
	case logging.LEVEL_ERROR:
		return "error"
	case logging.LEVEL_WARNING:
		return "warning"
	case logging.LEVEL_INFO:
		return "info"
	case logging.LEVEL_DEBUG:
		return "debug"
	default:
		return "(unknown)"
	}
}

func (a *App) GetLogLevel(w http.ResponseWriter, r *http.Request) {
	info := api.LogLevelInfo{LogLevel: map[string]string{}}
	info.LogLevel["glusterfs"] = a.logLevelName()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}
}

func (a *App) SetLogLevel(w http.ResponseWriter, r *http.Request) {
	msg := api.LogLevelInfo{}
	err := utils.GetJsonFromRequest(r, &msg)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("request unable to be parsed: %s", err.Error()),
			http.StatusBadRequest)
		return
	}
	wantLevel, ok := msg.LogLevel["glusterfs"]
	if !ok {
		err := fmt.Errorf("Only \"glusterfs\" logger may be modified")
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	err = SetLogLevel(wantLevel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	logger.Info("set new log level [%s]", msg.LogLevel)
	logger.Debug("debug logging enabled")

	a.GetLogLevel(w, r)
	return
}
