package rosacli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
	"gopkg.in/yaml.v3"
)

type Parser struct {
	JsonData  *jsonData
	TableData *tableData
	TextData  *textData
}

func NewParser() *Parser {
	jsonD := new(jsonData)
	tableD := new(tableData)
	textD := new(textData)

	p := &Parser{
		JsonData:  jsonD,
		TableData: tableD,
		TextData:  textD,
	}
	return p
}

type jsonData struct {
	input  bytes.Buffer
	output interface{}
}

type tableData struct {
	input  bytes.Buffer
	output []map[string]interface{}
}

type textData struct {
	input  bytes.Buffer
	output string
	tip    string
}

// Read the cmd input and return the []byte array
func ReadLines(in bytes.Buffer) [][]byte {
	lines := [][]byte{}
	var line []byte
	var err error
	for err == nil {
		line, err = in.ReadBytes('\n')
		lines = append(lines, line)
	}
	return lines
}

func (jd *jsonData) Input(input bytes.Buffer) *jsonData {
	jd.input = input
	return jd
}
func (jd *jsonData) Output() interface{} {
	return jd.output
}

func (td *textData) Input(input bytes.Buffer) *textData {
	td.input = input
	return td
}
func (td *textData) Output() string {
	return td.output
}
func (td *textData) Tip() string {
	return td.tip
}

func (tad *tableData) Input(input bytes.Buffer) *tableData {
	tad.input = input
	return tad
}
func (tad *tableData) Output() []map[string]interface{} {
	return tad.output
}

// It extracts the useful result struct as a map and the message as a string
func (td *textData) Parse() *textData {
	var tips bytes.Buffer
	var results bytes.Buffer

	input := td.input
	lines := ReadLines(input)
	reg1 := regexp.MustCompile(`.*[IEW].*:\x20\S.*\s+\S+`)
	reg2 := regexp.MustCompile("^```\\s*")
	for _, line := range lines {
		strline := string(line)
		if reg2.FindString(strline) != "" {
			continue
		}
		result := reg1.FindString(strline)
		if result == "" {
			results.WriteString(strline)
		} else {
			tips.WriteString(strline)
		}
	}

	td.output = results.String()
	td.tip = tips.String()
	return td
}

// TransformOutput allows to transform the string before it would be parsed
func (td *textData) TransformOutput(transformFunc func(str string) string) *textData {
	td.output = transformFunc(td.output)
	return td
}

func (td *textData) YamlToMap() (res map[string]interface{}, err error) {
	res = make(map[string]interface{})

	// Escape value(s) with quote due to https://github.com/go-yaml/yaml/issues/784
	// This happens sometimes in NodePool Message like `WaitingForAvailableMachines: InstanceNotReady,WaitingForNodeRef`
	// This would fail to unmarshal due to the `: ` in the value ...
	escapedOutput, err := escapeYamlStringValues(td.output)
	if err != nil {
		return
	}
	err = yaml.Unmarshal([]byte(escapedOutput), &res)
	return
}

// escapeYamlStringValues escapes yaml values if they contain any special characters: https://www.yaml.info/learn/quote.html#noplain
// Checks have to be completed on demande...
func escapeYamlStringValues(input string) (string, error) {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()
		key, value, found := strings.Cut(line, ":")
		if found {
			value = strings.TrimSpace(value)

			// Checks to perform
			if !strings.HasPrefix(value, "'") && strings.Contains(value, ": ") {
				line = fmt.Sprintf("%s: '%s'", key, value)
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), scanner.Err()
}

func (td *textData) JsonToMap() (map[string]interface{}, error) {
	res := make(map[string]interface{})
	err := json.Unmarshal([]byte(td.output), &res)
	return res, err
}

// Parse the cmd table ouptut title
func tableTitle(titleLine []byte) map[int]string {
	offsetMap := map[int]string{}
	var elem []byte
	startOffset := 0
	startCount := true

	for offset, char := range titleLine {
		if offset == len(titleLine)-1 {
			if len(elem) != 0 {
				key := string(elem)
				offsetMap[startOffset] = key
				elem = []byte{}
				startCount = true
			}
		}
		if char != ' ' {
			elem = append(elem, char)
			if startCount {
				startOffset = offset
				startCount = false
			}
		} else {
			if offset == 0 {
				continue
			} else if titleLine[offset-1] == ' ' {
				if len(elem) != 0 {
					key := string(elem)
					offsetMap[startOffset] = key
					elem = []byte{}
					startCount = true
				}
			} else {
				elem = append(elem, char)
			}
		}
	}
	for key, val := range offsetMap {
		offsetMap[key] = strings.TrimRight(val, " ")
	}
	return offsetMap
}

// Parse the cmd table ouptut line
func tableLine(line []byte, offsetMap map[int]string) map[string]interface{} {
	resultMap := map[string]interface{}{}
	var elemValue string
	for offset, key := range offsetMap {
		if offset >= len(line) {
			resultMap[key] = ""
			continue
		}
		for subOff, char := range line[offset:] {
			if subOff == len(line[offset:])-1 {
				elemValue = strings.TrimRight(string(line[offset:]), "\n")
				elemValue = strings.TrimLeft(elemValue, " ")
				resultMap[key] = elemValue
			}
			if char == ' ' && line[subOff+1+offset] == ' ' {
				elemValue = strings.TrimRight(string(line[offset:subOff+offset]), " ")
				elemValue = strings.TrimLeft(elemValue, " ")
				resultMap[key] = elemValue
				break
			}
		}
	}
	return resultMap
}

// Parse the table output of the rosa cmd
func (tab *tableData) Parse() *tableData {
	var results bytes.Buffer
	input := tab.input
	lines := ReadLines(input)
	reg1 := regexp.MustCompile(`.*[IEW].*:\x20\S.*\s+\S+`)
	reg2 := regexp.MustCompile("^```\\s*")
	reg3 := regexp.MustCompile(`time=(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z) level=(\w+) msg=(.*)`)
	for _, line := range lines {
		strline := string(line)
		if reg2.FindString(strline) != "" || reg3.FindString(strline) != "" {
			continue
		}
		result := reg1.FindString(strline)
		if result == "" {
			results.WriteString(strline)
		}
	}
	lines = ReadLines(results)
	titleLine := lines[0]
	offsetMap := tableTitle(titleLine)

	result := []map[string]interface{}{}
	if len(lines) >= 2 {
		for _, line := range lines[1 : len(lines)-1] {
			result = append(result, tableLine(line, offsetMap))
		}
	}
	tab.output = result
	return tab
}

func (jd *jsonData) Parse() *jsonData {
	var object map[string]interface{}
	err := json.Unmarshal(jd.input.Bytes(), &object)
	if err != nil {
		logger.Errorf(" error in Parse is %v", err)
	}

	jd.output = object
	return jd
}

func (jd *jsonData) ParseList(jsonStr string) *jsonData {
	var object []map[string]interface{}
	err := json.Unmarshal(jd.input.Bytes(), &object)
	if err != nil {
		logger.Errorf(" error in Parse is %v", err)
	}

	jd.output = object
	return jd
}

func (jd *jsonData) DigObject(keys ...interface{}) interface{} {
	value := dig(jd.output, keys)
	return value
}
func (jd *jsonData) DigString(keys ...interface{}) string {
	switch result := dig(jd.output, keys).(type) {
	case nil:
		return ""
	case string:
		return result
	case fmt.Stringer:
		return result.String()
	default:
		return fmt.Sprintf("%s", result)
	}
}
func (jd *jsonData) DigBool(keys ...interface{}) bool {
	switch result := dig(jd.output, keys).(type) {
	case nil:
		return false
	case bool:
		return result
	case string:
		b, err := strconv.ParseBool(result)
		if err != nil {
			return false
		}
		return b
	default:
		return false
	}
}

func (jd *jsonData) DigFloat(keys ...interface{}) float64 {
	value := dig(jd.output, keys)
	result := value.(float64)
	return result
}

func dig(object interface{}, keys []interface{}) interface{} {
	if object == nil || len(keys) == 0 {
		return nil
	}
	switch key := keys[0].(type) {
	case string:
		switch data := object.(type) {
		case map[string]interface{}:
			value := data[key]
			if len(keys) == 1 {
				return value
			}
			return dig(value, keys[1:])
		}
	case int:
		switch data := object.(type) {
		case []interface{}:
			value := data[key]
			if len(keys) == 1 {
				return value
			}
			return dig(value, keys[1:])
		}
	}
	return nil
}

// mapStructure will map the map to the address of the structre *i
func MapStructure(m map[string]interface{}, i interface{}) error {
	m = ConvertMapKey(m)
	jsonbody, err := json.Marshal(m)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonbody, i)
	if err != nil {
		return err
	}
	return nil
}

func ConvertMapKey(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		m[k] = Convert(v)
	}
	return m
}

func Convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = Convert(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = Convert(v)
		}
	}
	return i
}
