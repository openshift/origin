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
	old_types "github.com/coreos/ignition/v2/config/v3_3/types"
	"github.com/coreos/ignition/v2/config/v3_4/types"
)

func translateIgnition(old old_types.Ignition) (ret types.Ignition) {
	// use a new translator so we don't recurse infinitely
	translate.NewTranslator().Translate(&old, &ret)
	ret.Version = types.MaxVersion.String()
	return
}

func translateFileEmbedded1(old old_types.FileEmbedded1) (ret types.FileEmbedded1) {
	tr := translate.NewTranslator()
	tr.Translate(&old.Append, &ret.Append)
	tr.Translate(&old.Contents, &ret.Contents)
	if old.Mode != nil {
		// We support the special mode bits for specs >=3.4.0, so if
		// the user provides special mode bits in an Ignition config
		// with the version < 3.4.0, then we need to explicitly mask
		// those bits out during translation.
		ret.Mode = util.IntToPtr(*old.Mode & ^07000)
	}
	return
}

func translateDirectoryEmbedded1(old old_types.DirectoryEmbedded1) (ret types.DirectoryEmbedded1) {
	if old.Mode != nil {
		// We support the special mode bits for specs >=3.4.0, so if
		// the user provides special mode bits in an Ignition config
		// with the version < 3.4.0, then we need to explicitly mask
		// those bits out during translation.
		ret.Mode = util.IntToPtr(*old.Mode & ^07000)
	}
	return
}

func translateLuks(old old_types.Luks) (ret types.Luks) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateTang)
	tr.Translate(&old.Clevis, &ret.Clevis)
	tr.Translate(&old.Device, &ret.Device)
	tr.Translate(&old.KeyFile, &ret.KeyFile)
	tr.Translate(&old.Label, &ret.Label)
	tr.Translate(&old.Name, &ret.Name)
	tr.Translate(&old.Options, &ret.Options)
	tr.Translate(&old.UUID, &ret.UUID)
	tr.Translate(&old.WipeVolume, &ret.WipeVolume)
	return
}

func translateTang(old old_types.Tang) (ret types.Tang) {
	tr := translate.NewTranslator()
	tr.Translate(&old.Thumbprint, &ret.Thumbprint)
	tr.Translate(&old.URL, &ret.URL)
	return
}

func Translate(old old_types.Config) (ret types.Config) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateIgnition)
	tr.AddCustomTranslator(translateDirectoryEmbedded1)
	tr.AddCustomTranslator(translateFileEmbedded1)
	tr.AddCustomTranslator(translateLuks)
	tr.Translate(&old, &ret)
	return
}
