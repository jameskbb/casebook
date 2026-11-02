package export

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteCSVHeader(t *testing.T) {
	var b bytes.Buffer
	if err := WriteCSV(&b, nil, 50); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(b.String(), "id,total") {
		t.Fatalf("got %q", b.String())
	}
}
