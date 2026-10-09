//go:build windows && amd64

package main

import (
	_ "embed"
	"encoding/binary"
	"math"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var playSound = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

// Every clip remains rooted for the entire process lifetime (async PlaySound).
//
//go:embed assets/voice/left.wav
var voiceLeft []byte

//go:embed assets/voice/right.wav
var voiceRight []byte

//go:embed assets/voice/natural/left.wav
var naturalLeft []byte

//go:embed assets/voice/natural/right.wav
var naturalRight []byte

// The tones are synthesized: a low beep for left, an octave higher for right.
var toneLeft, toneRight = toneClip(440), toneClip(880)

// toneClip is a 60 ms sine beep as a mono PCM16 WAV. It reaches full volume
// within half a millisecond, so it is heard the moment it is played, and fades
// out over 5 ms so it does not click.
func toneClip(hz float64) []byte {
	const rate, samples, attack, release = 22050, 22050 * 60 / 1000, 11, 110
	clip := make([]byte, 44+samples*2)
	copy(clip, "RIFF")
	binary.LittleEndian.PutUint32(clip[4:], uint32(len(clip)-8))
	copy(clip[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(clip[16:], 16)
	binary.LittleEndian.PutUint16(clip[20:], 1) // PCM
	binary.LittleEndian.PutUint16(clip[22:], 1) // mono
	binary.LittleEndian.PutUint32(clip[24:], rate)
	binary.LittleEndian.PutUint32(clip[28:], rate*2)
	binary.LittleEndian.PutUint16(clip[32:], 2)
	binary.LittleEndian.PutUint16(clip[34:], 16)
	copy(clip[36:], "data")
	binary.LittleEndian.PutUint32(clip[40:], samples*2)
	for i := 0; i < samples; i++ {
		gain := math.Min(1, math.Min(float64(i+1)/attack, float64(samples-i)/release))
		value := 0.6 * 32767 * gain * math.Sin(2*math.Pi*hz*float64(i)/rate)
		binary.LittleEndian.PutUint16(clip[44+2*i:], uint16(int16(value)))
	}
	return clip
}

// playVoice starts the cue for a direction in the given voice style, cutting
// off any cue still playing. It does not wait for the sound to end.
func playVoice(style, direction string) {
	left, right := voiceLeft, voiceRight
	switch style {
	case "natural":
		left, right = naturalLeft, naturalRight
	case "tones":
		left, right = toneLeft, toneRight
	}
	clip := right
	if direction == "left" {
		clip = left
	}
	const async, noDefault, memory = 0x1, 0x2, 0x4 // SND_ASYNC, SND_NODEFAULT, SND_MEMORY
	playSound.Call(uintptr(unsafe.Pointer(&clip[0])), 0, async|noDefault|memory)
	runtime.KeepAlive(clip)
}

func stopVoice() { playSound.Call(0, 0, 0) }
