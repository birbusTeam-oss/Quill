package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"math"
)

// generateIcon creates a 16x16 ICO with a speech bubble / mouth icon — the Yappie brand.
func generateIcon() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))

	// Clear transparent
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 0})
		}
	}

	purple := color.RGBA{139, 92, 246, 255}       // Yappie purple
	purpleLight := color.RGBA{167, 139, 250, 255}  // Lighter purple
	white := color.RGBA{255, 255, 255, 255}

	// Draw speech bubble body (rounded rectangle 2,1 to 13,10)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			// Rounded rect: center at (7.5, 5), half-size (5.5, 4.5), radius 3
			dx := math.Abs(float64(x)-7.5) - 2.5
			dy := math.Abs(float64(y)-5.0) - 1.5
			if dx < 0 { dx = 0 }
			if dy < 0 { dy = 0 }
			dist := math.Sqrt(dx*dx+dy*dy) - 3.0
			if dist < -0.5 {
				img.Set(x, y, purple)
			} else if dist < 0.5 {
				a := uint8(255 * (0.5 - dist))
				img.Set(x, y, color.RGBA{purple.R, purple.G, purple.B, a})
			}
		}
	}

	// Speech bubble tail (bottom-left triangle)
	tailPixels := [][2]int{
		{4, 11}, {5, 11}, {6, 11},
		{3, 12}, {4, 12}, {5, 12},
		{2, 13}, {3, 13},
	}
	for _, p := range tailPixels {
		img.Set(p[0], p[1], purple)
	}

	// Sound waves (right side) — two arcs
	for _, wave := range []struct{ cx, r float64; col color.RGBA }{
		{13.0, 2.5, purpleLight},
		{13.0, 4.5, purpleLight},
	} {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				dx := float64(x) - wave.cx
				dy := float64(y) - 5.0
				dist := math.Sqrt(dx*dx+dy*dy)
				if dx > 0 && math.Abs(dist-wave.r) < 0.7 && math.Abs(dy) < wave.r*0.7 {
					a := uint8(180 * (1.0 - math.Abs(dist-wave.r)/0.7))
					existing := img.RGBAAt(x, y)
					if existing.A == 0 {
						img.Set(x, y, color.RGBA{wave.col.R, wave.col.G, wave.col.B, a})
					}
				}
			}
		}
	}

	// Three dots inside bubble (ellipsis — "talking")
	dotCenters := [][2]int{{5, 5}, {8, 5}, {11, 5}}
	for _, dc := range dotCenters {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				dx := float64(x) - float64(dc[0])
				dy := float64(y) - float64(dc[1])
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist < 1.2 {
					img.Set(x, y, white)
				}
			}
		}
	}

	return encodeICO(img)
}

// encodeICO creates a minimal .ico file from a 16x16 RGBA image.
func encodeICO(img *image.RGBA) []byte {
	w := 16
	h := 16

	// BMP pixel data (bottom-up, BGRA)
	var pixelData bytes.Buffer
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			pixelData.WriteByte(byte(b >> 8))
			pixelData.WriteByte(byte(g >> 8))
			pixelData.WriteByte(byte(r >> 8))
			pixelData.WriteByte(byte(a >> 8))
		}
	}

	// AND mask
	andMaskRowBytes := ((w + 31) / 32) * 4
	andMask := make([]byte, andMaskRowBytes*h)

	bmpInfoSize := 40
	pixelDataLen := pixelData.Len()
	andMaskLen := len(andMask)
	imageSize := bmpInfoSize + pixelDataLen + andMaskLen

	var buf bytes.Buffer

	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))

	buf.WriteByte(byte(w))
	buf.WriteByte(byte(h))
	buf.WriteByte(0)
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(32))
	binary.Write(&buf, binary.LittleEndian, uint32(imageSize))
	binary.Write(&buf, binary.LittleEndian, uint32(22))

	binary.Write(&buf, binary.LittleEndian, uint32(bmpInfoSize))
	binary.Write(&buf, binary.LittleEndian, int32(w))
	binary.Write(&buf, binary.LittleEndian, int32(h*2))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(32))
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(pixelDataLen+andMaskLen))
	binary.Write(&buf, binary.LittleEndian, int32(0))
	binary.Write(&buf, binary.LittleEndian, int32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	buf.Write(pixelData.Bytes())
	buf.Write(andMask)

	return buf.Bytes()
}
