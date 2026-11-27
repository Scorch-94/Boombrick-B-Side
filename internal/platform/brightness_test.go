package platform

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

type fakeDisplay struct{ raw int }

func (d *fakeDisplay) brightness() (int, error)  { return d.raw, nil }
func (d *fakeDisplay) setBrightness(v int) error { d.raw = v; return nil }

// Steps go through NextUI's levels, stopping at the ends, and are kept in
// its shared settings. Where those say another level than shows, the screen
// is gone by.
func TestBrightnessSteps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SharedSettings")
	settings := make([]byte, settingsFields*4)
	binary.LittleEndian.PutUint32(settings[0:], settingsV11)
	binary.LittleEndian.PutUint32(settings[4:], 6)
	if err := os.WriteFile(path, settings, 0o644); err != nil {
		t.Fatal(err)
	}
	old := brightnessSettings
	brightnessSettings = path
	defer func() { brightnessSettings = old }()

	d := &fakeDisplay{raw: 96}
	b := &Backlight{dev: d}
	if l, err := b.Step(1); err != nil || l != 7 || d.raw != 128 {
		t.Fatalf("up: level %d, raw %d, %v", l, d.raw, err)
	}
	if l, ok := sharedLevel(path); !ok || l != 7 {
		t.Fatalf("kept %d %v", l, ok)
	}
	for range 10 {
		b.Step(1)
	}
	if d.raw != 255 {
		t.Fatalf("top %d", d.raw)
	}
	d.raw = 30 // changed elsewhere: nearest is level 3's 32
	if l, _ := b.Step(-1); l != 2 || d.raw != 16 {
		t.Fatalf("from 30 down: level %d raw %d", l, d.raw)
	}
	for range 10 {
		b.Step(-1)
	}
	if d.raw != 1 {
		t.Fatalf("bottom %d: never off", d.raw)
	}
	b.off = true
	if _, err := b.Step(1); err == nil {
		t.Fatal("stepped with the screen off")
	}
	// Settings of a version not known are left alone.
	binary.LittleEndian.PutUint32(settings[0:], 9)
	os.WriteFile(path, settings, 0o644)
	setSharedLevel(path, 2)
	got, _ := os.ReadFile(path)
	if binary.LittleEndian.Uint32(got[4:]) != 6 {
		t.Fatal("wrote settings of an unknown version")
	}
}
