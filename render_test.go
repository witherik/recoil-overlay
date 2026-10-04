package main

import (
	"image/png"
	"os"
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
		s.Settings.Timeline = false
		hidden := drawPractice(s, int(620*dpi), int(600*dpi), dpi)
		if hidden.RGBAAt(int(310*dpi), int(235*dpi)).A != 0 {
			t.Fatal("hidden timeline leaves pixels")
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
