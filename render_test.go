package main

import (
	"image/color"
	"image/png"
	"os"
	"recoil-overlay/internal/pattern"
	"testing"
)

func TestPracticeCanvasKeepsCrosshairTransparent(t *testing.T) {
	for _, dpi := range []float64{1, 1.5, 2} {
		s := Snapshot{Settings: defaultSettings(), Armed: true, Focused: true, Direction: "right"}
		img := drawPractice(s, int(620*dpi), int(600*dpi), dpi)
		if img.RGBAAt(int(310*dpi), int(142*dpi)).A != 0 {
			t.Fatal("crosshair must be transparent")
		}
		if img.RGBAAt(int(310*dpi), int(235*dpi)).A == 0 {
			t.Fatal("timeline should be drawn")
		}
		if img.RGBAAt(int(381*dpi), int(142*dpi)).A == 0 {
			t.Fatal("arrow should be drawn")
		}
		s.Settings.TimelineIdle, s.Held = true, true
		if drawPractice(s, int(620*dpi), int(600*dpi), dpi).RGBAAt(int(310*dpi), int(235*dpi)).A != 0 {
			t.Fatal("timeline should hide while shooting")
		}
		s.Held = false
		if drawPractice(s, int(620*dpi), int(600*dpi), dpi).RGBAAt(int(310*dpi), int(235*dpi)).A == 0 {
			t.Fatal("timeline should return on release")
		}
		s.Settings.Timeline, s.Settings.Arrows = false, false
		hidden := drawPractice(s, int(620*dpi), int(600*dpi), dpi)
		if hidden.RGBAAt(int(310*dpi), int(235*dpi)).A != 0 {
			t.Fatal("hidden timeline leaves pixels")
		}
		if hidden.RGBAAt(int(381*dpi), int(142*dpi)).A != 0 {
			t.Fatal("hidden arrows leave pixels")
		}
		if os.Getenv("RECOIL_RENDER_TEST") == "1" && dpi == 1 {
			if err := os.MkdirAll("test-results", 0755); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create("test-results/native-practice.png")
			if err != nil {
				t.Fatal(err)
			}
			if err = png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}

func TestPlayerTrackAndReviewLabels(t *testing.T) {
	s := Snapshot{Settings: defaultSettings(), Armed: true, Focused: true, Direction: "right", Running: true, ElapsedMS: 1000}
	s.Player = []pattern.Segment{{StartMS: 0, EndMS: 800, Direction: "right"}, {StartMS: 800, EndMS: 900, Direction: ""}, {StartMS: 900, EndMS: 1000, Direction: "left"}}
	img := drawPractice(s, 464, 300, 1)
	// The player row sits 58 px below the top of the timeline box.
	y := int(arrowAnchorY + 21 + 32 + 58 + 7)
	at := func(ms float64) color.RGBA { return img.RGBAAt(int(232-210+12+396*ms/2210), y) }
	// The cut at each switch lines up with the gap in the expected bar above.
	for _, ms := range []float64{800, 1330} {
		above := img.RGBAAt(int(232-210+12+396*ms/2210), y-28)
		if at(ms) != above || at(ms) == at(ms-30) {
			t.Fatalf("cut at %v ms not aligned: %v above %v", ms, at(ms), above)
		}
	}
	if at(400) != (color.RGBA{112, 227, 192, 255}) || at(950) != (color.RGBA{244, 161, 140, 255}) {
		t.Fatalf("strafes not drawn: %v %v", at(400), at(950))
	}
	if neutral, ahead := at(850), at(1500); neutral != ahead || neutral == at(400) {
		t.Fatalf("neutral and unplayed time should stay grey: %v %v", neutral, ahead)
	}
}
