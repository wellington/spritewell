package spritewell

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/url"
	"regexp"
	"unicode/utf8"
)

func imageType(r io.Reader) (string, error) {
	_, str, err := image.DecodeConfig(r)
	if err != nil {
		return "", err
	}
	return str, nil
}

// IsSVG attempts to determine if a reader contains an SVG
func IsSVG(r io.Reader) bool {

	// Copy first 1k block and look for <svg
	var buf bytes.Buffer
	io.CopyN(&buf, r, bytes.MinRead)

	s := bufio.NewScanner(&buf)
	s.Split(bufio.ScanWords)
	mat := []byte("<svg")
	for s.Scan() {
		if bytes.Equal(mat, s.Bytes()) {
			return true
		}
		// Guesstimate that SVG with non-utf8 is no SVG at all
		if !utf8.Valid(s.Bytes()) {
			return false
		}
	}
	return false
}

// Inline accepts an io.Reader and returns a byte array of
// base64 encoded binary data or optionally base64 encodes
// svg data.
func Inline(r io.Reader, w io.Writer, encode ...bool) error {

	m, ext, err := image.Decode(r)
	_ = ext
	if err != nil {
		return err
	}

	// Check if SVG
	var buf bytes.Buffer
	tr := io.TeeReader(r, &buf)
	if IsSVG(tr) {
		mr := io.MultiReader(&buf, r)
		_ = mr
		return nil
	}

	err = png.Encode(w, m)
	return err
}

// InlineSVG accepts a byte slice and returns a valid utf8 svg+xml bytes.
// Any invalid utf8 runes are removed, unnecessary newline and whitespace
// are removed from the input.  This encoding is more error prone, but uses
// less space.
func InlineSVG(in []byte) []byte {
	out := []byte(`url("data:image/svg+xml;utf8,`)
	new := inlineSVG(in, false)
	out = append(out, new...)
	out = append(out, []byte(`")`)...)

	return out
}

// InlineSVG accepts a byte slice and returns a base64 byte slice
// compatible with image/svg+xml;base64
func InlineSVGBase64(in []byte) []byte {
	enc := inlineSVG(in, true)
	out := make([]byte, 0, len(enc)+40)
	out = []byte(`url("data:image/svg+xml;base64,`)
	out = append(out, enc...)
	out = append(out, []byte(`")"`)...)
	return out
}

// inlinesvg returns a byte slice that is utf8 compliant or base64
// encoded
func inlineSVG(in []byte, encode bool) []byte {
	if encode {
		enc := make([]byte, base64.StdEncoding.EncodedLen(len(in)))
		base64.StdEncoding.Encode(enc, in)
		return enc
	}

	// Strip unnecessary whitespace and newlines
	input := bytes.Replace(in, []byte("\r\n"), []byte(""), -1)

	reg := regexp.MustCompile(`>\\s+<`)
	input = reg.ReplaceAll(input, []byte("><"))

	// URL encode the string before return it
	u := &url.URL{Path: string(input)}
	encodedPath := u.String()
	return []byte(encodedPath)
}
