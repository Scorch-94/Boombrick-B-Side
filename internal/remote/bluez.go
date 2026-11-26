package remote

import (
	"context"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"

	"github.com/ujjwal313/boombrick/internal/player"
)

// The player Boombrick shows BlueZ: the MPRIS player interface, which BlueZ
// calls for the commands headphones send and reads what plays from.
const (
	playerPath  = dbus.ObjectPath("/org/boombrick/player")
	playerIface = "org.mpris.MediaPlayer2.Player"
	mediaIface  = "org.bluez.Media1"
)

// How often what plays is passed on, and BlueZ asked whether there is an
// adapter to register with: Bluetooth may be switched on, or BlueZ start
// again, at any time.
const (
	follow   = 500 * time.Millisecond
	rescan   = 15 * time.Second
	retryBus = time.Minute
)

// mpris is the player's methods, as BlueZ calls them.
type mpris struct{ r *Remote }

func (m mpris) Play() *dbus.Error      { m.r.Do(Play, "headphones"); return nil }
func (m mpris) Pause() *dbus.Error     { m.r.Do(Pause, "headphones"); return nil }
func (m mpris) PlayPause() *dbus.Error { m.r.Do(PlayPause, "headphones"); return nil }
func (m mpris) Stop() *dbus.Error      { m.r.Do(Stop, "headphones"); return nil }
func (m mpris) Next() *dbus.Error      { m.r.Do(Next, "headphones"); return nil }
func (m mpris) Previous() *dbus.Error  { m.r.Do(Previous, "headphones"); return nil }

// runBluez keeps a player registered with every Bluetooth adapter's media
// service, telling it what plays, until ctx ends. Without a system bus or
// BlueZ it waits and asks again.
func runBluez(ctx context.Context, r *Remote) {
	for ctx.Err() == nil {
		if err := serveBluez(ctx, r); err != nil && ctx.Err() == nil {
			r.log.Debug("bluetooth remote: no BlueZ player", "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(retryBus):
		}
	}
}

// serveBluez serves the player on its own connection to the system bus until
// ctx ends or the bus goes.
func serveBluez(ctx context.Context, r *Remote) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	defer conn.Close() // which unregisters the player
	if err := conn.Export(mpris{r}, playerPath, playerIface); err != nil {
		return err
	}
	s := r.p.State()
	props, err := prop.Export(conn, playerPath, prop.Map{playerIface: properties(s)})
	if err != nil {
		return err
	}
	var (
		registered = map[dbus.ObjectPath]bool{}
		owner      string // BlueZ's bus name, which changes when it starts again
		shown      = shownOf(s)
		told       = position{s.Elapsed, time.Now(), s.Playing}
		next       time.Time
	)
	tick := time.NewTicker(follow)
	defer tick.Stop()
	for {
		if now := time.Now(); !now.Before(next) {
			next = now.Add(rescan)
			if err := register(ctx, conn, r, registered, &owner); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
		s := r.p.State()
		now, at := shownOf(s), time.Now()
		jumped := told.jumped(s, at)
		if now == shown && !jumped {
			continue
		}
		all := properties(s)
		for _, name := range changed(shown, now) {
			props.SetMust(playerIface, name, all[name].Value)
		}
		// BlueZ counts the position on from the last it was told while
		// playing, so it is told again only with a change or a jump.
		props.SetMust(playerIface, "Position", all["Position"].Value)
		shown, told = now, position{s.Elapsed, at, s.Playing}
	}
}

// register registers the player with each adapter's media service it isn't
// registered with yet. BlueZ starting again forgets every registration.
func register(ctx context.Context, conn *dbus.Conn, r *Remote, registered map[dbus.ObjectPath]bool, owner *string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var name string
	if err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, "org.bluez").Store(&name); err != nil {
		clear(registered)
		*owner = ""
		return nil // BlueZ isn't running: asked again on the next rescan
	}
	if name != *owner {
		clear(registered)
		*owner = name
	}
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := conn.Object("org.bluez", "/").CallWithContext(ctx, "org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	if err != nil {
		return nil
	}
	for path, ifaces := range objects {
		if _, ok := ifaces[mediaIface]; !ok || registered[path] {
			continue
		}
		s := r.p.State()
		init := map[string]dbus.Variant{}
		for k, v := range properties(s) {
			init[k] = dbus.MakeVariant(v.Value)
		}
		call := conn.Object("org.bluez", path).CallWithContext(ctx, mediaIface+".RegisterPlayer", 0, playerPath, init)
		if call.Err != nil {
			r.log.Warn("bluetooth remote: register with BlueZ", "adapter", string(path), "err", call.Err)
			continue
		}
		registered[path] = true
		r.log.Info("bluetooth remote: registered with BlueZ", "adapter", string(path))
	}
	return nil
}

// shown is what the player shows BlueZ but its position, to tell when it
// changes.
type shown struct {
	status, loop string
	shuffle      bool
	track        player.Track
	duration     time.Duration
	index        int
}

func shownOf(s player.State) shown {
	return shown{status: status(s), loop: loop(s.Repeat), shuffle: s.Shuffle, track: s.Track,
		duration: s.Duration, index: s.Index}
}

// changed is the names of the properties that differ between was and now.
func changed(was, now shown) []string {
	var out []string
	if was.status != now.status {
		out = append(out, "PlaybackStatus")
	}
	if was.loop != now.loop {
		out = append(out, "LoopStatus")
	}
	if was.shuffle != now.shuffle {
		out = append(out, "Shuffle")
	}
	if was.track != now.track || was.duration != now.duration || was.index != now.index {
		out = append(out, "Metadata")
	}
	return out
}

// position is the position BlueZ was last told: where, when, and whether
// it was playing, so counting on.
type position struct {
	elapsed time.Duration
	at      time.Time
	playing bool
}

// jumpAfter is how far the position may be from where BlueZ counts it before
// it is told again: a seek.
const jumpAfter = 2 * time.Second

// jumped reports whether s's position is more than jumpAfter from where
// BlueZ, told p, counts it at now.
func (p position) jumped(s player.State, now time.Time) bool {
	want := p.elapsed
	if p.playing {
		want += now.Sub(p.at)
	}
	d := s.Elapsed - want
	return d > jumpAfter || d < -jumpAfter
}

// properties is the player's properties for s.
func properties(s player.State) map[string]*prop.Prop {
	ro := func(v any) *prop.Prop { return &prop.Prop{Value: v, Emit: prop.EmitTrue} }
	return map[string]*prop.Prop{
		"PlaybackStatus": ro(status(s)),
		"LoopStatus":     ro(loop(s.Repeat)),
		"Shuffle":        ro(s.Shuffle),
		"Metadata":       ro(metadata(s)),
		"Position":       ro(s.Elapsed.Microseconds()),
		"Rate":           ro(1.0),
		"MinimumRate":    ro(1.0),
		"MaximumRate":    ro(1.0),
		"Volume":         ro(1.0),
		"CanGoNext":      ro(s.Queued),
		"CanGoPrevious":  ro(s.Queued),
		"CanPlay":        ro(s.Queued),
		"CanPause":       ro(s.Queued),
		"CanSeek":        ro(false),
		"CanControl":     ro(true),
	}
}

func status(s player.State) string {
	switch {
	case !s.Queued:
		return "Stopped"
	case s.Playing:
		return "Playing"
	}
	return "Paused"
}

func loop(r player.Repeat) string {
	switch r {
	case player.RepeatOne:
		return "Track"
	case player.RepeatAll:
		return "Playlist"
	}
	return "None"
}

// metadata is the playing track's MPRIS metadata, which headphones and car
// stereos show.
func metadata(s player.State) map[string]dbus.Variant {
	m := map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(dbus.ObjectPath("/org/boombrick/track/" + trackID(s)))}
	if !s.Queued {
		return m
	}
	t := s.Track
	if title := t.Title; title != "" || s.LiveTitle != "" {
		if s.LiveTitle != "" {
			title = s.LiveTitle
		}
		m["xesam:title"] = dbus.MakeVariant(title)
	}
	if t.Artist != "" {
		m["xesam:artist"] = dbus.MakeVariant([]string{t.Artist})
	}
	if t.Album != "" {
		m["xesam:album"] = dbus.MakeVariant(t.Album)
	}
	if s.Duration > 0 {
		m["mpris:length"] = dbus.MakeVariant(s.Duration.Microseconds())
	}
	if s.Count > 0 {
		m["xesam:trackNumber"] = dbus.MakeVariant(int32(s.Index + 1))
	}
	return m
}

// trackID is a path element for the playing track: its place in the queue,
// which tells tracks apart.
func trackID(s player.State) string {
	if !s.Queued {
		return "none"
	}
	return "t" + strconv.Itoa(max(s.Index, 0))
}
