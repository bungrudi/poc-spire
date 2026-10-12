package kafkacmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"
	"unicode/utf8"
)

var integer = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// StrictJSON preserves the original bytes for signature verification while
// rejecting the ambiguities encoding/json ordinarily accepts.
func StrictJSON(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) || !validEscapes(raw) {
		return nil, errors.New("invalid_unicode")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	value, err := jsonValue(dec, 1)
	if err != nil {
		return nil, err
	}
	if _, err = dec.Token(); err != io.EOF {
		return nil, errors.New("trailing_json")
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("object_required")
	}
	return obj, nil
}
func jsonValue(dec *json.Decoder, depth int) (any, error) {
	if depth > 4 {
		return nil, errors.New("json_depth")
	}
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := token.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := map[string]any{}
			for dec.More() {
				token, err := dec.Token()
				if err != nil {
					return nil, err
				}
				name, ok := token.(string)
				if !ok {
					return nil, errors.New("member_required")
				}
				if _, exists := obj[name]; exists {
					return nil, errors.New("duplicate_member")
				}
				value, err := jsonValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				obj[name] = value
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim('}') {
				return nil, errors.New("invalid_object")
			}
			return obj, nil
		case '[':
			list := []any{}
			for dec.More() {
				value, err := jsonValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				list = append(list, value)
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim(']') {
				return nil, errors.New("invalid_array")
			}
			return list, nil
		}
	case json.Number:
		if !integer.MatchString(string(t)) {
			return nil, errors.New("decimal_integer_required")
		}
		if _, err := strconv.ParseInt(string(t), 10, 64); err != nil {
			return nil, errors.New("integer_range")
		}
		return t, nil
	case string:
		return t, nil
	case bool:
		return t, nil
	}
	return nil, errors.New("invalid_json_type")
}

// JSON's escaped UTF-16 surrogates must form a complete pair. A literal escaped
// backslash is skipped so the string "\\uD800" remains ordinary text.
func validEscapes(raw []byte) bool {
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if n >= 0xDC00 && n <= 0xDFFF {
			return false
		}
		if n >= 0xD800 && n <= 0xDBFF {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		}
	}
	return !quoted
}
func exactKeys(obj map[string]any, names ...string) bool {
	if len(obj) != len(names) {
		return false
	}
	for _, n := range names {
		if _, ok := obj[n]; !ok {
			return false
		}
	}
	return true
}
func stringField(obj map[string]any, name string) (string, bool) {
	s, ok := obj[name].(string)
	return s, ok
}
func intField(obj map[string]any, name string) (int64, bool) {
	n, ok := obj[name].(json.Number)
	if !ok {
		return 0, false
	}
	v, e := strconv.ParseInt(string(n), 10, 64)
	return v, e == nil
}
func timestamp(n int64) bool { return n >= 0 && n <= 4102444800 }
