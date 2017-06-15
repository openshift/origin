package lager

import "encoding/json"

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	ERROR
	FATAL
)

type Data map[string]interface{}

type LogFormat struct {
	Timestamp string   `json:"timestamp"`
	Source    string   `json:"source"`
	Message   string   `json:"message"`
	LogLevel  LogLevel `json:"log_level"`
	Data      Data     `json:"data"`
}

func (log LogFormat) ToJSON() []byte {
	content, err := json.Marshal(log)
	if err != nil {
		panic(err)
	}
	return content
}
