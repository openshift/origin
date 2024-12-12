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

package v34tov33

import (
	"fmt"
	"net/url"
	"reflect"

	"github.com/coreos/ignition/v2/config/translate"
	"github.com/coreos/ignition/v2/config/util"
	"github.com/coreos/ignition/v2/config/v3_3/types"
	old_types "github.com/coreos/ignition/v2/config/v3_4/types"
	"github.com/coreos/ignition/v2/config/validate"
)

// Copy of github.com/coreos/ignition/v2/config/v3_4/translate/translate.go
// with the types & old_types imports reversed (the referenced file translates
// from 3.3 -> 3.4 but as a result only touches fields that are understood by
// the 3.3 spec).
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

func translateConfig(old old_types.Config) (ret types.Config) {
	tr := translate.NewTranslator()
	tr.AddCustomTranslator(translateIgnition)
	tr.AddCustomTranslator(translateDirectoryEmbedded1)
	tr.AddCustomTranslator(translateFileEmbedded1)
	tr.AddCustomTranslator(translateLuks)
	tr.Translate(&old, &ret)
	return
}

// end copied Ignition v3_4/translate block

// Translate translates Ignition spec config v3.4 to spec v3.3
func Translate(cfg old_types.Config) (types.Config, error) {
	rpt := validate.ValidateWithContext(cfg, nil)
	if rpt.IsFatal() {
		return types.Config{}, fmt.Errorf("Invalid input config:\n%s", rpt.String())
	}

	err := checkValue(reflect.ValueOf(cfg))
	if err != nil {
		return types.Config{}, err
	}

	res := translateConfig(cfg)

	// Sanity check the returned config
	oldrpt := validate.ValidateWithContext(res, nil)
	if oldrpt.IsFatal() {
		return types.Config{}, fmt.Errorf("Converted spec has unexpected fatal error:\n%s", oldrpt.String())
	}
	return res, nil
}

func checkValue(v reflect.Value) error {
	switch v.Type() {
	case reflect.TypeOf(old_types.Tang{}):
		tang := v.Interface().(old_types.Tang)
		// 3.3 does not support tang offline provisioning
		if util.NotEmpty(tang.Advertisement) {
			return fmt.Errorf("Invalid input config: tang offline provisioning is not supported in spec v3.3")
		}
	case reflect.TypeOf(old_types.Luks{}):
		luks := v.Interface().(old_types.Luks)
		// 3.3 does not support luks discard
		if util.IsTrue(luks.Discard) {
			return fmt.Errorf("Invalid input config: luks discard is not supported in spec v3.3")
		}
		// 3.3 does not support luks openOptions
		if len(luks.OpenOptions) > 0 {
			return fmt.Errorf("Invalid input config: luks openOptions is not supported in spec v3.3")
		}
	case reflect.TypeOf(old_types.FileEmbedded1{}):
		f := v.Interface().(old_types.FileEmbedded1)
		// 3.3 does not support special mode bits in files
		if f.Mode != nil && (*f.Mode&07000) != 0 {
			return fmt.Errorf("Invalid input config: special mode bits are not supported in spec v3.3")
		}
	case reflect.TypeOf(old_types.DirectoryEmbedded1{}):
		d := v.Interface().(old_types.DirectoryEmbedded1)
		// 3.3 does not support special mode bits in directories
		if d.Mode != nil && (*d.Mode&07000) != 0 {
			return fmt.Errorf("Invalid input config: special mode bits are not supported in spec v3.3")
		}
	case reflect.TypeOf(old_types.Resource{}):
		resource := v.Interface().(old_types.Resource)
		// 3.3 does not support arn: scheme for s3
		if util.NotEmpty(resource.Source) {
			u, err := url.Parse(*resource.Source)
			if err != nil {
				return fmt.Errorf("Invalid input config: %v", err)
			}
			if u.Scheme == "arn" {
				return fmt.Errorf("Invalid input config: arn: scheme for s3 is not supported in spec v3.3")
			}
		}
	}
	return descend(v)
}

func descend(v reflect.Value) error {
	k := v.Type().Kind()
	switch {
	case util.IsPrimitive(k):
		return nil
	case k == reflect.Struct:
		for i := 0; i < v.NumField(); i += 1 {
			err := checkValue(v.Field(i))
			if err != nil {
				return err
			}
		}
	case k == reflect.Slice:
		for i := 0; i < v.Len(); i += 1 {
			err := checkValue(v.Index(i))
			if err != nil {
				return err
			}
		}
	case k == reflect.Ptr:
		v = v.Elem()
		if v.IsValid() {
			return checkValue(v)
		}
	}
	return nil
}
