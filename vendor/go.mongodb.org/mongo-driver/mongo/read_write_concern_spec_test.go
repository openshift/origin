// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

type connectionStringTest struct {
	Description  string
	URI          string
	Valid        bool
	ReadConcern  map[string]interface{}
	WriteConcern map[string]interface{}
}

type documentTest struct {
	Description          string
	URI                  string
	Valid                bool
	ReadConcern          *readConcern
	ReadConcernDocument  map[string]interface{}
	WriteConcern         *writeConcern
	WriteConcernDocument map[string]interface{}
	IsServerDefault      bool
	IsAcknowledged       *bool
}

type readConcern struct {
	Level *string
}

type writeConcern struct {
	W          interface{}
	Journal    *bool
	WtimeoutMS *int64
}

type connectionStringTests struct {
	Tests []connectionStringTest
}

type documentTestContainer struct {
	Tests []documentTest
}

const testsDir = "../data/read-write-concern/"
const connStringTestsDir = "connection-string"
const documentTestsDir = "document"

// Test case for all connection string spec tests.
func TestReadWriteConcernSpec(t *testing.T) {
	for _, file := range testhelpers.FindJSONFilesInDir(t, path.Join(testsDir, connStringTestsDir)) {
		runConnectionStringTestsInFile(t, file)
	}

	for _, file := range testhelpers.FindJSONFilesInDir(t, path.Join(testsDir, documentTestsDir)) {
		runDocumentTestsInFile(t, file)
	}
}

func runConnectionStringTestsInFile(t *testing.T, filename string) {
	filepath := path.Join(testsDir, connStringTestsDir, filename)
	content, err := ioutil.ReadFile(filepath)
	require.NoError(t, err)

	var container connectionStringTests
	require.NoError(t, json.Unmarshal(content, &container))

	// Remove ".json" from filename.
	filename = filename[:len(filename)-5]

	for _, testCase := range container.Tests {
		runConnectionStringTest(t, fmt.Sprintf("%s/%s/%s", connStringTestsDir, filename, testCase.Description), &testCase)
	}
}

func runDocumentTestsInFile(t *testing.T, filename string) {
	filepath := path.Join(testsDir, documentTestsDir, filename)
	content, err := ioutil.ReadFile(filepath)
	require.NoError(t, err)

	var container documentTestContainer
	require.NoError(t, json.Unmarshal(content, &container))

	// Remove ".json" from filename.
	filename = filename[:len(filename)-5]

	for _, testCase := range container.Tests {
		runDocumentTest(t, fmt.Sprintf("%s/%s/%s", documentTestsDir, filename, testCase.Description), &testCase)
	}
}

func runConnectionStringTest(t *testing.T, testName string, testCase *connectionStringTest) {
	t.Run(testName, func(t *testing.T) {
		cs, err := connstring.Parse(testCase.URI)
		if !testCase.Valid {
			require.Error(t, err)
			return
		}

		require.NoError(t, err)

		if testCase.ReadConcern != nil {
			rc := readConcernFromConnString(&cs)
			if rc == nil {
				rc = readconcern.New()
			}

			typ, data, err := rc.MarshalBSONValue()
			require.NoError(t, err)

			rcDoc := bson.RawValue{Type: typ, Value: data}.Document()
			expectedLevel, expectedFound := testCase.ReadConcern["level"]
			actualLevel, actualErr := rcDoc.LookupErr("level")
			require.Equal(t, expectedFound, actualErr == nil)

			if expectedFound {
				require.Equal(t, expectedLevel, actualLevel.StringValue())
			}
		}

		if testCase.WriteConcern != nil {
			wc := writeConcernFromConnString(&cs)
			if wc == nil {
				wc = writeconcern.New()
			}

			typ, data, err := wc.MarshalBSONValue()
			if err == writeconcern.ErrEmptyWriteConcern {
				if len(testCase.WriteConcern) == 0 {
					return
				}
				if _, exists := testCase.WriteConcern["journal"]; exists && len(testCase.WriteConcern) == 1 {
					return
				}
			}
			require.NoError(t, err)

			wcDoc := bson.RawValue{Type: typ, Value: data}.Document()

			// Don't count journal=false since our write concern type doesn't encode it.
			expectedLength := len(testCase.WriteConcern)
			if j, found := testCase.WriteConcern["journal"]; found && !j.(bool) {
				expectedLength--
			}

			elems, err := wcDoc.Elements()
			require.NoError(t, err)

			require.Equal(t, len(elems), expectedLength)

			for _, e := range elems {

				switch e.Key() {
				case "w":
					v, found := testCase.WriteConcern["w"]
					require.True(t, found)

					vInt := testhelpers.GetIntFromInterface(v)

					if vInt == nil {
						require.Equal(t, e.Value().Type, bson.TypeString)

						vString, ok := v.(string)
						require.True(t, ok)
						require.Equal(t, vString, e.Value().StringValue())

						break
					}

					require.Equal(t, e.Value().Type, bson.TypeInt32)
					require.Equal(t, *vInt, int64(e.Value().Int32()))
				case "wtimeout":
					v, found := testCase.WriteConcern["wtimeoutMS"]
					require.True(t, found)

					i := testhelpers.GetIntFromInterface(v)
					require.NotNil(t, i)
					require.Equal(t, *i, e.Value().Int64())
				case "j":
					v, found := testCase.WriteConcern["journal"]
					require.True(t, found)

					vBool, ok := v.(bool)
					require.True(t, ok)

					require.Equal(t, vBool, e.Value().Boolean())
				}
			}
		}
	})
}

func runDocumentTest(t *testing.T, testName string, testCase *documentTest) {
	t.Run(testName, func(t *testing.T) {
		if testCase.ReadConcern != nil {
			rc := readConcernFromStruct(*testCase.ReadConcern)
			typ, data, err := rc.MarshalBSONValue()
			require.NoError(t, err)

			rcBytes := bson.RawValue{Type: typ, Value: data}.Document()

			actual := make(map[string]interface{})
			err = bson.Unmarshal(rcBytes, &actual)

			requireMapEqual(t, testCase.ReadConcernDocument, actual)
		}

		if testCase.WriteConcern != nil {
			wc, err := writeConcernFromStruct(*testCase.WriteConcern)
			require.NoError(t, err)

			if testCase.IsAcknowledged != nil {
				require.Equal(t, *testCase.IsAcknowledged, wc.Acknowledged())
			}

			typ, data, err := wc.MarshalBSONValue()
			if !testCase.Valid {
				require.Error(t, err)
				return
			}

			if err == writeconcern.ErrEmptyWriteConcern {
				if len(testCase.WriteConcernDocument) == 0 {
					return
				}
				if _, exists := testCase.WriteConcernDocument["j"]; exists && len(testCase.WriteConcernDocument) == 1 {
					return
				}
			}
			require.NoError(t, err)

			wcBytes := bson.RawValue{Type: typ, Value: data}.Document()

			actual := make(map[string]interface{})
			err = bson.Unmarshal(wcBytes, &actual)
			require.NoError(t, err)

			requireMapEqual(t, testCase.WriteConcernDocument, actual)
		}
	})
}

func requireMapEqual(t *testing.T, expected, actual map[string]interface{}) {
	// Since `actual` won't contain j=false, we just check that actual isn't bigger than `expected`.
	// Later, we check that all other keys in `expected` are in `actual`.
	require.True(t, len(expected) >= len(actual))

	for key, expectedVal := range expected {
		actualVal, ok := actual[key]
		// Since write concern's MarshalBSON doesn't marshal j=false, we treat j=false as the
		// same as j not being present.
		//
		// We know that MarshalBSON will only populate j with a bool, so the coercion is safe.
		if key == "j" {
			require.Equal(t, expectedVal, ok && actualVal.(bool))
			continue
		}

		// Assert that the key from `expected` is in `actual`.
		require.True(t, ok)

		// Coerce both to integers if possible (to ensure that things like `float(3)` and `int32(3)` are true)/
		expectedInt := testhelpers.GetIntFromInterface(expectedVal)
		actualInt := testhelpers.GetIntFromInterface(actualVal)
		require.Equal(t, expectedInt == nil, actualInt == nil)

		if expectedInt != nil {
			require.Equal(t, expectedInt, actualInt)
			continue
		}

		// Otherwise, check equality regularly.
		require.Equal(t, expectedVal, actualVal)
	}
}

func readConcernFromStruct(rc readConcern) *readconcern.ReadConcern {
	opts := make([]readconcern.Option, 0)

	if rc.Level != nil {
		opts = append(opts, readconcern.Level(*rc.Level))
	}

	return readconcern.New(opts...)
}

func writeConcernFromStruct(wc writeConcern) (*writeconcern.WriteConcern, error) {
	opts := make([]writeconcern.Option, 0)

	if wc.W != nil {
		if i := testhelpers.GetIntFromInterface(wc.W); i != nil {
			if !int64FitsInInt(*i) {
				return nil, errors.New("write concern `w` value is too large for int")
			}

			opts = append(opts, writeconcern.W(int(*i)))
		} else if s, ok := wc.W.(string); ok {
			opts = append(opts, writeconcern.WTagSet(s))
		} else {
			return nil, errors.New("write concern `w` must be int or string")
		}
	}

	if wc.Journal != nil {
		opts = append(opts, writeconcern.J(*wc.Journal))
	}

	if wc.WtimeoutMS != nil {
		opts = append(opts, writeconcern.WTimeout(time.Duration(*wc.WtimeoutMS)*time.Millisecond))
	}

	return writeconcern.New(opts...), nil
}

func int64FitsInInt(i int64) bool {
	// If casting an int64 to an int changes the value, then it doesn't fit in an int.
	return int64(int(i)) == i
}

func readConcernFromConnString(cs *connstring.ConnString) *readconcern.ReadConcern {
	if len(cs.ReadConcernLevel) == 0 {
		return nil
	}

	rc := &readconcern.ReadConcern{}
	readconcern.Level(cs.ReadConcernLevel)(rc)

	return rc
}

func writeConcernFromConnString(cs *connstring.ConnString) *writeconcern.WriteConcern {
	var wc *writeconcern.WriteConcern

	if len(cs.WString) > 0 {
		if wc == nil {
			wc = writeconcern.New()
		}

		writeconcern.WTagSet(cs.WString)(wc)
	} else if cs.WNumberSet {
		if wc == nil {
			wc = writeconcern.New()
		}

		writeconcern.W(cs.WNumber)(wc)
	}

	if cs.JSet {
		if wc == nil {
			wc = writeconcern.New()
		}

		writeconcern.J(cs.J)(wc)
	}

	if cs.WTimeoutSet {
		if wc == nil {
			wc = writeconcern.New()
		}

		writeconcern.WTimeout(cs.WTimeout)(wc)
	}

	return wc
}
