//go:build windows && amd64

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
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
// shows the same window as its preview. See setOverlayMode for why the Wails window can't be used.
var gdi32 = windows.NewLazySystemDLL("gdi32.dll")
var overlayFont, _ = opentype.Parse(goregular.TTF)

// Vertical midpoint of the arrows, in unscaled pixels from the window top,
// while the timeline is below them or hidden.
const arrowAnchorY = 142.0

// Baseline of the status line under the arrows, used when the timeline is above.
const statusBelow = 89.0

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

// Opacity of the arrow that is not currently called for (the frontend matches).
const inactiveArrowAlpha = 40

// Height of the timeline box: header, expected track, player track, ticks.
const timelineHeight = 114.0

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
	height := anchorY(s) + float64(s.ArrowSize)/2 + 12
	switch {
	case timelineAbove(s):
		height = anchorY(s) + statusBelow + 12
	case s.Timeline:
		height += float64(s.TimelineOffset) + timelineHeight - 4
	}
	return int32(math.Round(float64(max(464, s.TimelineWidth+44)) * scale)), int32(math.Round(height * scale))
}

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

// A label is one run of text in the timeline header.
type label struct {
	text string
	size float64
	col  color.NRGBA
}

var missColor, closeColor = color.NRGBA{255, 180, 171, 255}, color.NRGBA{255, 213, 110, 255}

// scoreLabels lays the spray's result out as a scoreboard: the total time the
// switches were mistimed by, in large plain figures (the overlay's face has
// fixed-width digits, so they hold their place), with its captions small
// beside it.
func scoreLabels(score pattern.Score, text, dim color.NRGBA) []label {
	made := len(score.Switches) - score.Missed
	switch {
	case len(score.Switches) == 0:
		return []label{{"NO SWITCHES", 12, dim}}
	case made == 0:
		return []label{{fmt.Sprintf("%d", score.Missed), 20, text}, {" MISSED", 10, dim}}
	}
	labels := []label{{"TOTAL DEVIATION  ", 10, dim}, {fmt.Sprintf("%d", score.TotalMS), 20, text}, {" ms", 10, dim}}
	if score.Missed > 0 {
		labels = append(labels, label{fmt.Sprintf("   %d MISSED", score.Missed), 10, missColor})
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
	off := change.DeviationMS
	if off < 0 {
		off = -off
	}
	switch {
	case change.Missed || off > 100:
		return missColor
	case off > 40:
		return closeColor
	}
	return good
}

// A palette is the overlay's side of a colour scheme: one colour for each
// strafe direction. The settings window's side is in frontend/src/style.css.
type palette struct{ right, left color.NRGBA }

var palettes = map[string]palette{
	"green":  {color.NRGBA{112, 227, 192, 255}, color.NRGBA{244, 161, 140, 255}},
	"purple": {color.NRGBA{208, 188, 255, 255}, color.NRGBA{246, 193, 119, 255}},
	"red":    {color.NRGBA{255, 120, 108, 255}, color.NRGBA{127, 215, 232, 255}},
	"blue":   {color.NRGBA{147, 212, 255, 255}, color.NRGBA{232, 101, 10, 255}},
}

// shade is the dark fill behind a segment of the expected bar: its direction's
// colour, mostly mixed into the timeline box.
func shade(c color.NRGBA) color.NRGBA {
	mix := func(v uint8) uint8 { return uint8((int(v)*30 + 16*70) / 100) }
	return color.NRGBA{mix(c.R), mix(c.G), mix(c.B), 255}
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

// width measures text in unscaled pixels.
func (c canvas) width(size float64, text string) float64 {
	face, _ := opentype.NewFace(overlayFont, &opentype.FaceOptions{Size: size * c.scale, DPI: 72})
	defer face.Close()
	return float64(font.MeasureString(face, text).Ceil()) / c.scale
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
	scheme, known := palettes[s.Settings.Theme]
	if !known {
		scheme = palettes["green"]
	}
	right, left := scheme.right, scheme.left
	muted := color.NRGBA{184, 184, 184, 255}
	text := color.NRGBA{237, 237, 237, 255}
	dim := color.NRGBA{178, 178, 178, 255}
	size := float64(s.Settings.ArrowSize)
	anchor, above := anchorY(s.Settings), timelineAbove(s.Settings)
	gap := float64(s.Settings.Gap)
	if s.Moving {
		// A backdrop makes the whole rectangle grabbable, not just the drawn pixels.
		w, h := float64(width)/scale, float64(height)/scale
		c.rect(0, 0, w, h, right)
		c.rect(1, 1, w-2, h-2, color.NRGBA{16, 16, 16, 150})
		c.line(center-9, anchor, center+9, anchor, 1.5, text)
		c.line(center, anchor-9, center, anchor+9, 1.5, text)
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
				col = right
			} else {
				col = left
			}
		}
		px := func(v float64) float64 { return x - size/2 + v*size/64 }
		py := func(v float64) float64 { return anchor - size/2 + v*size/64 }
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
		box := image.Rect(int((x-size)*scale), int((anchor-size)*scale), int((x+size)*scale), int((anchor+size)*scale))
		draw.DrawMask(img, box, layer.image, box.Min, image.NewUniform(color.Alpha{inactiveArrowAlpha}), image.Point{}, draw.Over)
	}
	// The status line keeps clear of the timeline: it takes the other side.
	statusY := anchor - 81
	if above {
		statusY = anchor + statusBelow
	}
	c.centeredText(center, statusY, 11, practiceStatus(s), color.NRGBA{222, 222, 222, 230})
	// Optionally clear the view while shooting; the timeline returns on release.
	firing := s.Settings.TimelineIdle && s.Held && !s.Editing
	if s.Settings.Timeline && !firing {
		w := math.Min(float64(s.Settings.TimelineWidth), float64(width)/scale-44)
		x := center - w/2
		y := anchor + size/2 + float64(s.Settings.TimelineOffset)
		if above {
			y = anchor - size/2 + float64(s.Settings.TimelineOffset) - timelineHeight
		}
		box := color.NRGBA{16, 16, 16, 228}
		c.rect(x, y, w, timelineHeight, box)
		total := math.Max(1, float64(s.TotalMS))
		c.text(x+12, y+19, 12, s.Weapon, text)
		// Between sprays the last one stays up for review, with its score.
		review := !s.Running && s.Score != nil
		playheadMS := s.ElapsedMS
		summary := []label{{fmt.Sprintf("%.2f / %.2f s", float64(s.ElapsedMS)/1000, total/1000), 12, dim}}
		if review {
			playheadMS = s.PlayerEndMS
			summary = scoreLabels(*s.Score, text, dim)
		}
		summaryX := x + w - 12
		for _, part := range summary {
			summaryX -= c.width(part.size, part.text)
		}
		for at, i := summaryX, 0; i < len(summary); i++ {
			part := summary[i]
			// Large figures sit a little lower so they centre on the small text.
			c.text(at, y+19+(part.size-12)/4, part.size, part.text, part.col)
			at += c.width(part.size, part.text)
		}
		// The mode gives way when a narrow timeline has no room for it.
		if modeX := x + 12 + c.width(12, s.Weapon) + 9; modeX+c.width(10, s.Mode)+9 <= summaryX {
			c.text(modeX, y+19, 10, s.Mode, dim)
		}
		track := w - 24
		// Each switch is a 3 px cut centred on its exact time. The same cut runs
		// through the player's bar below, so the two rows line up.
		const cut = 3.0
		narrowest, startMS := track, 0.0
		for i, phase := range s.Phases {
			duration := float64(phase.DurationMS)
			from, to := x+12+track*startMS/total, x+12+track*(startMS+duration)/total
			narrowest = math.Min(narrowest, to-from)
			if i > 0 {
				from += cut / 2
			}
			if i < len(s.Phases)-1 {
				to -= cut / 2
			}
			// Neutral phases (fire without strafing) are grey.
			letter, fill, accent := "-", color.NRGBA{56, 56, 56, 255}, color.NRGBA{142, 142, 142, 255}
			switch phase.Direction {
			case "right":
				letter, fill, accent = "R", shade(right), right
			case "left":
				letter, fill, accent = "L", shade(left), left
			}
			c.rect(from, y+30, to-from, 24, fill)
			c.rect(from, y+30, to-from, 2, accent)
			// Short phases drop the duration, then the letter, rather than overflow.
			for _, label := range []string{fmt.Sprintf("%s  %d ms", letter, phase.DurationMS), letter} {
				if c.width(12, label) <= to-from-6 {
					c.centeredText((from+to)/2, y+47, 12, label, text)
					break
				}
			}
			startMS += duration
		}
		// The player's own strafes, on the same time axis; grey is neutral.
		c.rect(x+12, y+58, track, 24, color.NRGBA{44, 44, 44, 255})
		for _, segment := range s.Player {
			fill := right
			switch segment.Direction {
			case "left":
				fill = left
			case "right":
			default:
				continue
			}
			from := math.Min(track, track*float64(segment.StartMS)/total)
			to := math.Min(track, track*float64(segment.EndMS)/total)
			c.rect(x+12+from, y+58, to-from, 24, fill)
		}
		head := x + 12 + track*math.Min(1, float64(playheadMS)/total)
		// Tick labels sit under each phase start; in review they become deviations.
		compact := narrowest < 70
		startMS = 0
		for i, phase := range s.Phases {
			label, col := fmt.Sprintf("%.2f", startMS/1000), dim
			if i == 0 {
				label = "0"
			} else {
				c.rect(x+12+track*startMS/total-cut/2, y+58, cut, 24, box)
			}
			if review {
				for _, change := range s.Score.Switches {
					if change.AtMS == int64(startMS) {
						label, col = deviationLabel(change, compact), deviationColor(change, right)
					}
				}
			}
			if i == 0 {
				c.text(x+12, y+103, 13, label, col)
			} else {
				c.centeredText(x+12+track*startMS/total, y+103, 13, label, col)
			}
			startMS += float64(phase.DurationMS)
		}
		c.rect(head-1, y+27, 2, 58, color.NRGBA{255, 255, 255, 255})
		if n := len(s.Phases); n > 0 && track*float64(s.Phases[n-1].DurationMS)/total >= 70 {
			end := fmt.Sprintf("%.2f s", total/1000)
			c.text(x+w-12-c.width(13, end), y+103, 13, end, dim)
		}
	}
	return img
}
