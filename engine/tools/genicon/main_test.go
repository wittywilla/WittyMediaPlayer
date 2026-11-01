package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The icon in the repository is what this program draws.
func TestTheIconIsUpToDate(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "resources", "WittyMediaPlayer.ico"))
	if err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	if le.Uint16(data[2:]) != 1 {
		t.Fatal("that isn't an icon")
	}
	n := int(le.Uint16(data[4:]))
	if n < 5 {
		t.Fatalf("it has only %d sizes", n)
	}
	for i := 0; i < n; i++ {
		entry := data[6+16*i:]
		size := int(entry[0])
		if size == 0 {
			size = 256
		}
		length, offset := le.Uint32(entry[8:]), le.Uint32(entry[12:])
		img, err := png.Decode(bytes.NewReader(data[offset : offset+length]))
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		want := Image(size)
		if !sameImage(img, want) {
			t.Errorf("size %d differs: run go run ./tools/genicon ../resources/WittyMediaPlayer.ico", size)
		}
	}
}

func sameImage(a image.Image, b *image.NRGBA) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}

// The wizard's pictures are the sizes Windows Installer wants.
func TestBitmaps(t *testing.T) {
	dir := t.TempDir()
	if err := bitmaps(dir); err != nil {
		t.Fatal(err)
	}
	for name, size := range map[string][2]int32{"banner.bmp": {493, 58}, "welcome.bmp": {493, 312}} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		w, h := int32(binary.LittleEndian.Uint32(b[18:])), int32(binary.LittleEndian.Uint32(b[22:]))
		if b[0] != 'B' || b[1] != 'M' || w != size[0] || h != size[1] {
			t.Errorf("%s is %dx%d", name, w, h)
		}
	}
}
