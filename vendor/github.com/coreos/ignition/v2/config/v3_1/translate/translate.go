// Copyright 2019 Red Hat, Inc.
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
	old_types "github.com/coreos/ignition/v2/config/v3_0/types"
	"github.com/coreos/ignition/v2/config/v3_1/types"
)

func translateFilesystem(old old_types.Filesystem) (ret types.Filesystem) {
	// use a new translator so we don't recurse infinitely
	tr := translate.NewTranslator()
	tr.Translate(&old.Device, &ret.Device)
	tr.Translate(&old.Format, &ret.Format)
	tr.Translate(&old.Label, &ret.Label)
	tr.Translate(&old.Options, &ret.Options)
	tr.Translate(&old.Path, &ret.Path)
	tr.Translate(&old.UUID, &ret.UUID)
	tr.Translate(&old.WipeFilesystem, &ret.WipeFilesystem)
	return
}

func translateConfigReference(old old_types.ConfigReference) (ret types.Resource) {
	// use a new translator so we don't recurse infinitely
	tr := translate.NewTranslator()
	tr.Translate(&old.Source, &ret.Source)
	tr.Translate(&old.Verification, &ret.Verification)
	return
}

func translateCAReference(old old_types.CaReference) (ret types.Resource) {
	// use a new translator so we don't recurse infinitely
	tr := translate.NewTranslator()
	ret.Source = util.StrToPtr(old.Source)
	tr.Translate(&old.Verification, &ret.Verification)
	return
}

func translateFileContents(old old_types.FileContents) (ret types.Resource) {
	// use a new translator so we don't recurse infinitely
	tr := translate.NewTranslator()
	tr.Translate(&old.Compression, &ret.Compression)
	tr.Translate(&old.Source, &ret.Source)
	tr.Translate(&old.Verification, &ret.Verification)
	return
}

func translateIgnitionConfig(old old_types.IgnitionConfig) (ret types.IgnitionConfig) {
	// use a new translator so we don't recurse infinitely
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateConfigReference)
	tr.Translate(&old.Merge, &ret.Merge)
	tr.Translate(&old.Replace, &ret.Replace)
	return
}

func translateSecurity(old old_types.Security) (ret types.Security) {
	// use a new translator so we don't recurse infinitely
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateTLS)
	tr.Translate(&old.TLS, &ret.TLS)
	return
}

func translateTLS(old old_types.TLS) (ret types.TLS) {
	// use a new translator so we don't recurse infinitely
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateCAReference)
	tr.Translate(&old.CertificateAuthorities, &ret.CertificateAuthorities)
	return
}

func translateIgnition(old old_types.Ignition) (ret types.Ignition) {
	// use a new translator so we don't recurse infinitely
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateIgnitionConfig)
	tr.AddCustomTranslator(translateSecurity)
	tr.Translate(&old.Config, &ret.Config)
	tr.Translate(&old.Security, &ret.Security)
	tr.Translate(&old.Timeouts, &ret.Timeouts)
	ret.Version = types.MaxVersion.String()
	return
}

func Translate(old old_types.Config) (ret types.Config) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateFileContents)
	tr.AddCustomTranslator(translateIgnition)
	tr.AddCustomTranslator(translateFilesystem)
	tr.Translate(&old, &ret)
	return
}
