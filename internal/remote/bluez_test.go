package remote

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/ujjwal313/boombrick/internal/player"
)

// fakeBluez is BlueZ as the player sees it: one adapter with a media
// service, which records the players registered with it.
type fakeBluez struct {
	mu         sync.Mutex
	registered []dbus.ObjectPath
	sender     string
}

func (b *fakeBluez) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, *dbus.Error) {
	return map[dbus.ObjectPath]map[string]map[string]dbus.Variant{
		"/org/bluez/hci0":                    {mediaIface: {}, "org.bluez.Adapter1": {}},
		"/org/bluez/hci0/dev_AA_BB_CC_DD_EE": {"org.bluez.Device1": {}},
	}, nil
}

type media struct{ b *fakeBluez }

func (m media) RegisterPlayer(sender dbus.Sender, path dbus.ObjectPath, props map[string]dbus.Variant) *dbus.Error {
	m.b.mu.Lock()
	defer m.b.mu.Unlock()
	m.b.registered = append(m.b.registered, path)
	m.b.sender = string(sender)
	return nil
}

// lockedPlayer is fakePlayer safe to read from the test while the remote
// drives it.
type lockedPlayer struct {
	mu sync.Mutex
	fakePlayer
}

func (p *lockedPlayer) State() player.State { p.mu.Lock(); defer p.mu.Unlock(); return p.s }
func (p *lockedPlayer) Toggle()             { p.mu.Lock(); defer p.mu.Unlock(); p.fakePlayer.Toggle() }
func (p *lockedPlayer) Next()               { p.mu.Lock(); defer p.mu.Unlock(); p.fakePlayer.Next() }
func (p *lockedPlayer) Prev()               { p.mu.Lock(); defer p.mu.Unlock(); p.fakePlayer.Prev() }

// On a bus of its own, with BlueZ played by fakeBluez: the player registers,
// headphones' commands reach the player, and BlueZ is told it paused.
func TestBluezPlayer(t *testing.T) {
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("no dbus-daemon")
	}
	cmd := exec.Command(daemon, "--session", "--nofork", "--print-address")
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Skip("dbus-daemon won't start:", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	line := make([]byte, 512)
	n, _ := out.Read(line)
	addr := strings.TrimSpace(string(line[:n]))
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", addr)

	bz, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer bz.Close()
	fb := &fakeBluez{}
	bz.Export(fb, "/", "org.freedesktop.DBus.ObjectManager")
	bz.Export(media{fb}, "/org/bluez/hci0", mediaIface)
	if r, err := bz.RequestName("org.bluez", 0); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		t.Fatal("org.bluez", r, err)
	}
	changes := make(chan *dbus.Signal, 16)
	bz.Signal(changes)
	bz.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), dbus.WithMatchMember("PropertiesChanged"))

	p := &lockedPlayer{fakePlayer: fakePlayer{s: player.State{Queued: true, Track: player.Track{Path: "/m/a.flac", Title: "Neon"}}}}
	r := New(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := runBluezOnce(ctx, r)
	defer func() { cancel(); <-done }()

	var who string
	waitFor(t, "registration", func() bool {
		fb.mu.Lock()
		defer fb.mu.Unlock()
		who = fb.sender
		return len(fb.registered) == 1 && fb.registered[0] == playerPath
	})
	pl := bz.Object(who, playerPath)
	if err := pl.Call(playerIface+".Play", 0).Err; err != nil {
		t.Fatal(err)
	}
	var st dbus.Variant
	waitFor(t, "Playing", func() bool {
		return pl.Call("org.freedesktop.DBus.Properties.Get", 0, playerIface, "PlaybackStatus").Store(&st) == nil &&
			st.Value() == "Playing"
	})
	if err := pl.Call(playerIface+".Pause", 0).Err; err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case sig := <-changes:
			if len(sig.Body) > 1 {
				if ch, ok := sig.Body[1].(map[string]dbus.Variant); ok && ch["PlaybackStatus"].Value() == "Paused" {
					if os.Getenv("VERBOSE") != "" {
						t.Log("told BlueZ:", ch)
					}
					return
				}
			}
		case <-deadline:
			t.Fatal("BlueZ wasn't told of the pause")
		}
	}
}

// runBluezOnce serves the player as runBluez does, on the bus the test set.
func runBluezOnce(ctx context.Context, r *Remote) <-chan struct{} {
	done := make(chan struct{})
	go func() { defer close(done); serveBluez(ctx, r) }()
	return done
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("no", what)
}
