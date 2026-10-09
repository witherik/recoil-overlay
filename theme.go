package main

import (
	_ "embed"
	"encoding/json"
	"image/color"
	"strconv"
)

// theme.json holds every colour of the app's one look: flat neutral greys, and
// a colour for each strafe direction. The overlay draws with them directly;
// the settings window imports the same file.
//
//go:embed frontend/src/theme.json
var themeJSON []byte

// A palette is a colour scheme: one colour for each strafe direction.
type palette struct{ right, left color.NRGBA }

var ui, palettes = loadTheme()

type neutrals struct {
	Bg, Fill, Raised, Line, Dim, Text color.NRGBA
	Miss, Near                        color.NRGBA // a switch missed, or only close
}

func loadTheme() (neutrals, map[string]palette) {
	var file struct {
		Neutral map[string]string
		Schemes []struct{ ID, Right, Left string }
	}
	if err := json.Unmarshal(themeJSON, &file); err != nil {
		panic(err)
	}
	rgb := func(hex string) color.NRGBA {
		v, err := strconv.ParseUint(hex[1:], 16, 24)
		if err != nil {
			panic(err)
		}
		return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
	}
	n := file.Neutral
	schemes := map[string]palette{}
	for _, s := range file.Schemes {
		schemes[s.ID] = palette{rgb(s.Right), rgb(s.Left)}
	}
	return neutrals{rgb(n["bg"]), rgb(n["fill"]), rgb(n["raised"]), rgb(n["line"]), rgb(n["dim"]), rgb(n["text"]), rgb(n["miss"]), rgb(n["near"])}, schemes
}

// alpha is c at the given opacity, out of 255.
func alpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}
