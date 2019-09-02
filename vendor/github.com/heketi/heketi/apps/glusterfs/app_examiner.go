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
	"net/http"
)

// ExamineGluster ... Compares the state of heketi db with the state of Gluster
// This is the variant to be called via the API and running in the App
func (a *App) ExamineGluster(w http.ResponseWriter, r *http.Request) {

	response, err := a.OnDemandExaminer().ExamineGluster()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(response); err != nil {
		panic(err)
	}
}
