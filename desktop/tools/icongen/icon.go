package main

// The Casebook icon, defined once as a handful of primitives on a 1024 by
// 1024 canvas and rendered two ways: as SVG source and as antialiased PNG
// (4 by 4 supersampling against signed distance tests). The design is "a
// calm case file" in the D-14 palette: an ink-blue rounded tile, a paper
// case-file folder with a tab, three short text lines, and a claim-green
// badge with a white check at the lower right.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
)

const canvas = 1024.0

// Palette (D-14).
var (
	inkBlue = rgb{0x23, 0x47, 0x6B}
	paper   = rgb{0xF2, 0xF4, 0xF3}
	green   = rgb{0x2E, 0x6B, 0x4E}
	white   = rgb{0xFF, 0xFF, 0xFF}
)

type rgb struct{ r, g, b uint8 }

func (c rgb) hex() string { return fmt.Sprintf("#%02X%02X%02X", c.r, c.g, c.b) }

// shape is a primitive with a signed distance: negative inside.
type shape interface {
	dist(x, y float64) float64
	svg(fill string) string
}

type roundRect struct{ x0, y0, x1, y1, r float64 }

func (s roundRect) dist(x, y float64) float64 {
	cx, cy := (s.x0+s.x1)/2, (s.y0+s.y1)/2
	hx, hy := (s.x1-s.x0)/2-s.r, (s.y1-s.y0)/2-s.r
	qx, qy := math.Abs(x-cx)-hx, math.Abs(y-cy)-hy
	outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	inside := math.Min(math.Max(qx, qy), 0)
	return outside + inside - s.r
}

func (s roundRect) svg(fill string) string {
	return fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s" rx="%s" %s/>`,
		num(s.x0), num(s.y0), num(s.x1-s.x0), num(s.y1-s.y0), num(s.r), fill)
}

type circle struct{ cx, cy, r float64 }

func (s circle) dist(x, y float64) float64 { return math.Hypot(x-s.cx, y-s.cy) - s.r }

func (s circle) svg(fill string) string {
	return fmt.Sprintf(`<circle cx="%s" cy="%s" r="%s" %s/>`, num(s.cx), num(s.cy), num(s.r), fill)
}

// capsule is a stroke from a to b with round caps (radius r = half width).
type capsule struct{ ax, ay, bx, by, r float64 }

func (s capsule) dist(x, y float64) float64 {
	px, py := x-s.ax, y-s.ay
	dx, dy := s.bx-s.ax, s.by-s.ay
	h := math.Max(0, math.Min(1, (px*dx+py*dy)/(dx*dx+dy*dy)))
	return math.Hypot(px-dx*h, py-dy*h) - s.r
}

func (s capsule) svg(fill string) string {
	stroke := strings.Replace(fill, "fill=", "stroke=", 1)
	stroke = strings.Replace(stroke, "fill-opacity=", "stroke-opacity=", 1)
	return fmt.Sprintf(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke-width="%s" stroke-linecap="round" %s/>`,
		num(s.ax), num(s.ay), num(s.bx), num(s.by), num(2*s.r), stroke)
}

type layer struct {
	s       shape
	c       rgb
	opacity float64
	// minSize leaves fine detail out of tiny renders (0 means always drawn).
	minSize int
}

// layers is the whole icon, painted in order.
var layers = []layer{
	// Tile: rounded square, corner radius about 22 percent.
	{roundRect{48, 48, 976, 976, 204}, inkBlue, 1, 0},
	// Case-file folder: the tab (its lower corners hide under the body),
	// then the body.
	{roundRect{204, 236, 452, 400, 36}, paper, 1, 0},
	{roundRect{204, 292, 756, 740, 44}, paper, 1, 0},
	// Three short text lines on the folder; below 32 pixels they would only
	// blur the folder, so tiny renders leave them out.
	{roundRect{284, 420, 676, 456, 18}, inkBlue, 0.38, 32},
	{roundRect{284, 504, 620, 540, 18}, inkBlue, 0.38, 32},
	{roundRect{284, 588, 516, 624, 18}, inkBlue, 0.38, 32},
	// Badge: a tile-colored ring separates it from the folder.
	{circle{728, 712, 176}, inkBlue, 1, 0},
	{circle{728, 712, 142}, green, 1, 0},
	// The check: two strokes.
	{capsule{660, 716, 708, 764, 21}, white, 1, 0},
	{capsule{708, 764, 798, 664, 21}, white, 1, 0},
}

// num formats a coordinate without trailing zeros.
func num(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

// renderSVG returns the icon as SVG source.
func renderSVG() string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">` + "\n")
	b.WriteString("  <title>Casebook</title>\n")
	for _, l := range layers {
		fill := fmt.Sprintf(`fill="%s"`, l.c.hex())
		if l.opacity < 1 {
			fill += fmt.Sprintf(` fill-opacity="%s"`, num(l.opacity))
		}
		b.WriteString("  " + l.s.svg(fill) + "\n")
	}
	b.WriteString("</svg>\n")
	return b.String()
}

const samples = 4

// renderPNG rasterizes the icon at size by size pixels with 4 by 4
// supersampling. Colors are composited premultiplied, then averaged.
func renderPNG(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := canvas / float64(size)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var sr, sg, sb, sa float64
			for j := 0; j < samples; j++ {
				for i := 0; i < samples; i++ {
					x := (float64(px) + (float64(i)+0.5)/samples) * scale
					y := (float64(py) + (float64(j)+0.5)/samples) * scale
					r, g, b, a := sample(x, y, size)
					sr, sg, sb, sa = sr+r, sg+g, sb+b, sa+a
				}
			}
			n := float64(samples * samples)
			sr, sg, sb, sa = sr/n, sg/n, sb/n, sa/n
			var c color.NRGBA
			if sa > 0 {
				c = color.NRGBA{
					R: to8(sr / sa),
					G: to8(sg / sa),
					B: to8(sb / sa),
					A: to8(sa),
				}
			}
			img.SetNRGBA(px, py, c)
		}
	}
	return img
}

// sample composites every layer drawn at this size at one point and returns
// premultiplied color in 0..1.
func sample(x, y float64, size int) (r, g, b, a float64) {
	for _, l := range layers {
		if size < l.minSize || l.s.dist(x, y) >= 0 {
			continue
		}
		o := l.opacity
		r = float64(l.c.r)/255*o + r*(1-o)
		g = float64(l.c.g)/255*o + g*(1-o)
		b = float64(l.c.b)/255*o + b*(1-o)
		a = o + a*(1-o)
	}
	return r, g, b, a
}

func to8(v float64) uint8 {
	return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255))
}

// encodePNG encodes with fixed settings so output bytes are reproducible.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeICO writes an ICO with one PNG-encoded entry per size (Windows
// Vista and later read PNG entries; a width or height of 0 means 256).
func encodeICO(sizes []int) ([]byte, error) {
	images := make([][]byte, len(sizes))
	for i, s := range sizes {
		if s < 1 || s > 256 {
			return nil, fmt.Errorf("icon size %d is outside 1 to 256", s)
		}
		data, err := encodePNG(renderPNG(s))
		if err != nil {
			return nil, err
		}
		images[i] = data
	}
	var buf bytes.Buffer
	le := binary.LittleEndian
	header := make([]byte, 6)
	le.PutUint16(header[0:], 0) // reserved
	le.PutUint16(header[2:], 1) // type: icon
	le.PutUint16(header[4:], uint16(len(sizes)))
	buf.Write(header)
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		e := make([]byte, 16)
		dim := byte(s)
		if s == 256 {
			dim = 0
		}
		e[0], e[1] = dim, dim // width, height
		e[2], e[3] = 0, 0     // palette colors, reserved
		le.PutUint16(e[4:], 1)
		le.PutUint16(e[6:], 32)
		le.PutUint32(e[8:], uint32(len(images[i])))
		le.PutUint32(e[12:], uint32(offset))
		buf.Write(e)
		offset += len(images[i])
	}
	for _, data := range images {
		buf.Write(data)
	}
	return buf.Bytes(), nil
}
