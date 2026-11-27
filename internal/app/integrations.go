package app

import (
	"slices"

	"github.com/ujjwal313/boombrick/internal/screens"
	settingsscreen "github.com/ujjwal313/boombrick/internal/screens/settings"
	keys "github.com/ujjwal313/boombrick/internal/settings"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// The integrations' names, as Settings shows them.
const (
	spotifyName = "Spotify"
	youtubeName = "YouTube"
	radioName   = "Radio"
)

// spotifyOn and youtubeOn read the integrations' switches; without Prefs both
// are on.
func (a *App) spotifyOn() bool { return a.opts.Prefs == nil || a.opts.Prefs.Bool(keys.SpotifyOn) }
func (a *App) youtubeOn() bool { return a.opts.Prefs == nil || a.opts.Prefs.Bool(keys.YouTubeOn) }

// radioOn is whether Radio is there and its switch is on.
func (a *App) radioOn() bool {
	return a.opts.Radio != nil && (a.opts.Prefs == nil || a.opts.Prefs.Bool(keys.RadioOn))
}

// shownTabs is the tabs shown with the switches as given: Home and the
// Library always, Spotify's and Podcasts with Spotify, and YouTube's and Downloads with
// YouTube, and Radio's with the Radio service and its switch.
func shownTabs(spotify, youtube, radio bool) []int {
	ids := []int{widgets.TabHome, widgets.TabLibrary}
	if spotify {
		ids = append(ids, widgets.TabSpotify, widgets.TabPodcasts)
	}
	ids = append(ids, widgets.TabAudiobooks)
	if youtube {
		ids = append(ids, widgets.TabYouTube, widgets.TabDownloads)
	}
	if radio {
		ids = append(ids, widgets.TabRadio)
	}
	return ids
}

// tabKeys are the switches of the tabs Settings' Top tabs page can hide:
// all but Home and the Library.
var tabKeys = map[int]string{
	widgets.TabSpotify:    keys.TabSpotify,
	widgets.TabPodcasts:   keys.TabPodcasts,
	widgets.TabAudiobooks: keys.TabAudiobooks,
	widgets.TabYouTube:    keys.TabYouTube,
	widgets.TabDownloads:  keys.TabDownloads,
	widgets.TabRadio:      keys.TabRadio,
}

// tabAbout is what the Top tabs page says of each tab.
var tabAbout = map[int]string{
	widgets.TabSpotify:    "Your Spotify playlists and search",
	widgets.TabPodcasts:   "Your Spotify podcasts",
	widgets.TabAudiobooks: "Books in the Audiobooks folder",
	widgets.TabYouTube:    "Search and download from YouTube Music",
	widgets.TabDownloads:  "The download queue",
	widgets.TabRadio:      "Internet radio stations",
}

var tabIcons = map[int]string{
	widgets.TabSpotify: "stream", widgets.TabPodcasts: "headphones", widgets.TabAudiobooks: "music",
	widgets.TabYouTube: "circleplay", widgets.TabDownloads: "download", widgets.TabRadio: "radio",
}

// tabOn reads a tab's switch; without Prefs every tab shows.
func (a *App) tabOn(key string) bool { return a.opts.Prefs == nil || a.opts.Prefs.Bool(key) }

// tabSwitches are the Top tabs page's switches: a switch for each tab its
// integration allows, in the row's order. Switching one off only hides it.
func (a *App) tabSwitches() []settingsscreen.Integration {
	var ins []settingsscreen.Integration
	for _, t := range shownTabs(a.spotifyOn(), a.youtubeOn(), a.radioOn()) {
		if k, ok := tabKeys[t]; ok {
			ins = append(ins, settingsscreen.Integration{Key: k, Name: widgets.TabName(t), Icon: tabIcons[t], About: tabAbout[t]})
		}
	}
	return ins
}

// integrations are the switches Settings' Integrations page lists: one for
// each integration there is.
func (a *App) integrations() []settingsscreen.Integration {
	var ins []settingsscreen.Integration
	if a.opts.Spotify != nil {
		ins = append(ins, settingsscreen.Integration{Key: keys.SpotifyOn, Name: spotifyName, Icon: "stream",
			About: "Stream your playlists and liked songs · needs Premium"})
	}
	if a.opts.YouTube != nil || a.opts.Downloads != nil {
		ins = append(ins, settingsscreen.Integration{Key: keys.YouTubeOn, Name: youtubeName, Icon: "circleplay",
			About: "Search YouTube Music and download songs"})
	}
	if a.opts.Radio != nil {
		ins = append(ins, settingsscreen.Integration{Key: keys.RadioOn, Name: radioName, Icon: "radio",
			About: "Stations from Radio Browser"})
	}
	return ins
}

// applySwitches makes everything follow the switches: the tabs shown, the
// download queue's hold, the importer, the player (whose gate, which
// newPlayer sets, reads the switches itself), what the screens offer and
// Home's rows. Switching on takes effect at once; switching off starts
// nothing new, and what is under way finishes.
func (a *App) applySwitches() {
	sp, yt := a.spotifyOn(), a.youtubeOn()
	tabs := slices.DeleteFunc(shownTabs(sp, yt, a.radioOn()), func(t int) bool {
		k, ok := tabKeys[t]
		return ok && !a.tabOn(k)
	})
	a.shell.setShown(tabs)
	a.ui.Tabs.Shown = tabs
	if a.opts.Hold != nil {
		a.opts.Hold(!yt)
	}
	if a.opts.ImportOn != nil {
		a.opts.ImportOn(sp && yt)
	}
	screens.SetDownloads(yt)
	if a.opts.Player != nil {
		a.opts.Player.Regate()
	}
	a.showHome(a.progress)
	a.dirty = true
}

// switched is Settings flipping key's switch. Switched off while it is busy,
// a toast says what finishes first.
func (a *App) switched(key string, on bool) {
	if !on {
		switch {
		case key == keys.SpotifyOn && a.last.Playing && a.last.Track.Streamed():
			a.Toast("This song finishes, then Spotify songs are skipped")
		case key == keys.YouTubeOn && a.opts.Downloads != nil && a.opts.Downloads.Busy():
			a.Toast("Downloads under way will finish")
		case key == keys.RadioOn && a.last.Playing && a.last.Track.Live():
			a.Toast("This station plays until you stop it")
		}
	}
	a.applySwitches()
}
