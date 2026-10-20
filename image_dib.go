package goclip

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math/bits"
)

// decodeClipboardDIB handles uncompressed 24/32-bit BITMAPINFO/V4/V5 data,
// including top-down rows and explicitly declared bitfield/alpha masks.
func decodeClipboardDIB(data []byte) (image.Image, error) {
	if len(data) < 40 {
		return nil, fmt.Errorf("truncated clipboard DIB header")
	}
	header := int(binary.LittleEndian.Uint32(data))
	width := int64(int32(binary.LittleEndian.Uint32(data[4:])))
	height := int64(int32(binary.LittleEndian.Uint32(data[8:])))
	topDown := height < 0
	if topDown {
		height = -height
	}
	depth := binary.LittleEndian.Uint16(data[14:])
	compression := binary.LittleEndian.Uint32(data[16:])
	if header < 40 || header > len(data) || width <= 0 || height <= 0 || width*height > maxImagePixels || binary.LittleEndian.Uint16(data[12:]) != 1 {
		return nil, fmt.Errorf("invalid clipboard DIB dimensions/header")
	}
	if (depth != 24 && depth != 32) || (compression != 0 && compression != 3 && compression != 6) || (depth == 24 && compression != 0) {
		return nil, fmt.Errorf("unsupported clipboard DIB encoding")
	}
	offset := header
	masks := [4]uint32{0x00ff0000, 0x0000ff00, 0x000000ff, 0}
	if compression == 3 || compression == 6 || header >= 108 {
		start := 40
		count := 3
		if header >= 56 || compression == 6 {
			count = 4
		}
		if header == 40 {
			offset += count * 4
		}
		if start+count*4 > len(data) {
			return nil, fmt.Errorf("truncated clipboard DIB masks")
		}
		for i := 0; i < count; i++ {
			masks[i] = binary.LittleEndian.Uint32(data[start+i*4:])
		}
		if masks[0]|masks[1]|masks[2] == 0 {
			masks = [4]uint32{0x00ff0000, 0x0000ff00, 0x000000ff, 0}
		}
	}
	palette := int64(binary.LittleEndian.Uint32(data[32:])) * 4
	stride := ((width*int64(depth) + 31) / 32) * 4
	offset64 := int64(offset) + palette
	if offset64+stride*height > int64(len(data)) {
		return nil, fmt.Errorf("truncated clipboard DIB pixels")
	}
	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	channel := func(pixel, mask uint32) uint8 {
		if mask == 0 {
			return 0
		}
		shift := bits.TrailingZeros32(mask)
		value := uint64((pixel & mask) >> shift)
		maximum := uint64(mask >> shift)
		return uint8(value * 255 / maximum)
	}
	for y := 0; y < int(height); y++ {
		row := y
		if !topDown {
			row = int(height) - 1 - y
		}
		for x := 0; x < int(width); x++ {
			pos := int(offset64 + int64(row)*stride + int64(x)*int64(depth/8))
			c := color.NRGBA{R: data[pos+2], G: data[pos+1], B: data[pos], A: 255}
			if depth == 32 {
				pixel := binary.LittleEndian.Uint32(data[pos:])
				c.R, c.G, c.B = channel(pixel, masks[0]), channel(pixel, masks[1]), channel(pixel, masks[2])
				if masks[3] != 0 {
					c.A = channel(pixel, masks[3])
				}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img, nil
}
