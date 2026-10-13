//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package azlogs

// this file contains handwritten additions to the generated code

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// ClientOptions contains optional settings for Client.
type ClientOptions struct {
	azcore.ClientOptions
}

// NewClient creates a client that accesses Azure Monitor logs data.
func NewClient(credential azcore.TokenCredential, options *ClientOptions) (*Client, error) {
	if options == nil {
		options = &ClientOptions{}
	}
	if reflect.ValueOf(options.Cloud).IsZero() {
		options.Cloud = cloud.AzurePublic
	}
	c, ok := options.Cloud.Services[ServiceName]
	if !ok || c.Audience == "" || c.Endpoint == "" {
		return nil, errors.New("provided Cloud field is missing Azure Monitor Logs configuration")
	}

	authPolicy := runtime.NewBearerTokenPolicy(credential, []string{c.Audience + "/.default"}, nil)
	azcoreClient, err := azcore.NewClient(moduleName, version, runtime.PipelineOptions{PerRetry: []policy.Policy{authPolicy}}, &options.ClientOptions)
	if err != nil {
		return nil, err
	}
	return &Client{host: c.Endpoint, internal: azcoreClient}, nil
}

// ErrorInfo - The code and message for an error.
type ErrorInfo struct {
	// REQUIRED; A machine readable error code.
	Code string

	// full error message detailing why the operation failed.
	data []byte
}

// UnmarshalJSON implements the json.Unmarshaller interface for type ErrorInfo.
func (e *ErrorInfo) UnmarshalJSON(data []byte) error {
	e.data = data
	ei := struct{ Code string }{}
	if err := json.Unmarshal(data, &ei); err != nil {
		return fmt.Errorf("unmarshalling type %T: %v", e, err)
	}
	e.Code = ei.Code

	return nil
}

// Error implements a custom error for type ErrorInfo.
func (e *ErrorInfo) Error() string {
	return string(e.data)
}

// Row of data in a table, types of data used by service specified in ColumnType
type Row []any

// TimeInterval specifies the time range over which to query.
// Use NewTimeInterval() for help formatting.
// Follows the ISO8601 time interval standard with most common
// format being startISOTime/endISOTime. ISO8601 durations also supported (ex "PT2H" for last two hours).
// Use UTC for all times.
type TimeInterval string

// NewTimeInterval creates a TimeInterval for use in a query.
// Use UTC for start and end times.
func NewTimeInterval(start time.Time, end time.Time) TimeInterval {
	return TimeInterval(start.Format(time.RFC3339) + "/" + end.Format(time.RFC3339))
}

// Values returns the interval's start and end times if it's in the format startISOTime/endISOTime, else it will return an error.
func (i TimeInterval) Values() (time.Time, time.Time, error) {
	// split into different start and end times
	times := strings.Split(string(i), "/")
	if len(times) != 2 {
		return time.Time{}, time.Time{}, errors.New("time interval should be in format startISOTime/endISOTime")
	}
	start, err := time.Parse(time.RFC3339, times[0])
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("error parsing start time")
	}
	end, err := time.Parse(time.RFC3339, times[1])
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("error parsing end time")
	}
	// return times
	return start, end, nil
}

// QueryOptions sets server timeout, query statistics and visualization information
type QueryOptions struct {
	// Set Statistics to true to get logs query execution statistics,
	// such as CPU and memory consumption. Defaults to false.
	Statistics *bool

	// Set Visualization to true to get visualization
	// data for logs queries. Defaults to false.
	Visualization *bool

	// By default, the Azure Monitor Query service will run your
	// query for up to three minutes. To increase the default timeout,
	// set Wait to desired number of seconds.
	// Max wait time the service will allow is ten minutes (600 seconds).
	Wait *int
}

// preferHeader converts QueryOptions from struct to properly formatted sting
// to be used in the request Prefer Header
func (l QueryOptions) preferHeader() string {
	var options []string
	if l.Statistics != nil && *l.Statistics {
		options = append(options, "include-statistics=true")
	}
	if l.Visualization != nil && *l.Visualization {
		options = append(options, "include-render=true")
	}
	if l.Wait != nil {
		options = append(options, fmt.Sprintf("wait=%d", *l.Wait))
	}
	return strings.Join(options, ",")
}
