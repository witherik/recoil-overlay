//go:build windows && amd64

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"runtime"
	"unsafe"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/sys/windows"
)

// Practice draws a premultiplied alpha bitmap into a native layered window
// placed exactly over the (hidden) Wails window; the OS then handles true
// click-through. See setPracticeWindow for why the Wails window can't be used.
var gdi32 = windows.NewLazySystemDLL("gdi32.dll")
var overlayFont, _ = opentype.Parse(goregular.TTF)

// Vertical midpoint of the arrows, in unscaled pixels from the window top. The
// frontend stage uses the same anchor.
const arrowAnchorY = 142.0

var practiceFrameValid bool
var lastPracticeFrame frameKey

type frameKey struct {
	Settings          Settings
	Width, Height     int
	DPI               uint32
	Elapsed           int64
	Direction, Status string
	Armed             bool
}
type point struct{ X, Y int32 }
type bitmapHeader struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	XPels, YPels           int32
	Used, Important        uint32
}
type bitmapInfo struct {
	Header bitmapHeader
	Colors uint32
}
type blendFunction struct{ Operation, Flags, Alpha, Format byte }

func practiceStatus(s Snapshot) string {
	if !s.Armed {
		return "DISABLED  /  F9 TO ENABLE"
	}
	if !s.Focused {
		return "WAITING FOR APEX  /  F8 TO EDIT"
	}
	if s.Held && !s.Running {
		return "RELEASE TO RESET"
	}
	return "F8 EDIT  /  F9 DISABLE"
}
func renderPractice(s Snapshot) error {
	if practiceWindow == 0 {
		return fmt.Errorf("overlay window unavailable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rect := practiceRect
	width, height := int(rect.Right-rect.Left), int(rect.Bottom-rect.Top)
	if width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return fmt.Errorf("invalid overlay dimensions")
	}
	dpi := practiceDPI
	if dpi == 0 {
		dpi = 96
	}
	key := frameKey{s.Settings, width, height, uint32(dpi), s.ElapsedMS, s.Direction, practiceStatus(s), s.Armed}
	if practiceFrameValid && key == lastPracticeFrame {
		return nil
	}
	surface := drawPractice(s, width, height, float64(dpi)/96)
	dc, _, err := gdi32.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		return fmt.Errorf("create overlay canvas: %w", err)
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	info := bitmapInfo{Header: bitmapHeader{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32}}
	var pixels unsafe.Pointer
	bitmap, _, err := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if bitmap == 0 || pixels == nil {
		return fmt.Errorf("create overlay bitmap: %w", err)
	}
	defer gdi32.NewProc("DeleteObject").Call(bitmap)
	old, _, _ := gdi32.NewProc("SelectObject").Call(dc, bitmap)
	defer gdi32.NewProc("SelectObject").Call(dc, old)
	dest := unsafe.Slice((*byte)(pixels), width*height*4)
	// Go RGBA and Win32 BGRA both contain premultiplied channels.
	for i := 0; i < len(dest); i += 4 {
		dest[i] = surface.Pix[i+2]
		dest[i+1] = surface.Pix[i+1]
		dest[i+2] = surface.Pix[i]
		dest[i+3] = surface.Pix[i+3]
	}
	size := point{int32(width), int32(height)}
	origin := point{}
	position := point{rect.Left, rect.Top}
	blend := blendFunction{Alpha: byte(s.Settings.Opacity * 255 / 100), Format: 1}
	ok, _, err := user32.NewProc("UpdateLayeredWindow").Call(practiceWindow, 0, uintptr(unsafe.Pointer(&position)), uintptr(unsafe.Pointer(&size)), dc, uintptr(unsafe.Pointer(&origin)), 0, uintptr(unsafe.Pointer(&blend)), 2)
	if ok == 0 {
		return fmt.Errorf("paint transparent overlay: %w", err)
	}
	lastPracticeFrame = key
	practiceFrameValid = true
	return nil
}

type canvas struct {
	image *image.RGBA
	scale float64
}

func (c canvas) rect(x, y, w, h float64, col color.NRGBA) {
	r := image.Rect(int(math.Round(x*c.scale)), int(math.Round(y*c.scale)), int(math.Round((x+w)*c.scale)), int(math.Round((y+h)*c.scale)))
	draw.Draw(c.image, r, image.NewUniform(col), image.Point{}, draw.Src)
}
func (c canvas) text(x, y, size float64, text string, col color.NRGBA) {
	face, err := opentype.NewFace(overlayFont, &opentype.FaceOptions{Size: size * c.scale, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer face.Close()
	d := font.Drawer{Dst: c.image, Src: image.NewUniform(col), Face: face, Dot: fixed.P(int(math.Round(x*c.scale)), int(math.Round(y*c.scale)))}
	d.DrawString(text)
}
func (c canvas) centeredText(center, y, size float64, text string, col color.NRGBA) {
	face, err := opentype.NewFace(overlayFont, &opentype.FaceOptions{Size: size * c.scale, DPI: 72})
	if err != nil {
		return
	}
	defer face.Close()
	width := font.MeasureString(face, text).Ceil()
	c.text(center-float64(width)/(2*c.scale), y, size, text, col)
}
func (c canvas) line(x1, y1, x2, y2, width float64, col color.NRGBA) {
	x1 *= c.scale
	y1 *= c.scale
	x2 *= c.scale
	y2 *= c.scale
	r := width * c.scale / 2
	bounds := image.Rect(int(math.Floor(math.Min(x1, x2)-r-1)), int(math.Floor(math.Min(y1, y2)-r-1)), int(math.Ceil(math.Max(x1, x2)+r+1)), int(math.Ceil(math.Max(y1, y2)+r+1))).Intersect(c.image.Bounds())
	dx, dy := x2-x1, y2-y1
	length := dx*dx + dy*dy
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			t := 0.0
			if length > 0 {
				t = ((float64(x)+.5-x1)*dx + (float64(y)+.5-y1)*dy) / length
				t = math.Max(0, math.Min(1, t))
			}
			distance := math.Hypot(float64(x)+.5-x1-t*dx, float64(y)+.5-y1-t*dy)
			coverage := math.Max(0, math.Min(1, r+.5-distance))
			if coverage > 0 {
				a := uint8(float64(col.A) * coverage)
				p := color.NRGBA{col.R, col.G, col.B, a}
				c.image.Set(x, y, p)
			}
		}
	}
}
func drawPractice(s Snapshot, width, height int, scale float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	c := canvas{img, scale}
	center := float64(width) / scale / 2
	mint := color.NRGBA{112, 227, 192, 255}
	coral := color.NRGBA{244, 161, 140, 255}
	muted := color.NRGBA{82, 101, 105, 230}
	text := color.NRGBA{220, 232, 233, 255}
	dim := color.NRGBA{139, 161, 164, 255}
	size := float64(s.Settings.ArrowSize)
	gap := float64(s.Settings.Gap)
	for _, direction := range []string{"left", "right"} {
		x := center - (gap+size)/2
		if direction == "right" {
			x = center + (gap+size)/2
		}
		col := muted
		if s.Armed && s.Direction == direction {
			if direction == "right" {
				col = mint
			} else {
				col = coral
			}
		}
		px := func(v float64) float64 { return x - size/2 + v*size/64 }
		py := func(v float64) float64 { return arrowAnchorY - size/2 + v*size/64 }
		coords := [][4]float64{{37, 12, 17, 32}, {17, 32, 37, 52}, {18, 32, 52, 32}}
		if direction == "right" {
			coords = [][4]float64{{27, 12, 47, 32}, {47, 32, 27, 52}, {46, 32, 12, 32}}
		}
		for _, l := range coords {
			c.line(px(l[0]), py(l[1])+1, px(l[2]), py(l[3])+1, 9*size/64, color.NRGBA{0, 0, 0, 170})
		}
		for _, l := range coords {
			c.line(px(l[0]), py(l[1]), px(l[2]), py(l[3]), 7*size/64, col)
		}
	}
	c.centeredText(center, 61, 9, practiceStatus(s), color.NRGBA{183, 196, 200, 210})
	if s.Settings.Timeline {
		w := math.Min(420, float64(width)/scale-44)
		x := center - w/2
		y := arrowAnchorY + size/2 + float64(s.Settings.TimelineOffset)
		c.rect(x, y, w, 79, color.NRGBA{10, 17, 27, 215})
		c.text(x+12, y+17, 10, "R-301", text)
		c.text(x+55, y+17, 8, "EXPECTED STRAFE", dim)
		c.text(x+w-94, y+17, 10, fmt.Sprintf("%.2f / 2.21 s", float64(s.ElapsedMS)/1000), dim)
		track := w - 24
		offset := 0.0
		durations := []float64{800, 530, 880}
		labels := []string{"R  800 ms", "L  530 ms", "R  880 ms"}
		for i, duration := range durations {
			segment := track * duration / 2210
			fill := color.NRGBA{32, 63, 56, 255}
			accent := mint
			if i == 1 {
				fill = color.NRGBA{72, 49, 45, 255}
				accent = coral
			}
			c.rect(x+12+offset, y+30, segment-3, 24, fill)
			c.rect(x+12+offset, y+30, segment-3, 2, accent)
			c.centeredText(x+12+offset+(segment-3)/2, y+46, 10, labels[i], text)
			offset += segment
		}
		head := x + 12 + track*math.Min(1, float64(s.ElapsedMS)/2210)
		c.rect(head-1, y+27, 2, 30, color.NRGBA{255, 255, 255, 255})
		c.text(x+12, y+70, 8, "0", dim)
		c.centeredText(x+12+track*800/2210, y+70, 8, "0.80", dim)
		c.centeredText(x+12+track*1330/2210, y+70, 8, "1.33", dim)
		c.text(x+w-34, y+70, 8, "2.21 s", dim)
	}
	return img
}
