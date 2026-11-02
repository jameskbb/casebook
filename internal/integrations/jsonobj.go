package integrations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// jsonMember is one member of a jsonObject. Value is the member's raw JSON,
// never decoded into Go types, so numbers, string escapes and nested key
// order survive a round trip (D-05).
type jsonMember struct {
	Key   string
	Value json.RawMessage

	// rawKey is the key exactly as spelled in the parsed input (quotes
	// included); nil for members added by Set, which encode Key instead.
	rawKey []byte
	// offset is the byte offset of Value in the parsed document, for error
	// messages about nested values; 0 for members added by Set.
	offset int64
}

// jsonObject is a JSON object that keeps its members in input order.
type jsonObject struct {
	Members []jsonMember
}

// errNotObject reports a value that is not a JSON object.
var errNotObject = errors.New("top-level value is not a JSON object")

// utf8BOM is the UTF-8 byte order mark.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// jsonSpace is the JSON whitespace set.
const jsonSpace = " \t\r\n"

// parseJSONObject parses data as one JSON object. Empty or whitespace-only
// data gives an empty object. It refuses a leading byte order mark, a
// top-level value that is not an object, duplicate keys and trailing data.
// Error texts never quote the input: syntax errors carry a byte offset.
func parseJSONObject(data []byte) (*jsonObject, error) {
	return parseJSONObjectAt(data, 0)
}

// parseJSONObjectAt is parseJSONObject for data that starts at byte base of
// an enclosing document, so reported offsets point into that document.
func parseJSONObjectAt(data []byte, base int64) (*jsonObject, error) {
	if bytes.HasPrefix(data, utf8BOM) {
		return nil, errors.New("starts with a byte order mark")
	}
	if len(bytes.Trim(data, jsonSpace)) == 0 {
		return &jsonObject{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, decodeError(err, base)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errNotObject
	}
	o := &jsonObject{}
	seen := make(map[string]bool)
	for dec.More() {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			return nil, decodeError(err, base)
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("not valid JSON (syntax error at byte %d)", base+start)
		}
		end := dec.InputOffset()
		raw := data[start:end]
		rawKey := bytes.TrimLeft(raw, jsonSpace+",")
		keyPos := base + end - int64(len(rawKey))
		if seen[key] {
			return nil, fmt.Errorf("duplicate key at byte %d", keyPos)
		}
		seen[key] = true
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return nil, decodeError(err, base)
		}
		rest := data[end:]
		valPos := end + int64(len(rest)-len(bytes.TrimLeft(rest, jsonSpace+":")))
		o.Members = append(o.Members, jsonMember{
			Key:    key,
			Value:  val,
			rawKey: append([]byte(nil), rawKey...),
			offset: base + valPos,
		})
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return nil, decodeError(err, base)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the JSON object")
	}
	return o, nil
}

// decodeError turns a decoder error into a message that names a byte offset
// at most, never input text.
func decodeError(err error, base int64) error {
	var se *json.SyntaxError
	switch {
	case errors.As(err, &se):
		return fmt.Errorf("not valid JSON (syntax error at byte %d)", base+se.Offset)
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New("not valid JSON (unexpected end of data)")
	default:
		return errors.New("not valid JSON")
	}
}

// index returns the position of key, or -1.
func (o *jsonObject) index(key string) int {
	for i, m := range o.Members {
		if m.Key == key {
			return i
		}
	}
	return -1
}

// Get returns the raw value of key.
func (o *jsonObject) Get(key string) (json.RawMessage, bool) {
	if i := o.index(key); i >= 0 {
		return o.Members[i].Value, true
	}
	return nil, false
}

// Set replaces key's value in place, or appends a new member.
func (o *jsonObject) Set(key string, v json.RawMessage) {
	if i := o.index(key); i >= 0 {
		o.Members[i].Value = v
		o.Members[i].offset = 0
		return
	}
	o.Members = append(o.Members, jsonMember{Key: key, Value: v})
}

// Delete removes key and reports whether it was present.
func (o *jsonObject) Delete(key string) bool {
	i := o.index(key)
	if i < 0 {
		return false
	}
	o.Members = append(o.Members[:i], o.Members[i+1:]...)
	return true
}

// childObject parses the member key as a JSON object. present is false when
// the member is absent. Errors name the key and, for duplicate keys, a byte
// offset in the parsed document.
func (o *jsonObject) childObject(key string) (child *jsonObject, present bool, err error) {
	i := o.index(key)
	if i < 0 {
		return nil, false, nil
	}
	m := o.Members[i]
	child, err = parseJSONObjectAt(m.Value, m.offset)
	if err != nil {
		if errors.Is(err, errNotObject) || len(bytes.Trim(m.Value, jsonSpace)) == 0 {
			return nil, true, fmt.Errorf("%s is not a JSON object", key)
		}
		return nil, true, fmt.Errorf("%s: %w", key, err)
	}
	return child, true, nil
}

// Encode renders the object with two-space indentation and a trailing
// newline. Member values keep their tokens exactly; only whitespace between
// tokens changes, so a canonical two-space document round-trips byte for
// byte.
func (o *jsonObject) Encode() ([]byte, error) {
	if len(o.Members) == 0 {
		return []byte("{}\n"), nil
	}
	var b bytes.Buffer
	b.WriteString("{\n")
	for i, m := range o.Members {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString("  ")
		key := m.rawKey
		if key == nil {
			k, err := marshalNoEscape(m.Key)
			if err != nil {
				return nil, err
			}
			key = k
		}
		b.Write(key)
		b.WriteString(": ")
		if err := json.Indent(&b, m.Value, "  ", "  "); err != nil {
			return nil, err
		}
	}
	b.WriteString("\n}\n")
	return b.Bytes(), nil
}

// marshalNoEscape encodes v as compact JSON without HTML escaping and
// without a trailing newline.
func marshalNoEscape(v any) (json.RawMessage, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimRight(b.Bytes(), "\n")), nil
}
