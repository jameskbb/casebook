package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var icoSizes = []int{16, 24, 32, 48, 64, 128, 256}

func TestRenderPNG1024(t *testing.T) {
	img := renderPNG(1024)
	if b := img.Bounds(); b.Dx() != 1024 || b.Dy() != 1024 {
		t.Fatalf("size = %dx%d, want 1024x1024", b.Dx(), b.Dy())
	}
	if a := img.NRGBAAt(0, 0).A; a != 0 {
		t.Errorf("corner alpha = %d, want 0 (rounded tile)", a)
	}
	if a := img.NRGBAAt(512, 140).A; a != 255 {
		t.Errorf("tile alpha = %d, want 255", a)
	}
	data, err := encodePNG(img)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b := decoded.Bounds(); b.Dx() != 1024 || b.Dy() != 1024 {
		t.Fatalf("decoded size = %dx%d", b.Dx(), b.Dy())
	}
}

func TestEncodeICO(t *testing.T) {
	data, err := encodeICO(icoSizes)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 6 {
		t.Fatalf("ico too short: %d bytes", len(data))
	}
	reserved := binary.LittleEndian.Uint16(data[0:])
	kind := binary.LittleEndian.Uint16(data[2:])
	count := binary.LittleEndian.Uint16(data[4:])
	if reserved != 0 || kind != 1 || count != 7 {
		t.Fatalf("header = %d %d %d, want 0 1 7", reserved, kind, count)
	}
	for i, size := range icoSizes {
		e := data[6+16*i : 6+16*(i+1)]
		w, h := int(e[0]), int(e[1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		if w != size || h != size {
			t.Errorf("entry %d declares %dx%d, want %d", i, w, h, size)
		}
		if planes, bpp := binary.LittleEndian.Uint16(e[4:]), binary.LittleEndian.Uint16(e[6:]); planes != 1 || bpp != 32 {
			t.Errorf("entry %d planes %d bpp %d, want 1 and 32", i, planes, bpp)
		}
		n := binary.LittleEndian.Uint32(e[8:])
		off := binary.LittleEndian.Uint32(e[12:])
		if int(off+n) > len(data) {
			t.Fatalf("entry %d runs past the end", i)
		}
		blob := data[off : off+n]
		if !bytes.HasPrefix(blob, []byte("\x89PNG\r\n\x1a\n")) {
			t.Fatalf("entry %d is not stored as PNG", i)
		}
		img, err := png.Decode(bytes.NewReader(blob))
		if err != nil {
			t.Fatalf("entry %d decode: %v", i, err)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Errorf("entry %d image is %dx%d, want %d", i, b.Dx(), b.Dy(), size)
		}
	}
}

func TestSVG(t *testing.T) {
	svg := renderSVG()
	if strings.Count(svg, "<svg") != 1 {
		t.Fatalf("want exactly one <svg element:\n%s", svg)
	}
	for _, want := range []string{`viewBox="0 0 1024 1024"`, "#23476B", "#F2F4F3", "#2E6B4E"} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg lacks %q", want)
		}
	}
	if strings.Contains(svg, "<text") {
		t.Error("the icon has no text")
	}
}

func TestDeterministic(t *testing.T) {
	if renderSVG() != renderSVG() {
		t.Error("svg differs between runs")
	}
	a, err := encodePNG(renderPNG(256))
	if err != nil {
		t.Fatal(err)
	}
	b, err := encodePNG(renderPNG(256))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("png differs between runs")
	}
	x, err := encodeICO(icoSizes)
	if err != nil {
		t.Fatal(err)
	}
	y, err := encodeICO(icoSizes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(x, y) {
		t.Error("ico differs between runs")
	}
}

func TestWriteIcons(t *testing.T) {
	dir := t.TempDir()
	if err := writeIcons(dir); err != nil {
		t.Fatal(err)
	}
	first := map[string][]byte{}
	for _, name := range []string{"appicon.svg", "appicon.png", filepath.Join("windows", "icon.ico")} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s is empty", name)
		}
		first[name] = data
	}
	if err := writeIcons(dir); err != nil {
		t.Fatal(err)
	}
	for name, data := range first {
		again, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, again) {
			t.Errorf("%s changed on the second run", name)
		}
	}
}
