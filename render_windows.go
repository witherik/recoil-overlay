//go:build windows && amd64

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"recoil-overlay/internal/pattern"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/sys/windows"
)

// Practice draws a premultiplied alpha bitmap into a native layered window
// placed over the crosshair; the OS then handles true click-through. Edit mode
// shows the same window as its preview. See practiceWindow for why the Wails
// window can't be used.
var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	createCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	createDIBSection   = gdi32.NewProc("CreateDIBSection")
	selectObject       = gdi32.NewProc("SelectObject")
	deleteObject       = gdi32.NewProc("DeleteObject")
	deleteDC           = gdi32.NewProc("DeleteDC")
)

// The overlay is set in Segoe UI, like the settings window. Go Regular stands
// in if Windows has no copy of it.
var overlayFont = func() *opentype.Font {
	if b, err := os.ReadFile(filepath.Join(os.Getenv("WINDIR"), "Fonts", "segoeui.ttf")); err == nil {
		if f, err := opentype.Parse(b); err == nil {
			return f
		}
	}
	f, _ := opentype.Parse(goregular.TTF)
	return f
}()

// The layout, in unscaled pixels.

// Vertical midpoint of the arrows, from the window top, while the timeline is
// below them or hidden.
const arrowAnchorY = 142.0

// Baseline of the status line under the arrows, used when the timeline is above.
const statusBelow = 89.0

// Height of the timeline box: header, expected track, player track, ticks.
const timelineHeight = 114.0

// Opacity of the arrow that is not currently called for (the frontend matches).
const inactiveArrowAlpha = 40

// A negative timeline spacing puts the timeline above the arrows, that far
// from their top edge.
func timelineAbove(s Settings) bool { return s.Timeline && s.TimelineOffset < 0 }

// anchorY is the vertical midpoint of the arrows within the overlay window. It
// moves down to make room when the timeline sits above them.
func anchorY(s Settings) float64 {
	if timelineAbove(s) {
		return 4 + timelineHeight - float64(s.TimelineOffset) + float64(s.ArrowSize)/2
	}
	return arrowAnchorY
}

// Pixel size of the overlay window: just large enough for what is drawn.
func overlaySize(s Settings, scale float64) (int32, int32) {
	height := anchorY(s) + float64(s.ArrowSize)/2 + 12
	switch {
	case timelineAbove(s):
		height = anchorY(s) + statusBelow + 12
	case s.Timeline:
		height += float64(s.TimelineOffset) + timelineHeight - 4
	}
	return int32(math.Round(float64(max(464, s.TimelineWidth+44)) * scale)), int32(math.Round(height * scale))
}

// practiceStatus is the line of text that says what the overlay is waiting for.
func practiceStatus(s Snapshot) string {
	if s.Moving {
		return "DRAG TO POSITION"
	}
	start, end, pause := strings.ToUpper(s.StartKey), strings.ToUpper(s.EndKey), strings.ToUpper(s.PauseKey)
	if s.Editing {
		return "EDIT MODE  /  " + start + " TO PRACTICE"
	}
	if !s.Armed {
		return "DISABLED  /  " + pause + " TO ENABLE"
	}
	if !s.Focused {
		return "WAITING FOR APEX  /  " + end + " TO EDIT"
	}
	if s.Held && !s.Running {
		return "RELEASE TO RESET"
	}
	return end + " EDIT  /  " + pause + " DISABLE"
}

// Everything a frame depends on. An unchanged frame is not drawn again.
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

var practiceFrameValid bool
var lastPracticeFrame frameKey

// renderPractice draws s on the overlay window, where placeOverlay last put it.
func renderPractice(s Snapshot) error {
	if practiceWindow == 0 {
		return fmt.Errorf("overlay window unavailable")
	}
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
	// Only place the window once per mode change, so a drag is never undone.
	var position *point
	if !practicePlaced {
		position = &point{rect.Left, rect.Top}
	}
	surface := drawPractice(s, width, height, float64(dpi)/96)
	if err := presentOverlay(surface, position, byte(s.Settings.Opacity*255/100)); err != nil {
		return err
	}
	lastPracticeFrame = key
	practiceFrameValid, practicePlaced = true, true
	return nil
}

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

// presentOverlay makes surface the content of the overlay window, blended
// onto the screen at the given opacity, and moves the window if position is
// not nil.
func presentOverlay(surface *image.RGBA, position *point, opacity byte) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	width, height := surface.Rect.Dx(), surface.Rect.Dy()
	dc, _, err := createCompatibleDC.Call(0)
	if dc == 0 {
		return fmt.Errorf("create overlay canvas: %w", err)
	}
	defer deleteDC.Call(dc)
	info := bitmapInfo{Header: bitmapHeader{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32}}
	var pixels unsafe.Pointer
	bitmap, _, err := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if bitmap == 0 || pixels == nil {
		return fmt.Errorf("create overlay bitmap: %w", err)
	}
	defer deleteObject.Call(bitmap)
	old, _, _ := selectObject.Call(dc, bitmap)
	defer selectObject.Call(dc, old)
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
	blend := blendFunction{Alpha: opacity, Format: 1} // AC_SRC_ALPHA
	const alphaBlend = 2                              // ULW_ALPHA
	ok, _, err := updateLayeredWindow.Call(practiceWindow, 0, uintptr(unsafe.Pointer(position)), uintptr(unsafe.Pointer(&size)), dc, uintptr(unsafe.Pointer(&origin)), 0, uintptr(unsafe.Pointer(&blend)), alphaBlend)
	if ok == 0 {
		return fmt.Errorf("paint transparent overlay: %w", err)
	}
	return nil
}

// A canvas draws in unscaled pixels onto an image scale times as large.
type canvas struct {
	image *image.RGBA
	scale float64
}

func (c canvas) rect(x, y, w, h float64, col color.NRGBA) {
	r := image.Rect(int(math.Round(x*c.scale)), int(math.Round(y*c.scale)), int(math.Round((x+w)*c.scale)), int(math.Round((y+h)*c.scale)))
	draw.Draw(c.image, r, image.NewUniform(col), image.Point{}, draw.Src)
}

// unkerned is a face without pair kerning. Segoe UI's pairs are sized for
// large text; at the overlay's 10 to 13 px they pull letters such as the IAT of
// DEVIATION into one another.
type unkerned struct{ font.Face }

func (unkerned) Kern(r0, r1 rune) fixed.Int26_6 { return 0 }

func (c canvas) face(size float64) font.Face {
	face, _ := opentype.NewFace(overlayFont, &opentype.FaceOptions{Size: size * c.scale, DPI: 72, Hinting: font.HintingFull})
	return unkerned{face}
}

// text draws text with its baseline starting at (x, y).
func (c canvas) text(x, y, size float64, text string, col color.NRGBA) {
	face := c.face(size)
	defer face.Close()
	d := font.Drawer{Dst: c.image, Src: image.NewUniform(col), Face: face, Dot: fixed.P(int(math.Round(x*c.scale)), int(math.Round(y*c.scale)))}
	d.DrawString(text)
}

// width measures text in unscaled pixels.
func (c canvas) width(size float64, text string) float64 {
	face := c.face(size)
	defer face.Close()
	return float64(font.MeasureString(face, text).Ceil()) / c.scale
}

func (c canvas) centeredText(center, y, size float64, text string, col color.NRGBA) {
	c.text(center-c.width(size, text)/2, y, size, text, col)
}

// line draws an anti-aliased stroke with round ends.
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

// drawPractice draws the whole overlay: the arrows either side of the
// crosshair, the status line and the timeline.
func drawPractice(s Snapshot, width, height int, scale float64) *image.RGBA {
	c := canvas{image.NewRGBA(image.Rect(0, 0, width, height)), scale}
	w, h := float64(width)/scale, float64(height)/scale
	center, anchor := w/2, anchorY(s.Settings)
	scheme := palettes[s.Settings.Theme]
	if s.Moving {
		// A backdrop makes the whole rectangle grabbable, not just the drawn pixels.
		c.rect(0, 0, w, h, scheme.right)
		c.rect(1, 1, w-2, h-2, alpha(ui.Bg, 150))
		c.line(center-9, anchor, center+9, anchor, 1.5, ui.Text)
		c.line(center, anchor-9, center, anchor+9, 1.5, ui.Text)
	}
	size := float64(s.Settings.ArrowSize)
	if s.Settings.Arrows {
		apart := (float64(s.Settings.Gap) + size) / 2
		c.arrow("left", center-apart, anchor, size, scheme.left, s.Armed && s.Direction == "left")
		c.arrow("right", center+apart, anchor, size, scheme.right, s.Armed && s.Direction == "right")
	}
	// The status line keeps clear of the timeline: it takes the other side.
	above := timelineAbove(s.Settings)
	statusY := anchor - 81
	if above {
		statusY = anchor + statusBelow
	}
	c.centeredText(center, statusY, 11, practiceStatus(s), ui.Text)
	// Optionally clear the view while shooting; the timeline returns on release.
	firing := s.Settings.TimelineIdle && s.Held && !s.Editing
	if s.Settings.Timeline && !firing {
		boxWidth := math.Min(float64(s.Settings.TimelineWidth), w-44)
		y := anchor + size/2 + float64(s.Settings.TimelineOffset)
		if above {
			y = anchor - size/2 + float64(s.Settings.TimelineOffset) - timelineHeight
		}
		c.timeline(s, center-boxWidth/2, y, boxWidth, scheme)
	}
	return c.image
}

// The arrow on a 64 px grid, pointing left: two strokes for the head and one
// for the shaft, each from (x1, y1) to (x2, y2).
var arrowStrokes = [][4]float64{{37, 12, 17, 32}, {17, 32, 37, 52}, {18, 32, 52, 32}}

// arrow draws the arrow for a direction, size pixels wide and centred on
// (x, y). Lit, it is drawn in col over a dark outline; otherwise it is only a
// faint hint.
func (c canvas) arrow(direction string, x, y, size float64, col color.NRGBA, lit bool) {
	strokes := func(to canvas, width, down float64, col color.NRGBA) {
		px := func(v float64) float64 {
			if direction == "right" {
				v = 64 - v
			}
			return x - size/2 + v*size/64
		}
		py := func(v float64) float64 { return y - size/2 + v*size/64 + down }
		for _, l := range arrowStrokes {
			to.line(px(l[0]), py(l[1]), px(l[2]), py(l[3]), width*size/64, col)
		}
	}
	if lit {
		strokes(c, 9, 1, alpha(ui.Bg, 170))
		strokes(c, 7, 0, col)
		return
	}
	// Draw it opaque on a scratch layer first, so its overlapping strokes do
	// not add up at the joints.
	layer := canvas{image.NewRGBA(c.image.Bounds()), c.scale}
	strokes(layer, 7, 0, ui.Dim)
	box := image.Rect(int((x-size)*c.scale), int((y-size)*c.scale), int((x+size)*c.scale), int((y+size)*c.scale))
	draw.DrawMask(c.image, box, layer.image, box.Min, image.NewUniform(color.Alpha{inactiveArrowAlpha}), image.Point{}, draw.Over)
}

// timeline draws the timeline box, w wide with its top-left corner at (x, y):
// a header, the expected strafes, the player's own on the same time axis below
// them, and a label under each switch.
func (c canvas) timeline(s Snapshot, x, y, w float64, scheme palette) {
	box := alpha(ui.Bg, 228)
	c.rect(x, y, w, timelineHeight, box)
	// Between sprays the last one stays up for review, with its score.
	review := !s.Running && s.Score != nil
	total := math.Max(1, float64(s.TotalMS))
	c.header(s, x, y, w, review, total)

	// The bars leave a 12 px margin in the box; at is where a time falls on them.
	left, track := x+12, w-24
	at := func(ms float64) float64 { return left + track*ms/total }
	// starts[i] is when phase i begins; the last entry is the end of the pattern.
	starts := make([]float64, len(s.Phases)+1)
	for i, phase := range s.Phases {
		starts[i+1] = starts[i] + float64(phase.DurationMS)
	}

	// Each switch is a 3 px cut centred on its exact time. The same cut runs
	// through the player's bar below, so the two rows line up.
	const cut = 3.0
	narrowest := track
	for i, phase := range s.Phases {
		from, to := at(starts[i]), at(starts[i+1])
		narrowest = math.Min(narrowest, to-from)
		if i > 0 {
			from += cut / 2
		}
		if i < len(s.Phases)-1 {
			to -= cut / 2
		}
		// Neutral phases (fire without strafing) are grey.
		letter, fill, accent := "-", ui.Raised, ui.Line
		switch phase.Direction {
		case "right":
			letter, fill, accent = "R", shade(scheme.right), scheme.right
		case "left":
			letter, fill, accent = "L", shade(scheme.left), scheme.left
		}
		c.rect(from, y+30, to-from, 24, fill)
		c.rect(from, y+30, to-from, 2, accent)
		// Short phases drop the duration, then the letter, rather than overflow.
		for _, label := range []string{fmt.Sprintf("%s  %d ms", letter, phase.DurationMS), letter} {
			if c.width(12, label) <= to-from-6 {
				c.centeredText((from+to)/2, y+47, 12, label, ui.Text)
				break
			}
		}
	}

	// The player's own strafes; grey is neutral.
	c.rect(left, y+58, track, 24, ui.Fill)
	for _, segment := range s.Player {
		fill := scheme.right
		switch segment.Direction {
		case "left":
			fill = scheme.left
		case "right":
		default:
			continue
		}
		from := math.Min(track, track*float64(segment.StartMS)/total)
		to := math.Min(track, track*float64(segment.EndMS)/total)
		c.rect(left+from, y+58, to-from, 24, fill)
	}

	// Tick labels sit under each phase start; in review they become deviations.
	compact := narrowest < 70
	for i := range s.Phases {
		label, col := fmt.Sprintf("%.2f", starts[i]/1000), ui.Dim
		if i == 0 {
			label = "0"
		}
		if review {
			for _, change := range s.Score.Switches {
				if change.AtMS == int64(starts[i]) {
					label, col = deviationLabel(change, compact), deviationColor(change, scheme.right)
				}
			}
		}
		if i == 0 {
			c.text(left, y+103, 13, label, col)
			continue
		}
		c.rect(at(starts[i])-cut/2, y+58, cut, 24, box)
		c.centeredText(at(starts[i]), y+103, 13, label, col)
	}
	if n := len(s.Phases); n > 0 && track*float64(s.Phases[n-1].DurationMS)/total >= 70 {
		end := fmt.Sprintf("%.2f s", total/1000)
		c.text(x+w-12-c.width(13, end), y+103, 13, end, ui.Dim)
	}

	playheadMS := s.ElapsedMS
	if review {
		playheadMS = s.PlayerEndMS
	}
	head := left + track*math.Min(1, float64(playheadMS)/total)
	c.rect(head-1, y+27, 2, 58, ui.Text)
}

// A label is one run of text in the timeline header.
type label struct {
	text string
	size float64
	col  color.NRGBA
}

// header draws the top line of the timeline box: the weapon and its mode at
// the left; at the right the spray's clock or, in review, its score.
func (c canvas) header(s Snapshot, x, y, w float64, review bool, total float64) {
	c.text(x+12, y+19, 12, s.Weapon, ui.Text)
	summary := []label{{fmt.Sprintf("%.2f / %.2f s", float64(s.ElapsedMS)/1000, total/1000), 12, ui.Dim}}
	if review {
		summary = scoreLabels(*s.Score)
	}
	summaryX := x + w - 12
	for _, part := range summary {
		summaryX -= c.width(part.size, part.text)
	}
	at := summaryX
	for _, part := range summary {
		// Large figures sit a little lower so they centre on the small text.
		c.text(at, y+19+(part.size-12)/4, part.size, part.text, part.col)
		at += c.width(part.size, part.text)
	}
	// The mode gives way when a narrow timeline has no room for it.
	if modeX := x + 12 + c.width(12, s.Weapon) + 9; modeX+c.width(10, s.Mode)+9 <= summaryX {
		c.text(modeX, y+19, 10, s.Mode, ui.Dim)
	}
}

// scoreLabels lays the spray's result out as a scoreboard: the total time the
// switches were mistimed by, in large plain figures (the overlay's face has
// fixed-width digits, so they hold their place), with its captions small
// beside it.
func scoreLabels(score pattern.Score) []label {
	made := len(score.Switches) - score.Missed
	switch {
	case len(score.Switches) == 0:
		return []label{{"NO SWITCHES", 12, ui.Dim}}
	case made == 0:
		return []label{{fmt.Sprintf("%d", score.Missed), 20, ui.Text}, {" MISSED", 10, ui.Dim}}
	}
	labels := []label{{"TOTAL DEVIATION  ", 10, ui.Dim}, {fmt.Sprintf("%d", score.TotalMS), 20, ui.Text}, {" ms", 10, ui.Dim}}
	if score.Missed > 0 {
		labels = append(labels, label{fmt.Sprintf("   %d MISSED", score.Missed), 10, ui.Miss})
	}
	return labels
}

func deviationLabel(change pattern.Change, compact bool) string {
	if change.Missed {
		return "MISS"
	}
	if compact {
		return fmt.Sprintf("%+d", change.DeviationMS)
	}
	return fmt.Sprintf("%+d ms", change.DeviationMS)
}

// Within 40 ms reads as on time, within 100 ms as close.
func deviationColor(change pattern.Change, good color.NRGBA) color.NRGBA {
	off := max(change.DeviationMS, -change.DeviationMS)
	switch {
	case change.Missed || off > 100:
		return ui.Miss
	case off > 40:
		return ui.Near
	}
	return good
}

// shade is the dark fill behind a segment of the expected bar: its direction's
// colour, mostly mixed into the timeline box.
func shade(c color.NRGBA) color.NRGBA {
	mix := func(v, bg uint8) uint8 { return uint8((int(v)*30 + int(bg)*70) / 100) }
	return color.NRGBA{mix(c.R, ui.Bg.R), mix(c.G, ui.Bg.G), mix(c.B, ui.Bg.B), 255}
}
