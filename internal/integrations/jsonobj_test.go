package integrations

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// backslash is one backslash, so JSON escapes in test strings stay
// readable.
const backslash = string(rune(92))

// firstDiff describes the first differing line of two texts, for failure
// messages.
func firstDiff(got, want []byte) string {
	g := strings.Split(string(got), "\n")
	w := strings.Split(string(want), "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return "first difference at line " + itoa(i+1) + ":\n- want: " + wl + "\n+ got:  " + gl
		}
	}
	return "texts are equal"
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// readFixture returns a testdata file's bytes.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "claude", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestJSONObjectRoundTrip(t *testing.T) {
	inline := "{\n  \"n\": [\n    1e3,\n    1.50,\n    -0,\n    1E-7\n  ],\n  \"s\": \"caf" + backslash + "u00e9 " + backslash + "/ \u00e9 <&>\",\n" +
		"  \"nested\": {\n    \"z\": 1,\n    \"a\": {},\n    \"m\": []\n  },\n  \"k" + backslash + "u0041\": true\n}\n"
	cases := map[string][]byte{
		"inline":              []byte(inline),
		"typical.json":        readFixture(t, "typical.json"),
		"no-mcp.json":         readFixture(t, "no-mcp.json"),
		"stale-casebook.json": readFixture(t, "stale-casebook.json"),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			o, err := parseJSONObject(in)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			out, err := o.Encode()
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !bytes.Equal(out, in) {
				t.Fatalf("round trip changed bytes; %s", firstDiff(out, in))
			}
		})
	}
}

func TestJSONObjectReindents(t *testing.T) {
	o, err := parseJSONObject([]byte(`{"a":{"b":[1,2]},"c":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	out, err := o.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": {\n    \"b\": [\n      1,\n      2\n    ]\n  },\n  \"c\": \"x\"\n}\n"
	if string(out) != want {
		t.Fatalf("encode = %q, want %q", out, want)
	}
}

func TestJSONObjectEmpty(t *testing.T) {
	for _, in := range []string{"", "  \n\t\r\n", "{}", "{ }\n"} {
		o, err := parseJSONObject([]byte(in))
		if err != nil {
			t.Fatalf("parse %q: %v", in, err)
		}
		out, err := o.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "{}\n" {
			t.Errorf("encode of %q = %q, want {} and a newline", in, out)
		}
	}
}

func TestJSONObjectEdit(t *testing.T) {
	o, err := parseJSONObject([]byte(`{"a":1,"b":2,"c":3}`))
	if err != nil {
		t.Fatal(err)
	}
	o.Set("b", json.RawMessage(`"two"`))
	o.Set("d", json.RawMessage(`4`))
	if !o.Delete("a") || o.Delete("missing") {
		t.Fatal("Delete reported the wrong presence")
	}
	if v, ok := o.Get("b"); !ok || string(v) != `"two"` {
		t.Fatalf("Get(b) = %s, %v", v, ok)
	}
	out, _ := o.Encode()
	if want := "{\n  \"b\": \"two\",\n  \"c\": 3,\n  \"d\": 4\n}\n"; string(out) != want {
		t.Fatalf("encode = %q, want %q", out, want)
	}
}

func TestParseJSONObjectRefuses(t *testing.T) {
	// SECRETX marks input text that must never reach an error message.
	cases := []struct {
		name, in, want string
	}{
		{"trailing data", `{"SECRETX":1} SECRETX`, "unexpected data after the JSON object"},
		{"second object", `{"SECRETX":1}{}`, "unexpected data after the JSON object"},
		{"array", `["SECRETX"]`, "top-level value is not a JSON object"},
		{"null", `null`, "top-level value is not a JSON object"},
		{"string", `"SECRETX"`, "top-level value is not a JSON object"},
		{"duplicate key", `{"SECRETX":1,"SECRETX":2}`, "duplicate key at byte 13"},
		{"escaped duplicate key", `{"A":1,"` + backslash + `u0041":2}`, "duplicate key at byte 7"},
		{"byte order mark", "\xEF\xBB\xBF{\"SECRETX\":1}", "starts with a byte order mark"},
		{"truncated", `{"SECRETX":`, "not valid JSON (unexpected end of data)"},
		{"unclosed", `{"SECRETX":1`, "not valid JSON"},
		{"bad token", `{"SECRETX": SECRETX}`, "not valid JSON (syntax error at byte 13)"},
		{"missing comma", `{"SECRETX":1 "b":2}`, "not valid JSON (syntax error at byte"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseJSONObject([]byte(c.in))
			if err == nil {
				t.Fatalf("parse %q succeeded, want an error", c.in)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want it to contain %q", err, c.want)
			}
			if strings.Contains(err.Error(), "SECRETX") {
				t.Errorf("error %q quotes the input", err)
			}
		})
	}
}

func TestChildObjectErrors(t *testing.T) {
	doc := `{"x": 1, "mcpServers": {"a": 1, "a": 2}}`
	o, err := parseJSONObject([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	_, present, err := o.childObject("mcpServers")
	if !present || err == nil || err.Error() != "mcpServers: duplicate key at byte 32" {
		t.Fatalf("childObject = present %v, err %v; want the duplicate key offset in the whole document", present, err)
	}
	if doc[32:35] != `"a"` {
		t.Fatalf("test offset is wrong: %q", doc[32:35])
	}
	o, _ = parseJSONObject([]byte(`{"mcpServers": []}`))
	if _, _, err := o.childObject("mcpServers"); err == nil || err.Error() != "mcpServers is not a JSON object" {
		t.Fatalf("childObject of an array = %v", err)
	}
	if _, present, err := o.childObject("absent"); present || err != nil {
		t.Fatalf("childObject of an absent key = %v, %v", present, err)
	}
}

func TestMarshalNoEscape(t *testing.T) {
	raw, err := marshalNoEscape("C:/Program Files/a&b <c>")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `"C:/Program Files/a&b <c>"` {
		t.Fatalf("marshalNoEscape = %s", raw)
	}
}
