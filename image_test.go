package goclip

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

type imageTestDriver struct {
	img image.Image
	err error
}

func (*imageTestDriver) Name() string                                     { return "test-image" }
func (*imageTestDriver) Available() bool                                  { return true }
func (*imageTestDriver) ReadText() (string, error)                        { return "", ErrEmpty }
func (*imageTestDriver) WriteText(string) error                           { return nil }
func (*imageTestDriver) Clear() error                                     { return nil }
func (d *imageTestDriver) ReadImage(context.Context) (image.Image, error) { return d.img, d.err }

func TestReadImageOptionalDriver(t *testing.T) {
	old := ActiveDriver()
	defer SetActiveDriver(old)
	SetActiveDriver(NewFileDriver(""))
	if _, err := ReadImage(context.Background()); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	SetActiveDriver(&imageTestDriver{img: img})
	if got, err := ReadImage(context.Background()); err != nil || got != img {
		t.Fatal(got, err)
	}
	SetActiveDriver(&imageTestDriver{err: ErrNoImage})
	if _, err := ReadImage(context.Background()); !errors.Is(err, ErrNoImage) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadImage(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if text, err := ReadImageText(context.Background()); err != nil || text != "" {
		t.Fatal(text, err)
	}
}

func TestClipboardPNGPreservesAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 91, G: 52, B: 13, A: 77})
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	got, err := decodeClipboardImage(data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if color.NRGBAModel.Convert(got.At(0, 0)) != img.NRGBAAt(0, 0) {
		t.Fatal("alpha lost")
	}
	if _, err := decodeClipboardImage([]byte("bad")); err == nil {
		t.Fatal("corrupt image accepted")
	}
}

func dibFixture(depth uint16, topDown bool) []byte {
	header := 40
	if depth == 32 {
		header = 124
	}
	data := make([]byte, header+8)
	binary.LittleEndian.PutUint32(data, uint32(header))
	binary.LittleEndian.PutUint32(data[4:], 1)
	height := int32(2)
	if topDown {
		height = -2
	}
	binary.LittleEndian.PutUint32(data[8:], uint32(height))
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], depth)
	if depth == 32 {
		binary.LittleEndian.PutUint32(data[16:], 3)
		for i, mask := range []uint32{0xff0000, 0xff00, 0xff, 0xff000000} {
			binary.LittleEndian.PutUint32(data[40+i*4:], mask)
		}
	}
	copy(data[header:], []byte{10, 20, 30, 77, 40, 50, 60, 99})
	return data
}

func TestClipboardDIBRowsAndAlpha(t *testing.T) {
	for _, depth := range []uint16{24, 32} {
		for _, topDown := range []bool{false, true} {
			got, err := decodeClipboardDIB(dibFixture(depth, topDown))
			if err != nil {
				t.Fatal(err)
			}
			want := color.NRGBA{R: 60, G: 50, B: 40, A: 255}
			if topDown {
				want = color.NRGBA{R: 30, G: 20, B: 10, A: 255}
			}
			if depth == 32 {
				want.A = 99
				if topDown {
					want.A = 77
				}
			}
			if color.NRGBAModel.Convert(got.At(0, 0)) != want {
				t.Fatalf("depth=%d topDown=%v: %v want %v", depth, topDown, got.At(0, 0), want)
			}
		}
	}
	for _, data := range [][]byte{nil, make([]byte, 40), dibFixture(24, false)[:42]} {
		if _, err := decodeClipboardDIB(data); err == nil {
			t.Fatal("invalid DIB accepted")
		}
	}
}
