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

	"recoil-overlay/internal/pattern"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/sys/windows"
)

// Practice draws a premultiplied alpha bitmap into a native layered window
// placed exactly over the (hidden) Wails window; the OS then handles true
// click-through. See setOverlayMode for why the Wails window can't be used.
var gdi32 = windows.NewLazySystemDLL("gdi32.dll")
var overlayFont, _ = opentype.Parse(goregular.TTF)

// Vertical midpoint of the arrows, in unscaled pixels from the window top. The
// frontend stage uses the same anchor.
const arrowAnchorY = 142.0

// Opacity of the arrow that is not currently called for (the frontend matches).
const inactiveArrowAlpha = 40

// Height of the timeline box: header, expected track, player track, ticks.
const timelineHeight = 107.0

var practiceFrameValid bool
var lastPracticeFrame frameKey

type frameKey struct {
	Settings          Settings
	Width, Height     int
	DPI               uint32
	Elapsed           int64
	Direction, Status string
	Armed, Moving     bool
	Held, Editing     bool
	Revision          uint64
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

// Pixel size of the overlay window: just large enough for what is drawn.
func overlaySize(s Settings, scale float64) (int32, int32) {
	height := arrowAnchorY + float64(s.ArrowSize)/2 + 12
	if s.Timeline {
		height += float64(s.TimelineOffset) + timelineHeight - 4
	}
	return int32(math.Round(464 * scale)), int32(math.Round(height * scale))
}

func practiceStatus(s Snapshot) string {
	if s.Moving {
		return "DRAG TO POSITION"
	}
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
	key := frameKey{s.Settings, width, height, uint32(dpi), s.ElapsedMS, s.Direction, practiceStatus(s), s.Armed, s.Moving, s.Held, s.Editing, s.revision}
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
	// Only place the window once per mode change, so a drag is never undone.
	var position unsafe.Pointer
	if !practicePlaced {
		position = unsafe.Pointer(&point{rect.Left, rect.Top})
	}
	blend := blendFunction{Alpha: byte(s.Settings.Opacity * 255 / 100), Format: 1}
	ok, _, err := user32.NewProc("UpdateLayeredWindow").Call(practiceWindow, 0, uintptr(position), uintptr(unsafe.Pointer(&size)), dc, uintptr(unsafe.Pointer(&origin)), 0, uintptr(unsafe.Pointer(&blend)), 2)
	if ok == 0 {
		return fmt.Errorf("paint transparent overlay: %w", err)
	}
	lastPracticeFrame = key
	practiceFrameValid, practicePlaced = true, true
	return nil
}

func scoreSummary(score pattern.Score) string {
	made := len(score.Switches) - score.Missed
	switch {
	case len(score.Switches) == 0:
		return "NO SWITCHES"
	case made == 0:
		return fmt.Sprintf("%d MISSED", score.Missed)
	case score.Missed > 0:
		return fmt.Sprintf("AVG %d ms / %d MISSED", score.AverageMS, score.Missed)
	}
	return fmt.Sprintf("AVG %d ms", score.AverageMS)
}
func deviationLabel(change pattern.Change) string {
	if change.Missed {
		return "MISS"
	}
	return fmt.Sprintf("%+d ms", change.DeviationMS)
}

// Within 40 ms reads as on time, within 100 ms as close.
func deviationColor(change pattern.Change) color.NRGBA {
	off := change.DeviationMS
	if off < 0 {
		off = -off
	}
	switch {
	case change.Missed || off > 100:
		return color.NRGBA{244, 161, 140, 255}
	case off > 40:
		return color.NRGBA{240, 200, 110, 255}
	}
	return color.NRGBA{112, 227, 192, 255}
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
func (c canvas) face(size float64) font.Face {
	face, _ := opentype.NewFace(overlayFont, &opentype.FaceOptions{Size: size * c.scale, DPI: 72})
	return face
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
				// Source-over onto the premultiplied canvas.
				a := float64(col.A) * coverage / 255
				d := c.image.RGBAAt(x, y)
				mix := func(src, dst uint8) uint8 { return uint8(float64(src)*a + float64(dst)*(1-a) + .5) }
				c.image.SetRGBA(x, y, color.RGBA{mix(col.R, d.R), mix(col.G, d.G), mix(col.B, d.B), uint8(255*a + float64(d.A)*(1-a) + .5)})
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
	muted := color.NRGBA{150, 168, 172, 255}
	text := color.NRGBA{220, 232, 233, 255}
	dim := color.NRGBA{139, 161, 164, 255}
	size := float64(s.Settings.ArrowSize)
	gap := float64(s.Settings.Gap)
	if s.Moving {
		// A backdrop makes the whole rectangle grabbable, not just the drawn pixels.
		w, h := float64(width)/scale, float64(height)/scale
		c.rect(0, 0, w, h, mint)
		c.rect(1, 1, w-2, h-2, color.NRGBA{10, 17, 27, 150})
		c.line(center-9, arrowAnchorY, center+9, arrowAnchorY, 1.5, text)
		c.line(center, arrowAnchorY-9, center, arrowAnchorY+9, 1.5, text)
	}
	for _, direction := range []string{"left", "right"} {
		if !s.Settings.Arrows {
			break
		}
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
		if col != muted {
			for _, l := range coords {
				c.line(px(l[0]), py(l[1])+1, px(l[2]), py(l[3])+1, 9*size/64, color.NRGBA{0, 0, 0, 170})
			}
			for _, l := range coords {
				c.line(px(l[0]), py(l[1]), px(l[2]), py(l[3]), 7*size/64, col)
			}
			continue
		}
		// The waiting arrow is only a faint hint. Draw it opaque on a scratch
		// layer first so its overlapping strokes do not add up at the joints.
		layer := canvas{image.NewRGBA(img.Bounds()), scale}
		for _, l := range coords {
			layer.line(px(l[0]), py(l[1]), px(l[2]), py(l[3]), 7*size/64, col)
		}
		box := image.Rect(int((x-size)*scale), int((arrowAnchorY-size)*scale), int((x+size)*scale), int((arrowAnchorY+size)*scale))
		draw.DrawMask(img, box, layer.image, box.Min, image.NewUniform(color.Alpha{inactiveArrowAlpha}), image.Point{}, draw.Over)
	}
	c.centeredText(center, 61, 9, practiceStatus(s), color.NRGBA{183, 196, 200, 210})
	// Optionally clear the view while shooting; the timeline returns on release.
	firing := s.Settings.TimelineIdle && s.Held && !s.Editing
	if s.Settings.Timeline && !firing {
		w := math.Min(420, float64(width)/scale-44)
		x := center - w/2
		y := arrowAnchorY + size/2 + float64(s.Settings.TimelineOffset)
		box := color.NRGBA{10, 17, 27, 215}
		c.rect(x, y, w, timelineHeight, box)
		c.text(x+12, y+17, 10, "R-301", text)
		c.text(x+55, y+17, 8, "EXPECTED STRAFE", dim)
		// Between sprays the last one stays up for review, with its score.
		review := !s.Running && s.Score != nil
		playheadMS := s.ElapsedMS
		summary := fmt.Sprintf("%.2f / 2.21 s", float64(s.ElapsedMS)/1000)
		if review {
			playheadMS = s.PlayerEndMS
			summary = scoreSummary(*s.Score)
		}
		summaryFace := c.face(10)
		c.text(x+w-12-float64(font.MeasureString(summaryFace, summary).Ceil())/scale, y+17, 10, summary, dim)
		summaryFace.Close()
		track := w - 24
		offset := 0.0
		durations := []float64{800, 530, 880}
		labels := []string{"R  800 ms", "L  530 ms", "R  880 ms"}
		// Each switch is a 3 px cut centred on its exact time. The same cut runs
		// through the player's bar below, so the two rows line up.
		const cut = 3.0
		for i, duration := range durations {
			from, to := x+12+offset, x+12+offset+track*duration/2210
			if i > 0 {
				from += cut / 2
			}
			if i < len(durations)-1 {
				to -= cut / 2
			}
			fill := color.NRGBA{32, 63, 56, 255}
			accent := mint
			if i == 1 {
				fill = color.NRGBA{72, 49, 45, 255}
				accent = coral
			}
			c.rect(from, y+30, to-from, 24, fill)
			c.rect(from, y+30, to-from, 2, accent)
			c.centeredText((from+to)/2, y+46, 10, labels[i], text)
			offset += track * duration / 2210
		}
		// The player's own strafes, on the same time axis; grey is neutral.
		c.rect(x+12, y+58, track, 24, color.NRGBA{38, 48, 54, 255})
		for _, segment := range s.Player {
			fill := mint
			switch segment.Direction {
			case "left":
				fill = coral
			case "right":
			default:
				continue
			}
			from := math.Min(track, track*float64(segment.StartMS)/2210)
			to := math.Min(track, track*float64(segment.EndMS)/2210)
			c.rect(x+12+from, y+58, to-from, 24, fill)
		}
		for _, dueMS := range []float64{800, 1330} {
			c.rect(x+12+track*dueMS/2210-cut/2, y+58, cut, 24, box)
		}
		head := x + 12 + track*math.Min(1, float64(playheadMS)/2210)
		c.rect(head-1, y+27, 2, 58, color.NRGBA{255, 255, 255, 255})
		ticks := []struct {
			atMS  int64
			label string
		}{{0, "0"}, {800, "0.80"}, {1330, "1.33"}}
		for i, tick := range ticks {
			label, col := tick.label, dim
			if review {
				for _, change := range s.Score.Switches {
					if change.AtMS == tick.atMS {
						label, col = deviationLabel(change), deviationColor(change)
					}
				}
			}
			if i == 0 {
				c.text(x+12, y+98, 8, label, col)
			} else {
				c.centeredText(x+12+track*float64(tick.atMS)/2210, y+98, 8, label, col)
			}
		}
		c.text(x+w-34, y+98, 8, "2.21 s", dim)
	}
	return img
}
