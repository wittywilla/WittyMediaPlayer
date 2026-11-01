// Command genicon draws WittyMediaPlayer's icon, and the pictures the Windows installer's wizard
// shows. It's a build tool: nothing of it ships.
//
//	go run ./tools/genicon out.ico               the icon, at every size Windows asks for
//	go run ./tools/genicon out.png [size]        one size, as a picture
//	go run ./tools/genicon -bitmaps <folder>     banner.bmp and welcome.bmp
//
// resources/WittyMediaPlayer.ico is this program's output, kept in the repository so that a build
// of the window needs no Go. A test fails if the two ever differ.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: genicon <out.ico|out.png> [size] | -bitmaps <folder>")
		os.Exit(2)
	}
	if os.Args[1] == "-bitmaps" && len(os.Args) == 3 {
		check(bitmaps(os.Args[2]))
		return
	}
	out, size := os.Args[1], 256
	if len(os.Args) > 2 {
		size, _ = strconv.Atoi(os.Args[2])
	}
	var data []byte
	if strings.EqualFold(filepath.Ext(out), ".ico") {
		data = ICO()
	} else {
		data = PNG(size)
	}
	check(os.WriteFile(out, data, 0o644))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type rgb struct{ r, g, b float64 }

func lerp(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// Image draws the icon: a dark tile, a disc in a teal-to-blue sweep with the tile showing through
// a play triangle. Below 32 pixels the disc fills more of the tile, so it still reads.
func Image(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float64(size)
	cx, cy := s/2, s/2
	radius := s * 0.22
	disc := s * 0.36
	if size < 32 {
		disc = s * 0.42
	}
	tile := rgb{27, 30, 37}
	c1, c2 := rgb{52, 211, 190}, rgb{59, 123, 246}
	tx0, ty0 := cx-disc*0.34, cy-disc*0.50
	tx1, ty1 := cx+disc*0.58, cy
	tx2, ty2 := cx-disc*0.34, cy+disc*0.50
	inTri := func(x, y float64) bool {
		d1 := (x-tx1)*(ty0-ty1) - (tx0-tx1)*(y-ty1)
		d2 := (x-tx2)*(ty1-ty2) - (tx1-tx2)*(y-ty2)
		d3 := (x-tx0)*(ty2-ty0) - (tx2-tx0)*(y-ty0)
		neg := d1 < 0 || d2 < 0 || d3 < 0
		pos := d1 > 0 || d2 > 0 || d3 > 0
		return !(neg && pos)
	}
	const ss = 4
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var acc [4]float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := float64(x) + (float64(sx)+0.5)/ss
					py := float64(y) + (float64(sy)+0.5)/ss
					dx := math.Max(math.Abs(px-cx)-(s/2-radius), 0)
					dy := math.Max(math.Abs(py-cy)-(s/2-radius), 0)
					if math.Hypot(dx, dy) > radius {
						continue
					}
					c := tile
					if math.Hypot(px-cx, py-cy) <= disc && !inTri(px, py) {
						c = lerp(c1, c2, (px+py)/(2*s))
					}
					acc[0] += c.r
					acc[1] += c.g
					acc[2] += c.b
					acc[3] += 255
				}
			}
			if acc[3] == 0 {
				continue
			}
			covered := acc[3] / 255
			img.SetNRGBA(x, y, color.NRGBA{uint8(acc[0] / covered), uint8(acc[1] / covered), uint8(acc[2] / covered), uint8(acc[3] / (ss * ss))})
		}
	}
	return img
}

// PNG encodes the icon at one size.
func PNG(size int) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, Image(size))
	return b.Bytes()
}

// ICO holds the sizes Windows asks for, each as a PNG, which every Windows this runs on reads.
func ICO() []byte {
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	var head, body bytes.Buffer
	le := binary.LittleEndian
	for _, v := range []uint16{0, 1, uint16(len(sizes))} {
		_ = binary.Write(&head, le, v)
	}
	offset := 6 + 16*len(sizes)
	for _, s := range sizes {
		frame := PNG(s)
		dim := byte(s)
		if s >= 256 {
			dim = 0
		}
		head.Write([]byte{dim, dim, 0, 0})
		_ = binary.Write(&head, le, uint16(1))
		_ = binary.Write(&head, le, uint16(32))
		_ = binary.Write(&head, le, uint32(len(frame)))
		_ = binary.Write(&head, le, uint32(offset))
		offset += len(frame)
		body.Write(frame)
	}
	head.Write(body.Bytes())
	return head.Bytes()
}

// bitmaps draws the two pictures in the sizes Windows Installer's wizard wants: a banner along
// the top of a page, and the panel down the left of the first and last pages.
func bitmaps(dir string) error {
	white := image.NewUniform(color.White)
	panel := image.NewUniform(color.RGBA{0x1b, 0x1e, 0x25, 0xff})

	banner := image.NewRGBA(image.Rect(0, 0, 493, 58))
	draw.Draw(banner, banner.Bounds(), white, image.Point{}, draw.Src)
	drawIcon(banner, 44, 493-44-8, 7)

	welcome := image.NewRGBA(image.Rect(0, 0, 493, 312))
	draw.Draw(welcome, welcome.Bounds(), white, image.Point{}, draw.Src)
	draw.Draw(welcome, image.Rect(0, 0, 164, 312), panel, image.Point{}, draw.Src)
	drawIcon(welcome, 96, (164-96)/2, 40)

	if err := writeBMP(filepath.Join(dir, "banner.bmp"), banner); err != nil {
		return err
	}
	return writeBMP(filepath.Join(dir, "welcome.bmp"), welcome)
}

func drawIcon(dst draw.Image, size, x, y int) {
	draw.Draw(dst, image.Rect(x, y, x+size, y+size), Image(size), image.Point{}, draw.Over)
}

// writeBMP writes the 24-bit, bottom-up form Windows Installer takes.
func writeBMP(path string, img *image.RGBA) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	stride := (w*3 + 3) &^ 3
	var out bytes.Buffer
	size := 54 + stride*h
	out.WriteString("BM")
	le := binary.LittleEndian
	for _, v := range []any{uint32(size), uint32(0), uint32(54), uint32(40), int32(w), int32(h), uint16(1), uint16(24), uint32(0), uint32(stride * h), int32(2835), int32(2835), uint32(0), uint32(0)} {
		if err := binary.Write(&out, le, v); err != nil {
			return err
		}
	}
	row := make([]byte, stride)
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, y)
			row[x*3], row[x*3+1], row[x*3+2] = c.B, c.G, c.R
		}
		out.Write(row)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}
