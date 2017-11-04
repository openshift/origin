/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func normalEventBuilder(reason string) *MessageBuilder {
	return new(MessageBuilder).normal().reason(reason)
}

func warningEventBuilder(reason string) *MessageBuilder {
	return new(MessageBuilder).warning().reason(reason)
}

// MessageBuilder builds Event Type messages to help unit tests create these strings
// Example usage:
// mb := new(MessageBuilder).warning().reson("ReasonForError).msg("Error: hello world.")
// mb.String()
type MessageBuilder struct {
	eventMessage  string
	reasonMessage string
	message       string
}

// normal sets the event type field to normal for the message builder in <type> <reason> <message>
func (mb *MessageBuilder) normal() *MessageBuilder {
	mb.eventMessage = corev1.EventTypeNormal
	return mb
}

// warning sets the event type field to warning for the message builder in <type> <reason> <message>
func (mb *MessageBuilder) warning() *MessageBuilder {
	mb.eventMessage = corev1.EventTypeWarning
	return mb
}

// reason sets the event reason field for the message builder in <type> <reason> <message>
func (mb *MessageBuilder) reason(reason string) *MessageBuilder {
	mb.reasonMessage = reason
	return mb
}

// msg appends a message to the message builder
func (mb *MessageBuilder) msg(msg string) *MessageBuilder {
	space := ""
	if mb.message > "" {
		space = " "
	}
	mb.message = fmt.Sprintf(`%s%s%s`, mb.message, space, msg)
	return mb
}

// msgf appends a message after formatting to the message builder
func (mb *MessageBuilder) msgf(format string, a ...interface{}) *MessageBuilder {
	msg := fmt.Sprintf(format, a...)
	return mb.msg(msg)
}

// stringArr is a fast way to create a string array containing mb.String()
func (mb *MessageBuilder) stringArr() []string {
	return []string{mb.String()}
}

func (mb *MessageBuilder) String() string {
	s := ""
	space := ""
	if mb.eventMessage > "" {
		s += fmt.Sprintf("%s%s", space, mb.eventMessage)
		space = " "
	}
	if mb.reasonMessage > "" {
		s += fmt.Sprintf("%s%s", space, mb.reasonMessage)
		space = " "
	}
	if mb.message > "" {
		s += fmt.Sprintf("%s%s", space, mb.message)
		space = " "
	}
	return s
}
