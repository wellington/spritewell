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
	"unicode/utf8"
)

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

	// Check if SVG
	var buf bytes.Buffer
	tr := io.TeeReader(r, &buf)
	if IsSVG(tr) {
		enc := len(encode) > 0 && encode[0]
		mr := io.MultiReader(&buf, r)
		inlineSVG(w, mr, enc)
		return nil
	}
	mr := io.MultiReader(&buf, r)
	m, _, err := image.Decode(mr)
	if err != nil {
		return err
	}

	err = png.Encode(w, m)
	return err
}

// InlineSVG accepts a byte slice and returns a valid utf8 svg+xml bytes.
// Any invalid utf8 runes are removed, unnecessary newline and whitespace
// are removed from the input.  This encoding is more error prone, but uses
// less space.
func InlineSVG(r io.Reader, w io.Writer) {
	w.Write([]byte(`url("data:image/svg+xml;utf8,`))
	io.Copy(w, r)
	w.Write([]byte(`")`))
}

// InlineSVG accepts a byte slice and returns a base64 byte slice
// compatible with image/svg+xml;base64
func InlineSVGBase64(in []byte) []byte {
	/*enc := inlineSVG(in, true)
	out := make([]byte, 0, len(enc)+40)
	out = []byte(`url("data:image/svg+xml;base64,`)
	out = append(out, enc...)
	out = append(out, []byte(`")"`)...)
	return out*/
	return []byte{}
}

// inlinesvg returns a byte slice that is utf8 compliant or base64
// encoded
func inlineSVG(w io.Writer, r io.Reader, encode bool) {
	if encode {
		bw := base64.NewEncoder(base64.StdEncoding, w)
		w.Write([]byte(`url("data:image/svg+xml;base64,`))
		io.Copy(bw, r)
		w.Write([]byte(`")`))
		return
	}

	w.Write([]byte(`url("data:image/svg+xml;utf8,`))
	// Exhaust the buffer and do some sloppy regex stuff to the input
	// TODO: convert this to streaming reader
	var buf bytes.Buffer
	buf.ReadFrom(r)

	// // Strip unnecessary whitespace and newlines
	input := bytes.Replace(buf.Bytes(), []byte("\r\n"), []byte(""), -1)
	// input = bytes.Replace(input, []byte(`"`), []byte("'"), -1)
	// reg := regexp.MustCompile(`>\\s+<`)
	// input = reg.ReplaceAll(input, []byte("><"))

	u := &url.URL{Path: string(input)}
	encodedPath := []byte(u.String())

	w.Write(encodedPath)
	w.Write([]byte(`")`))
}
