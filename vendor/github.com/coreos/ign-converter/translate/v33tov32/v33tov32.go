// Copyright 2021 Red Hat, Inc.
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

package v33tov32

import (
	"fmt"
	"reflect"

	"github.com/coreos/ignition/v2/config/translate"
	"github.com/coreos/ignition/v2/config/v3_2/types"
	old_types "github.com/coreos/ignition/v2/config/v3_3/types"
	"github.com/coreos/ignition/v2/config/validate"
)

// Mostly a copy of github.com/coreos/ignition/v2/config/v3_3/translate/translate.go
// with the types & old_types imports reversed (the referenced file translates
// from 3.2 -> 3.3 but as a result only touches fields that are understood by
// the 3.2 spec). With additional logic to account for translation from a non-pointer
// field to a pointer field (e.g. ClevisCustom and Clevis), and the translation
// from a pointer to a non-pointer field (e.g. Link.Target, Raid.Level).
func translateIgnition(old old_types.Ignition) (ret types.Ignition) {
	// use a new translator so we don't recurse infinitely
	translate.NewTranslator().Translate(&old, &ret)
	ret.Version = types.MaxVersion.String()
	return
}

func translateRaid(old old_types.Raid) (ret types.Raid) {
	tr := translate.NewTranslator()
	tr.Translate(&old.Devices, &ret.Devices)
	tr.Translate(old.Level, &ret.Level)
	tr.Translate(&old.Name, &ret.Name)
	tr.Translate(&old.Options, &ret.Options)
	tr.Translate(&old.Spares, &ret.Spares)
	return
}

func translateLuks(old old_types.Luks) (ret types.Luks) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateClevis)
	// this goes from "not pointer" in 3.3 to "pointer" in 3.2 so we need to
	// populate it if old.Clevis isn't empty
	if !reflect.DeepEqual(old.Clevis, old_types.Clevis{}) {
		ret.Clevis = &types.Clevis{}
		tr.Translate(&old.Clevis, ret.Clevis)
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
	// this goes from "not pointer" in 3.3 to "pointer" in 3.2 so we need to
	// populate it if old.Custom isn't empty
	if !reflect.DeepEqual(old.Custom, old_types.ClevisCustom{}) {
		ret.Custom = &types.Custom{}
		tr.Translate(&old.Custom, ret.Custom)
	}
	tr.Translate(&old.Tang, &ret.Tang)
	tr.Translate(&old.Threshold, &ret.Threshold)
	tr.Translate(&old.Tpm2, &ret.Tpm2)
	return
}

func translateClevisCustom(old old_types.ClevisCustom) (ret types.Custom) {
	tr := translate.NewTranslator()
	tr.Translate(old.Config, &ret.Config)
	tr.Translate(&old.NeedsNetwork, &ret.NeedsNetwork)
	tr.Translate(old.Pin, &ret.Pin)
	return
}

func translateLinkEmbedded1(old old_types.LinkEmbedded1) (ret types.LinkEmbedded1) {
	tr := translate.NewTranslator()
	tr.Translate(&old.Hard, &ret.Hard)
	tr.Translate(old.Target, &ret.Target)
	return
}

func translateConfig(old old_types.Config) (ret types.Config) {
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

// end copied Ignition v3_3/translate block

// Translate translates Ignition spec config v3.3 to spec v3.2
func Translate(cfg old_types.Config) (types.Config, error) {
	rpt := validate.ValidateWithContext(cfg, nil)
	if rpt.IsFatal() {
		return types.Config{}, fmt.Errorf("Invalid input config:\n%s", rpt.String())
	}

	if len(cfg.KernelArguments.ShouldExist) > 0 || len(cfg.KernelArguments.ShouldNotExist) > 0 {
		return types.Config{}, fmt.Errorf("KernelArguments is not supported on 3.2")
	}

	res := translateConfig(cfg)

	// Sanity check the returned config
	oldrpt := validate.ValidateWithContext(res, nil)
	if oldrpt.IsFatal() {
		return types.Config{}, fmt.Errorf("Converted spec has unexpected fatal error:\n%s", oldrpt.String())
	}
	return res, nil
}
