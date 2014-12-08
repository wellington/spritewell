package spritewell

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
)

var formats = []string{"png", "gif", "jpg"}

type GoImages []image.Image
type ImageList struct {
	bytes.Buffer
	GoImages
	BuildDir, ImageDir, GenImgDir string
	Out                           draw.Image
	OutFile                       string
	Combined                      bool
	Globs, Paths                  []string
	Vertical                      bool
}

// SafeImageMap provides a thread-safe data structure for
// creating maps of ImageLists
type SafeImageMap struct {
	sync.RWMutex
	M map[string]ImageList
}

func funnyNames() string {

	names := []string{"White_and_Nerdy",
		"Fat",
		"Eat_It",
		"Foil",
		"Like_a_Surgeon",
		"Amish_Paradise",
		"The_Saga_Begins",
		"Polka_Face"}
	return names[mrand.Intn(len(names))]
}

// String generates CSS path to output file
func (l ImageList) String() string {
	paths := ""
	for _, path := range l.Paths {
		path += strings.TrimSuffix(filepath.Base(path),
			filepath.Ext(path)) + " "
	}
	return paths
}

// Return relative path to File
// TODO: Return abs path to file
func (l ImageList) File(f string) string {
	pos := l.Lookup(f)
	if pos > -1 {
		return l.Paths[pos]
	}
	return ""
}

func (l ImageList) Lookup(f string) int {
	var base string
	pos := -1
	for i, v := range l.Paths {
		base = filepath.Base(v)
		base = strings.TrimSuffix(base, filepath.Ext(v))
		if f == v {
			pos = i
			//Do partial matches, for now
		} else if f == base {
			pos = i
		}
	}

	if pos > -1 {
		if l.GoImages[pos] != nil {
			return pos
		}
	}
	// TODO: Find a better way to send these to cli so tests
	// aren't impacted.
	// Debug.Printf("File not found: %s\n Try one of %s", f, l)

	return -1
}

// Return the X position of an image based
// on the layout (vertical/horizontal) and
// position in Image slice
func (l ImageList) X(pos int) int {
	x := 0
	if pos > len(l.GoImages) {
		return -1
	}
	if l.Vertical {
		return 0
	}
	for i := 0; i < pos; i++ {
		x += l.ImageWidth(i)
	}
	return x
}

// Return the Y position of an image based
// on the layout (vertical/horizontal) and
// position in Image slice
func (l ImageList) Y(pos int) int {
	y := 0
	if pos > len(l.GoImages) {
		return -1
	}
	if !l.Vertical {
		return 0
	}
	for i := 0; i < pos; i++ {
		y += l.ImageHeight(i)
	}
	return y
}

// Map builds a sass-map with the information contained in ImageList.
// This is deprecated.
func (l ImageList) Map(name string) string {
	var res []string
	rel, _ := filepath.Rel(l.BuildDir, l.GenImgDir)
	for i := range l.GoImages {
		base := strings.TrimSuffix(filepath.Base(l.Paths[i]),
			filepath.Ext(l.Paths[i]))
		res = append(res, fmt.Sprintf(
			"%s: map_merge(%s,(%s: (width: %d, height: %d, "+
				"x: %d, y: %d, url: '%s')))",
			name, name,
			base, l.ImageWidth(i), l.ImageHeight(i),
			l.X(i), l.Y(i), filepath.Join(rel, l.OutFile),
		))
	}
	return "(); " + strings.Join(res, "; ") + ";"
}

func (l ImageList) CSS(s string) string {
	pos := l.Lookup(s)
	if pos == -1 {
		log.Printf("File not found: %s\n Try one of: %s",
			s, l)
		return ""
	}

	return fmt.Sprintf(`url("%s") %s`,
		l.OutFile, l.Position(s))
}

func (l ImageList) Position(s string) string {
	pos := l.Lookup(s)
	if pos == -1 {
		log.Printf("File not found: %s\n Try one of: %s",
			s, l)
		return ""
	}

	return fmt.Sprintf(`%dpx %dpx`, -l.X(pos), -l.Y(pos))
}

func (l ImageList) Dimensions(s string) string {
	if pos := l.Lookup(s); pos > -1 {

		return fmt.Sprintf("width: %dpx;\nheight: %dpx",
			l.ImageWidth(pos), l.ImageHeight(pos))
	}
	return ""
}

func (l ImageList) inline() []byte {

	r, w := io.Pipe()
	go func(w io.WriteCloser) {
		err := png.Encode(w, l.GoImages[0])
		if err != nil {
			panic(err)
		}
		w.Close()
	}(w)
	var scanned []byte
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanBytes)
	for scanner.Scan() {
		scanned = append(scanned, scanner.Bytes()...)
	}
	return scanned
}

// Inline creates base64 encoded string of the underlying
// image data blog
func (l ImageList) Inline() string {
	encstr := base64.StdEncoding.EncodeToString(l.inline())
	return fmt.Sprintf("url('data:image/png;base64,%s')", encstr)
}

func (l ImageList) SImageWidth(s string) int {
	if pos := l.Lookup(s); pos > -1 {
		return l.ImageWidth(pos)
	}
	return -1
}

func (l ImageList) ImageWidth(pos int) int {
	if pos > len(l.GoImages) || pos < 0 {
		return -1
	}

	return l.GoImages[pos].Bounds().Dx()
}

func (l ImageList) SImageHeight(s string) int {
	if pos := l.Lookup(s); pos > -1 {
		return l.ImageHeight(pos)
	}
	return -1
}

func (l ImageList) ImageHeight(pos int) int {
	if pos > len(l.GoImages) || pos < 0 {
		return -1
	}
	return l.GoImages[pos].Bounds().Dy()
}

// Return the cumulative Height of the
// image slice.
func (l *ImageList) Height() int {
	h := 0
	ll := *l

	for pos, _ := range ll.GoImages {
		if l.Vertical {
			h += ll.ImageHeight(pos)
		} else {
			h = int(math.Max(float64(h), float64(ll.ImageHeight(pos))))
		}
	}
	return h
}

// Return the cumulative Width of the
// image slice.
func (l *ImageList) Width() int {
	w := 0

	for pos, _ := range l.GoImages {
		if !l.Vertical {
			w += l.ImageWidth(pos)
		} else {
			w = int(math.Max(float64(w), float64(l.ImageWidth(pos))))
		}
	}
	return w
}

// Build an output file location based on
// [genimagedir|location of file matched by glob] + glob pattern
func (l *ImageList) OutputPath() (string, error) {
	// This only looks at the first glob pattern
	globpath := l.Globs[0]
	path := filepath.Dir(globpath)
	if path == "." {
		path = "image"
	}
	path = strings.Replace(path, "/", "", -1)
	ext := filepath.Ext(globpath)
	// Remove invalid characters from path
	path = strings.Replace(path, "*", "", -1)
	hasher := md5.New()
	hasher.Write(l.Buffer.Bytes())
	salt := hex.EncodeToString(hasher.Sum(nil))[:6]
	l.OutFile = path + "-" + salt + ext
	return l.OutFile, nil
}

// Decode accepts a variable number of glob patterns.  The ImageDir
// is assumed to be prefixed to the globs provided.
func (l *ImageList) Decode(rest ...string) error {

	// Invalidate the composite cache
	l.Out = nil
	var (
		paths []string
	)
	absImageDir, _ := filepath.Abs(l.ImageDir)
	for _, r := range rest {
		matches, err := filepath.Glob(filepath.Join(l.ImageDir, r))
		if err != nil {
			panic(err)
		}
		if len(matches) == 0 {
			// No matches found, try appending * and trying again
			// This supports the case "139" > "139.jpg" "139.png" etc.
			matches, err = filepath.Glob(filepath.Join(l.ImageDir, r+"*"))
			if err != nil {
				panic(err)
			}
		}
		rel := make([]string, len(matches))
		for i := range rel {
			// Attempt both relative and absolute to path
			if p, err := filepath.Rel(l.ImageDir, matches[i]); err == nil {
				rel[i] = p
			} else if p, err := filepath.Rel(absImageDir, matches[i]); err == nil {
				rel[i] = p
			}
		}
		l.Paths = append(l.Paths, rel...)
		paths = append(paths, matches...)
	}
	// turn paths into relative paths to the files

	l.Globs = paths
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		goimg, _, err := image.Decode(f)
		if err != nil {
			ext := filepath.Ext(path)
			if !CanDecode(ext) {
				return fmt.Errorf("format: %s not supported", ext)
			} else {
				return fmt.Errorf("Error processing: %s\n%s", path, err)
			}
		}
		l.GoImages = append(l.GoImages, goimg)
	}

	if len(l.Paths) == 0 {
		log.Printf("No images were found for glob: %v",
			rest,
		)
	}

	return nil
}

// CanDecode checks if the file extension is supported by
// spritewell.
func CanDecode(ext string) bool {
	for i := range formats {
		if ext == formats[i] {
			return true
		}
	}
	return false
}

// Combine all images in the slice into a final output
// image.
func (l *ImageList) Combine() (string, error) {

	var (
		maxW, maxH int
	)

	if l.Out != nil {
		return "", errors.New("Sprite is empty")
	}

	maxW, maxH = l.Width(), l.Height()
	curH, curW := 0, 0

	goimg := image.NewRGBA(image.Rect(0, 0, maxW, maxH))
	l.Out = goimg
	for _, img := range l.GoImages {

		draw.Draw(goimg, goimg.Bounds(), img,
			image.Point{
				X: curW,
				Y: curH,
			}, draw.Src)

		if l.Vertical {
			curH -= img.Bounds().Dy()
		} else {
			curW -= img.Bounds().Dx()
		}
	}
	l.Combined = true

	// Set the buf so bytes.Buffer works
	err := png.Encode(&l.Buffer, goimg)
	if err != nil {
		return "", err
	}
	return l.OutputPath()
}

func randString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

// Export saves out the ImageList to the specified file
func (l *ImageList) Export() (string, error) {
	// Use the auto generated path if none is specified
	// TODO: Differentiate relative file path (in css) to this abs one
	abs := filepath.Join(l.GenImgDir, filepath.Base(l.OutFile))
	if _, err := os.Stat(abs); err == nil {
		return abs, nil
	}

	fo, err := os.Create(abs)
	if err != nil {
		log.Printf("Failed to create file: %s\n", abs)
		log.Print(err)
		return "", err
	}
	//This call is cached if already run
	l.Combine()
	defer fo.Close()

	n, err := io.Copy(fo, &l.Buffer)
	if n == 0 {
		log.Fatalf("no bytes written of: %d", l.Buffer.Len())
	}
	if err != nil {
		log.Printf("Failed to create: %s\n%s", abs, err)
		return "", err
	}
	// log.Print("Created file: ", abs)
	return abs, nil
}
