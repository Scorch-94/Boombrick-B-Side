# Boombrick B-Side

> [!NOTE]
> **This is a vibecoded fork.** Boombrick B-Side is [Boombrick](https://github.com/ujjwal313/boombrick) by [ujjwal313](https://github.com/ujjwal313), changed to suit one person's taste. The changes were written by an AI coding assistant (Claude) from plain-language requests, and tried out on a TrimUI Brick. Expect rough edges, and please report problems with anything listed under [What B-Side adds](#what-b-side-adds) or [Smaller changes](#smaller-changes) [here](https://github.com/Scorch-94/Boombrick-B-Side/issues), not to the original project. Everything else is the original's work: if you like the player, the credit is ujjwal313's.

A music player for the [TrimUI Brick](https://trimui.com) running [NextUI](https://github.com/LoveRetro/NextUI). It plays the music on your SD card, streams your Spotify playlists and podcasts, finds covers and synced lyrics, and downloads songs and albums from YouTube Music for offline listening. Its colours come from the album that's playing.

B-Side installs beside the original Boombrick, so you can keep both. It has its own folder, library, settings and log, and updates itself from this fork's releases. Both read the same `Music` folder.

<p align="center">
  <img src=".github/resources/screenshots/overview.png" alt="Boombrick's screens: Now Playing, synced lyrics, the library, YouTube Music, downloads and the queue">
</p>

The screenshots are of the original Boombrick. See [CHANGELOG.md](CHANGELOG.md) for what each B-Side release adds.

## What B-Side adds

- **Podcasts tab:** your Spotify podcasts, in three lists: episodes you're part way through on any device, the shows you follow, and your saved episodes. Episodes play from where you left off, and your place goes back to Spotify as you listen, so your phone picks up from there. Left and Right on Now Playing jump 30 seconds during an episode. [More below](#podcasts).
- **Bluetooth headphone controls:** play/pause, next and previous from headphones and speakers, AirPods' and Beats' taps included, with the screen on or off and in background play. Boombrick tells them what's playing, for headphones that pause when you take an earbud out. [More below](#bluetooth-headphones).
- **Playlists:** `.m3u`, `.m3u8` and `.csv` files in `Music/Playlists` become playlists in the Library. A `.csv` exported from Spotify by Exportify or TuneMyMusic is matched to your songs by Spotify track, title, artist, album and length, and songs you don't have stream from Spotify in their place. [More below](#playlists).
- **No duplicate downloads:** YouTube Music marks songs already in your library, from anywhere in your music folder, with "In library", and downloading skips them (X again downloads anyway). Playing one plays your copy.
- **Library search:** X in the Library searches artists, albums and songs as you type.
- **Shuffle all songs:** at the top of the Library's Songs list, and as a card on Home. It plays only the songs on your SD card, never Spotify streams.
- **A Home you arrange:** switch each section on or off and put them in your order, in Settings → Home sections. Besides the original's sections, Home can show:
  - **Shuffle**: all your songs, shuffled.
  - **Random albums**: six albums picked afresh each time the app opens.
  - **Playlists**: your playlist files.
  - **Podcasts**: the shows you follow.
  - Spotify **podcast episodes you've started**, in Continue listening.
- **Album view setting:** the Library's albums as a 4 × 2 grid (the default), three across with big covers as in the original, or a list.
- **Clock setting:** the time at the top left on the 24-hour or the 12-hour clock.

## Smaller changes

Compared with the original Boombrick:

- **Now Playing controls:**
  - Left and Right are previous and next track.
  - A plays or pauses.
  - Up switches shuffle and Down steps repeat; X no longer does anything there.
  - L1 and R1 still skip as well.
- **Now Playing looks:** play/pause is no longer highlighted all the time; each control flashes when its button is pressed. The Lyrics button under the controls is gone: Y Lyrics shows in the hint bar instead.
- **Album, genre and playlist pages:** they have no Play or Shuffle buttons. They open on the first track; A plays from the track you're on and Y queues it.
- **Choosing something to play opens Now Playing:** a song, album, playlist, episode or station. B goes back to where you were.
- **Library albums:** a 4 × 2 grid with smaller covers, rather than three across. The original's view is a setting.
- **Random albums on Home:** each card shows the album's title and artist.
- **NextUI only:** B-Side isn't built for CrossMix or Knulli.
- **Its own name and folders:** `Boombrick B-Side`, so that it never replaces the original and the original's updates never replace it.

## Installing

You need a TrimUI Brick or Brick Pro with NextUI. Wi-Fi is used for Spotify, YouTube Music, covers, lyrics, fonts and updates; music on the card plays without it. Spotify needs a Premium account.

1. Download `Boombrick-B-Side.pak.zip` from the [latest release](https://github.com/Scorch-94/Boombrick-B-Side/releases/latest). (`Boombrick-B-Side-PakStore.zip` is the same app for the Pak Store, which unzips it into the app's folder; it isn't for the root of the card.)
2. Unzip it at the root of the SD card. It goes to `Tools/tg5040/Boombrick B-Side.pak`; your other apps, the original Boombrick included, stay as they are.
3. Put your music in the card's `Music` folder, making it if it isn't there. Any layout of folders inside it works; `Artist/Album/01 Title.flac` works best when files have no tags. The empty library shows the folder B-Side reads, and Y there scans it again.
4. Start **Boombrick B-Side** from Tools. The library is scanned on the first launch, and again on every launch after that.

### Updating

Open Settings from the quick menu (Menu). When a newer release is out, the app's card offers **Update Boombrick**. The update downloads, is checked, and is installed the next time B-Side starts; choose **Quit now** and open it again.

Versions 0.7.0 to 0.7.2 checked the original Boombrick's releases by mistake, so their Update button stalls: install 0.7.3 or later from its zip once, and updates work from Settings after that.

The Downloader card updates yt-dlp, which YouTube Music downloads use, the same way.

## Features

All of the original Boombrick's features are here:

- **Your music:** MP3, FLAC, Opus and M4A (AAC), gapless, with ReplayGain levelling. Browse by album, artist, song, playlist or genre; the queue has shuffle and repeat and is kept between launches.
- **Covers** from the files, folder pictures or the Cover Art Archive.
- **Synced lyrics** from the files, `.lrc` files or LRCLIB, with the line being sung highlighted.
- **Every script:** titles and lyrics in Japanese, Chinese, Korean, Arabic, Hebrew, Thai, Hindi and the other main Indian scripts, and emoji. Each script's font is downloaded over Wi-Fi the first time it's needed, then kept on the card.
- **YouTube Music:** search, browse albums and artists, and download songs as tagged Opus files with covers and lyrics. A song that is still downloading plays as soon as it lands.
- **Spotify** (Premium): sign in with a code on your phone, then play Liked Songs and your playlists, Spotify's own mixes included, in the same queue as your music. Streams play at 96, 160 or 320 kbps over Wi-Fi.
- **Spotify import:** download your playlists' songs, or Liked Songs, from YouTube Music to keep them offline. Each song is matched by its ISRC, or its title, artist and length, and you check the doubtful ones. A Spotify song with a downloaded copy plays the copy, without Wi-Fi.
- **Internet radio** from Radio Browser, with stations you save on Home.
- **Sleep timer:** 15 to 60 minutes, or the end of the track or album, with a fade.
- **Play in background** from the quick menu: the music goes on after you leave B-Side, as while you play a game, with the volume keys as usual. Open B-Side again to take it back.
- **Screen off** with Power, with the music still playing and the buttons for skipping and pausing still working.
- **Updates** for B-Side and its downloader, from Settings.

## Podcasts

With Spotify signed in, the **Podcasts** tab lists the podcasts on your Spotify account: **Continue** (episodes you are part way through, on any device), **Shows** (the podcasts you follow) and **Your Episodes** (the ones you saved). L2 and R2 change between them. A show lists its newest 200 episodes. A plays an episode from where you left off; Y adds it to the queue. Your place goes back to Spotify every 30 seconds while an episode plays, when you pause and when you move on, so the Spotify app on your phone carries on from there. On Now Playing, Left and Right jump 30 seconds back and forward during an episode.

Only episodes Spotify hosts itself play; the ones it only links to, on the podcast's own host, show "Not playable yet".

## Playlists

Put `.m3u`, `.m3u8` or `.csv` files in a `Playlists` folder inside your music folder (`Music/Playlists/` on the SD card). They show under **Playlists** in the Library (called **Lists** when Genres also shows), and on Home, in the order the file lists them. Playlists are read again on every launch and rescan; on an empty Playlists filter, Y rescans.

In an `.m3u`, each line can be a path relative to the playlist (`../Artist/Album/01 Song.flac`), relative to the music folder (`Artist/Album/01 Song.flac`), or absolute; paths written on a PC (`C:\Users\me\Music\...`) or another device (`/storage/.../Music/...`) work too, from their `Music` folder on. Songs the library doesn't have are counted as "not found".

A `.csv` playlist, as Exportify, TuneMyMusic or Soundiiz export one, is read by its header row: each song is found by its Spotify track (a `spotify:track:` URI, an `open.spotify.com` link or a bare ID, for songs the Spotify import downloaded), then by title and artist, with the album and length telling versions apart; a title's extras, like `(From "…")` or ` - Piano Version`, are set aside when that leaves only one song and its album and length don't say it's another version. A file with a `Playlist name` column, as TuneMyMusic writes, becomes one playlist for each name in it, called by that name; rows without one make a playlist named after the file. A `Path` column works as an `.m3u` does. A song you don't have that the file gives a Spotify track for plays from Spotify instead (marked "Spotify" on the playlist's page), so the whole playlist plays as long as Spotify is set up and you're online; without Spotify those songs are skipped. Underscores in the file's name become spaces in the playlist's.

## Bluetooth headphones

Headphones and speakers connected in NextUI's settings control playback, with the screen on or off and in background play: play/pause, next and previous play, pause and skip, and on a podcast episode skip 30 seconds back and on. On AirPods and Beats these are the taps you've set for each earbud. B-Side registers with the Brick's Bluetooth service as the media player, so headphones know whether music is playing, as their pause on taking an earbud out needs, and can show the song's title and artist.

If a button doesn't seem to do anything, each command that arrives is written to the log as a `bluetooth remote` line: a button with no line never reached the Brick.

## Controls

| Button | Action |
|---|---|
| D-pad | Move |
| A | Play or open |
| B | Back |
| X | Download (YouTube Music); search (Library); move a section (Settings → Home sections) |
| Y | Add to the queue; on Now Playing, lyrics |
| D-pad Left, Right (Now Playing) | Previous and next track; 30 s back and forward in a podcast episode |
| A (Now Playing) | Play or pause |
| D-pad Up (Now Playing) | Shuffle on or off |
| D-pad Down (Now Playing) | Step repeat: off, all, one |
| L1, R1 | Switch tabs; on Now Playing, previous and next track |
| L2, R2 | Filters in the Library and YouTube Music; on Now Playing and Lyrics, 10 s back and on, or hold to rewind and fast-forward |
| Start | Now Playing |
| Select | Queue |
| Menu | Quick menu: sleep timer, screen off, play in background, Settings, quit |
| Power | Screen off and on; hold for a second to power off |

With the screen off, L1 and R1 skip back and on, and A plays or pauses. Each screen shows its buttons along the bottom.

## Files

On the SD card:

| What | Where |
|---|---|
| The app | `Tools/tg5040/Boombrick B-Side.pak/` |
| Your music | `Music/` |
| Playlists | `Music/Playlists/` |
| Downloads | `Music/Boombrick/<Artist>/<Album>/`, shared with the original Boombrick |
| Library, settings, covers, lyrics, downloader, Spotify sign-in | `.userdata/tg5040/Boombrick B-Side/` |
| Log | `.userdata/tg5040/logs/Boombrick B-Side.txt` |

Settings → Storage shows how much room the caches take and clears them. Your music and downloads are never touched.

An empty file named `perf` in the app's folder (`Tools/tg5040/Boombrick B-Side.pak/`) turns on perf mode, which logs launch times and, every 10 seconds, memory, CPU and frame times.

## Building

Development happens on Linux. On Windows, use WSL with Ubuntu 24.04.

```
bash scripts/setup-wsl.sh   # once: Go, SDL2 headers, build tools
make test
make run                    # desktop window at 1024x768
```

The device build needs Docker:

```
make pak VERSION=0.7.5      # dist/Boombrick-B-Side.pak.zip and dist/Boombrick-B-Side-PakStore.zip
```

On the desktop the keyboard stands in for the Brick's buttons:

| Brick | Keyboard |
|---|---|
| D-pad | Arrow keys |
| A, B, X, Y | K, J, I, U |
| L1, R1 | Q, E |
| L2, R2 | 1, 3 |
| Start, Select | Return, Tab |
| Menu | Escape |
| Power | P (holding it quits) |
| Volume down, up | -, = |

`BOOMBRICK_PERF=1 make run` turns on perf mode.

### Releasing

Add a section for the version to [CHANGELOG.md](CHANGELOG.md) and set the same version in `pak.json`, then publish a GitHub release with a tag such as `v0.7.6` (`V0.7.6` works too, from v0.7.5 on, but Bricks on older versions only see a small v). The Release workflow builds the zips with that version and attaches them to the release, with their `SHA256SUMS`. Bricks running an older version are offered it in Settings.

A tag with a suffix, such as `v0.8.0-beta.1`, is published as a pre-release, which Settings never offers: it is for testers, who install it by hand.

## Support

Boombrick is ujjwal313's work and free. If you'd like to help keep it going, you can [buy them a coffee](https://buymeacoffee.com/ujjwal_).

## Licence

GPL-3.0, as the original. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
