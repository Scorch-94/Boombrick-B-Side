package platform

import (
	"encoding/binary"
	"errors"
	"os"
)

// brickLevels are the display driver's brightness at NextUI's levels 0 to 10
// on the Brick: level 1 is the 8 it wakes the panel with, and level 6 the 96
// it falls back to.
var brickLevels = [...]int{1, 8, 16, 32, 48, 72, 96, 128, 160, 192, 255}

// settingsBrightness is the brightness level's field in NextUI's shared
// settings (SettingsV11, as volume.go reads).
const settingsBrightness = 1

// brightnessSettings is where NextUI keeps the level; tests point it elsewhere.
var brightnessSettings = volumeSettingsFile

// Step makes the screen one of NextUI's levels brighter (delta 1) or dimmer
// (-1), and returns the level it's at, 0 to 10. The level is kept in NextUI's
// shared settings too, so it stays after Boombrick quits. With the light off
// it does nothing.
func (b *Backlight) Step(delta int) (int, error) {
	if b.off {
		return 0, errors.New("the screen is off")
	}
	raw, err := b.dev.brightness()
	if err != nil {
		return 0, err
	}
	level, ok := sharedLevel(brightnessSettings)
	if !ok || brickLevels[level] != raw {
		level = nearestLevel(raw) // NextUI's level isn't what shows: go by the screen
	}
	level = min(max(level+delta, 0), len(brickLevels)-1)
	if err := b.dev.setBrightness(brickLevels[level]); err != nil {
		return 0, err
	}
	setSharedLevel(brightnessSettings, level)
	return level, nil
}

// nearestLevel is the level whose brightness is nearest raw.
func nearestLevel(raw int) int {
	best := 0
	for i, v := range brickLevels {
		if abs(v-raw) < abs(brickLevels[best]-raw) {
			best = i
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// sharedLevel reads the brightness level from NextUI's shared settings, if
// they're there and the version volume.go knows.
func sharedLevel(path string) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	var raw [8]byte
	if n, _ := f.ReadAt(raw[:], 0); n < len(raw) {
		return 0, false
	}
	if int32(binary.LittleEndian.Uint32(raw[0:])) != settingsV11 {
		return 0, false
	}
	l := int(int32(binary.LittleEndian.Uint32(raw[settingsBrightness*4:])))
	if l < 0 || l >= len(brickLevels) {
		return 0, false
	}
	return l, true
}

// setSharedLevel writes level into NextUI's shared settings, when they're
// the version volume.go knows; otherwise it leaves them be.
func setSharedLevel(path string, level int) {
	if _, ok := sharedLevel(path); !ok {
		return
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(int32(level)))
	f.WriteAt(b[:], settingsBrightness*4)
}
