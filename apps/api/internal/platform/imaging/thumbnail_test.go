package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Left half red, right half blue: tells whether the picture was turned.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{R: 255, A: 255}
			if x >= w/2 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withOrientation inserts an EXIF APP1 segment (big-endian TIFF) right after SOI.
func withOrientation(data []byte, o uint16) []byte {
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, byte(o >> 8), byte(o), 0, 0, 0, 0, 0, 0, 0, 0}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	size := len(payload) + 2
	seg := append([]byte{0xFF, 0xE1, byte(size >> 8), byte(size)}, payload...)
	out := append([]byte{}, data[:2]...)
	out = append(out, seg...)
	return append(out, data[2:]...)
}

func decode(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestThumbnailScalesDown(t *testing.T) {
	out, err := Thumbnail(encodeJPEG(t, 1000, 500), 512)
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, out).Bounds(); b.Dx() != 512 || b.Dy() != 256 {
		t.Fatalf("size = %v, want 512x256", b.Size())
	}
}

func TestThumbnailKeepsSmallPictures(t *testing.T) {
	out, err := Thumbnail(encodeJPEG(t, 300, 200), 512)
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, out).Bounds(); b.Dx() != 300 || b.Dy() != 200 {
		t.Fatalf("size = %v, want 300x200", b.Size())
	}
}

func TestThumbnailTurnsByEXIF(t *testing.T) {
	// Orientation 6: the camera was turned clockwise; red (left) goes on top.
	out, err := Thumbnail(withOrientation(encodeJPEG(t, 400, 200), 6), 512)
	if err != nil {
		t.Fatal(err)
	}
	img := decode(t, out)
	if b := img.Bounds(); b.Dx() != 200 || b.Dy() != 400 {
		t.Fatalf("size = %v, want 200x400", b.Size())
	}
	r, _, bl, _ := img.At(100, 20).RGBA()
	if r < bl {
		t.Fatalf("top should be red after turning, got r=%d b=%d", r, bl)
	}
}

func TestThumbnailPNGGetsWhiteBackground(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10)) // fully transparent
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, err := Thumbnail(buf.Bytes(), 512)
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b, _ := decode(t, out).At(5, 5).RGBA(); r < 0xF000 || g < 0xF000 || b < 0xF000 {
		t.Fatalf("background = %d,%d,%d, want white", r, g, b)
	}
}

func TestThumbnailRejectsOtherFiles(t *testing.T) {
	if _, err := Thumbnail([]byte("%PDF-1.7 not a picture"), 512); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestExifOrientationWithoutExif(t *testing.T) {
	if o := exifOrientation(encodeJPEG(t, 10, 10)); o != 1 {
		t.Fatalf("orientation = %d, want 1", o)
	}
	if o := exifOrientation(withOrientation(encodeJPEG(t, 10, 10), 8)); o != 8 {
		t.Fatalf("orientation = %d, want 8", o)
	}
}
