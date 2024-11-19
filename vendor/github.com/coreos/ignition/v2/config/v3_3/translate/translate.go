// Copyright 2020 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translate

import (
	"github.com/coreos/ignition/v2/config/translate"
	"github.com/coreos/ignition/v2/config/util"
	old_types "github.com/coreos/ignition/v2/config/v3_2/types"
	"github.com/coreos/ignition/v2/config/v3_3/types"
)

func translateIgnition(old old_types.Ignition) (ret types.Ignition) {
	// use a new translator so we don't recurse infinitely
	translate.NewTranslator().Translate(&old, &ret)
	ret.Version = types.MaxVersion.String()
	return
}

func translateRaid(old old_types.Raid) (ret types.Raid) {
	tr := translate.NewTranslator()
	tr.Translate(&old.Devices, &ret.Devices)
	ret.Level = util.StrToPtr(old.Level)
	tr.Translate(&old.Name, &ret.Name)
	tr.Translate(&old.Options, &ret.Options)
	tr.Translate(&old.Spares, &ret.Spares)
	return
}

func translateLuks(old old_types.Luks) (ret types.Luks) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateClevis)
	if old.Clevis != nil {
		tr.Translate(old.Clevis, &ret.Clevis)
	}
	tr.Translate(&old.Device, &ret.Device)
	tr.Translate(&old.KeyFile, &ret.KeyFile)
	tr.Translate(&old.Label, &ret.Label)
	tr.Translate(&old.Name, &ret.Name)
	tr.Translate(&old.Options, &ret.Options)
	tr.Translate(&old.UUID, &ret.UUID)
	tr.Translate(&old.WipeVolume, &ret.WipeVolume)
	return
}

func translateClevis(old old_types.Clevis) (ret types.Clevis) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateClevisCustom)
	if old.Custom != nil {
		tr.Translate(old.Custom, &ret.Custom)
	}
	tr.Translate(&old.Tang, &ret.Tang)
	tr.Translate(&old.Threshold, &ret.Threshold)
	tr.Translate(&old.Tpm2, &ret.Tpm2)
	return
}

func translateClevisCustom(old old_types.Custom) (ret types.ClevisCustom) {
	tr := translate.NewTranslator()
	ret.Config = util.StrToPtr(old.Config)
	tr.Translate(&old.NeedsNetwork, &ret.NeedsNetwork)
	ret.Pin = util.StrToPtr(old.Pin)
	return
}

func translateLinkEmbedded1(old old_types.LinkEmbedded1) (ret types.LinkEmbedded1) {
	tr := translate.NewTranslator()
	tr.Translate(&old.Hard, &ret.Hard)
	ret.Target = util.StrToPtr(old.Target)
	return
}

func Translate(old old_types.Config) (ret types.Config) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateIgnition)
	tr.AddCustomTranslator(translateRaid)
	tr.AddCustomTranslator(translateLuks)
	tr.AddCustomTranslator(translateLinkEmbedded1)
	tr.Translate(&old.Ignition, &ret.Ignition)
	tr.Translate(&old.Passwd, &ret.Passwd)
	tr.Translate(&old.Storage, &ret.Storage)
	tr.Translate(&old.Systemd, &ret.Systemd)
	return
}
