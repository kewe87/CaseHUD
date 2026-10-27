package panel

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
)

// encodePNGFast writes a straight-alpha picture as an 8-bit RGBA PNG without
// row filters. The standard encoder spends about half its time choosing
// filters; the overlays are mostly transparent, which the fast deflate level
// packs well without them.
func encodePNGFast(img *image.NRGBA) ([]byte, error) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(kind string, data []byte) {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(data)))
		out.Write(n[:])
		start := out.Len()
		out.WriteString(kind)
		out.Write(data)
		binary.BigEndian.PutUint32(n[:], crc32.ChecksumIEEE(out.Bytes()[start:]))
		out.Write(n[:])
	}
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8], ihdr[9] = 8, 6 // 8 bits, RGBA
	chunk("IHDR", ihdr[:])

	var z bytes.Buffer
	zw, err := zlib.NewWriterLevel(&z, zlib.BestSpeed)
	if err != nil {
		return nil, err
	}
	row := make([]byte, 1+w*4) // filter type 0 (none) + the pixels
	for y := 0; y < h; y++ {
		copy(row[1:], img.Pix[y*img.Stride:y*img.Stride+w*4])
		if _, err := zw.Write(row); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	chunk("IDAT", z.Bytes())
	chunk("IEND", nil)
	return out.Bytes(), nil
}
