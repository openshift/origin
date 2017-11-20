package configuration

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type optionConverter func(interface{}, interface{}) (interface{}, error)

func convertBool(value interface{}, defval interface{}) (b interface{}, err error) {
	switch t := value.(type) {
	case bool:
		return t, nil
	case string:
		switch strings.ToLower(t) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return defval, fmt.Errorf("%#+v is not a boolean", value)
}

func convertString(value interface{}, defval interface{}) (b interface{}, err error) {
	s, ok := value.(string)
	if !ok {
		return defval, fmt.Errorf("expected string, not %T", value)
	}
	return s, err
}

func convertDuration(value interface{}, defval interface{}) (d interface{}, err error) {
	s, ok := value.(string)
	if !ok {
		return defval, fmt.Errorf("expected string, not %T", value)
	}

	parsed, err := time.ParseDuration(s)
	if err != nil {
		return defval, fmt.Errorf("parse duration error: %v", err)
	}
	return parsed, nil
}

func getOptionValue(
	envVar string,
	optionName string,
	defval interface{},
	options map[string]interface{},
	conversionFunc optionConverter,
) (value interface{}, err error) {
	value = defval
	if optValue, ok := options[optionName]; ok {
		converted, convErr := conversionFunc(optValue, defval)
		if convErr != nil {
			err = fmt.Errorf("config option %q: invalid value: %v", optionName, convErr)
		} else {
			value = converted
		}
	}

	if len(envVar) == 0 {
		return
	}
	envValue := os.Getenv(envVar)
	if len(envValue) == 0 {
		return
	}

	converted, convErr := conversionFunc(envValue, defval)
	if convErr != nil {
		err = fmt.Errorf("invalid value of environment variable %s: %v", envVar, convErr)
	} else {
		value = converted
	}

	return
}

func getBoolOption(envVar string, optionName string, defval bool, options map[string]interface{}) (bool, error) {
	value, err := getOptionValue(envVar, optionName, defval, options, convertBool)
	return value.(bool), err
}

func getStringOption(envVar string, optionName string, defval string, options map[string]interface{}) (string, error) {
	value, err := getOptionValue(envVar, optionName, defval, options, convertString)
	return value.(string), err
}

func getDurationOption(envVar string, optionName string, defval time.Duration, options map[string]interface{}) (time.Duration, error) {
	value, err := getOptionValue(envVar, optionName, defval, options, convertDuration)
	return value.(time.Duration), err
}
