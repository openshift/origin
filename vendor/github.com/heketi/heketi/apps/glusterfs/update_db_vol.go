//
// Copyright (c) 2019 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"

	"github.com/boltdb/bolt"

	"github.com/heketi/heketi/executors"
	"github.com/heketi/heketi/pkg/db"
)

func UpdateDbVol(app *App, volName string) error {
	if volName == "" {
		return fmt.Errorf("Volume name missing")
	}

	// we need a volume to validate the name we were given
	// and so we know what hosts to contact
	var vol *VolumeEntry
	err := app.db.View(func(tx *bolt.Tx) error {
		vl, err := VolumeList(tx)
		if err != nil {
			return err
		}
		for _, vid := range vl {
			vol, err = NewVolumeEntryFromId(tx, vid)
			if err != nil {
				return err
			}
			if vol.Info.Name == volName {
				return nil
			}
		}
		return fmt.Errorf("Volume %+v not found", volName)
	})
	if err != nil {
		return err
	}

	// query the current volume on gluster to get a dump of the
	// current options
	var vinfo *executors.Volume
	hosts, err := vol.hosts(app.db)
	if err != nil {
		return err
	}
	err = newTryOnHosts(hosts).once().run(func(h string) error {
		var err error
		vinfo, err = app.executor.VolumeInfo(h, volName)
		return err
	})
	if err != nil {
		return err
	}

	// detect the current level or assume "0" if not present
	var level = "0"
	for _, o := range vinfo.Options.OptionList {
		if o.Name == "user.heketi.dbstoragelevel" {
			level = o.Value
			logger.Info("Found db storage level: %v", level)
		}
	}

	if level == "0" {
		// level is zero. we need to update volume options
		req := &executors.VolumeModifyRequest{
			Name:                 volName,
			Stopped:              false,
			GlusterVolumeOptions: db.DbVolumeGlusterOptions,
		}

		err = newTryOnHosts(hosts).once().run(func(h string) error {
			return app.executor.VolumeModify(h, req)
		})
		if err != nil {
			return err
		}
	}

	return nil
}
