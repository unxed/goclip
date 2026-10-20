package goclip

import (
	"encoding/binary"
	"fmt"
	"image"
)

func decodeClipboardBMP(data []byte) (image.Image, error) {
	if len(data) < 54 || string(data[:2]) != "BM" {
		return nil, fmt.Errorf("invalid clipboard BMP")
	}
	offset := int(binary.LittleEndian.Uint32(data[10:]))
	header := int(binary.LittleEndian.Uint32(data[14:]))
	if header < 40 || header > len(data)-14 || offset < 14+header || offset > len(data) {
		return nil, fmt.Errorf("invalid clipboard BMP offsets")
	}
	// Preserve the DIB header and optional masks/palette; BMP offsets can also
	// include a gap that is not part of the DIB representation.
	end := 14 + header
	compression := binary.LittleEndian.Uint32(data[30:])
	if header == 40 && (compression == 3 || compression == 6) {
		end += 12
		if compression == 6 {
			end += 4
		}
	}
	palette := uint64(binary.LittleEndian.Uint32(data[46:])) * 4
	if uint64(end)+palette > uint64(offset) {
		return nil, fmt.Errorf("invalid clipboard BMP palette")
	}
	end += int(palette)
	dib := append([]byte(nil), data[14:end]...)
	dib = append(dib, data[offset:]...)
	return decodeClipboardDIB(dib)
}
