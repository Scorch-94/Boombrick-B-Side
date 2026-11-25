# Boombrick

> **Boombrick B-Side** is a fork of [ujjwal313/boombrick](https://github.com/ujjwal313/boombrick) for NextUI, with direct transport controls on Now Playing, library search, `.m3u` playlists and Shuffle all songs. It installs beside the original: it is `Tools/tg5040/Boombrick B-Side.pak`, keeps its library and settings in `.userdata/tg5040/Boombrick B-Side/` and logs to `Boombrick B-Side.txt`, and Settings checks this fork for its updates. Both read the same `Music` folder, and downloads from either go to `Music/Boombrick`.
>
> To install, download `Boombrick-B-Side.pak.zip` from this fork's [latest release](https://github.com/Scorch-94/Boombrick-B-Side/releases/latest) and unzip it at the root of the SD card.

A music player for the [TrimUI Brick](https://trimui.com) running [NextUI](https://github.com/LoveRetro/NextUI). It plays the music on your SD card, streams your Spotify playlists, finds covers and synced lyrics, and downloads songs and albums from YouTube Music for offline listening. Its colours come from the album that's playing.

<p align="center">
  <img src=".github/resources/screenshots/overview.png" alt="Boombrick's screens: Now Playing, synced lyrics, the library, YouTube Music, downloads and the queue">
</p>

See [CHANGELOG.md](CHANGELOG.md) for what each release adds.

## Features

- **Your music:** MP3, FLAC, Opus and M4A (AAC), gapless, with ReplayGain levelling. Browse by album, artist, song or genre; the queue has shuffle and repeat and is kept between launches.
- **Covers** from the files, folder pictures or the Cover Art Archive.
- **Synced lyrics** from the files, `.lrc` files or LRCLIB, with the line being sung highlighted.
- **Every script:** titles and lyrics in Japanese, Chinese, Korean, Arabic, Hebrew, Thai, Hindi and the other main Indian scripts, and emoji. Each script's font is downloaded over Wi-Fi the first time it's needed, then kept on the card.
- **YouTube Music:** search, browse albums and artists, and download songs as tagged Opus files with covers and lyrics. A song that is still downloading plays as soon as it lands.
- **Spotify** (Premium): sign in with a code on your phone, then play Liked Songs and your playlists, Spotify's own mixes included, in the same queue as your music. Streams play at 96, 160 or 320 kbps over Wi-Fi.
- **Spotify import:** download your playlists' songs, or Liked Songs, from YouTube Music to keep them offline. Each song is matched by its ISRC, or its title, artist and length, and you check the doubtful ones. A Spotify song with a downloaded copy plays the copy, without Wi-Fi.
- **Sleep timer:** 15 to 60 minutes, or the end of the track or album, with a fade.
- **Play in background** from the quick menu: the music goes on after you leave Boombrick, as while you play a game, with the volume keys as usual. Open Boombrick again to take it back.
- **Screen off** with Power, with the music still playing and the buttons for skipping and pausing still working.
- **Updates** for Boombrick and its downloader, from Settings.

## Installing

You need a TrimUI Brick or Brick Pro with NextUI, CrossMix or Knulli. Wi-Fi is used for Spotify, YouTube Music, covers, lyrics, fonts and updates; music on the card plays without it. Spotify needs a Premium account.

1. Download the zip for your firmware:
   - **NextUI:** `Boombrick.pak.zip` from the [latest release](https://github.com/ujjwal313/boombrick/releases/latest).
   - **CrossMix:** `Boombrick-CrossMix.zip` from the same release.
   - **Knulli:** `Boombrick-Knulli.zip` from the same release.

   `Boombrick-PakStore.zip` holds the same NextUI app for the Pak Store, which unzips it into the app's folder; it isn't for the root of the card.
2. Unzip it at the root of the SD card; on Knulli, at the root of the SHARE partition. It goes to `Tools/tg5040/Boombrick.pak` on NextUI, `Apps/Boombrick` on CrossMix, and `roms/ports` on Knulli, where it adds `Boombrick.sh` and a `boombrick` folder. Its folders join the ones already on the card: your other apps and ports stay as they are.
3. Put your music in the card's `Music` folder, making it if it isn't there; the zip doesn't hold one. On Knulli it's the `music` folder at the root of the SHARE partition, beside `roms` rather than inside it, which Knulli also plays in its menus when its background music is on. Any layout of folders inside it works; `Artist/Album/01 Title.flac` works best when files have no tags. The empty library shows the folder Boombrick reads, and Y there scans it again.
4. Start Boombrick from Tools on NextUI, from Apps on CrossMix, or from Ports on Knulli (update the game lists first if it isn't there). The library is scanned on the first launch, and again on every launch after that.

Betas are pre-releases: Settings never offers them, so they are only installed by hand. A beta is offered the next full release like any other version.

### Updating

Open Settings from the quick menu (Menu). When a newer release is out, the Boombrick card offers **Update Boombrick**. The update downloads, is checked, and is installed the next time Boombrick starts; choose **Quit now** and open it again.

The Downloader card updates yt-dlp, which YouTube Music downloads use, the same way.

A `.csv` playlist, as Exportify, Soundiiz or TuneMyMusic export one, is read by its header row: each song is found by its Spotify track (for songs the Spotify import downloaded), then by title and artist, with the album and length telling versions apart; a title's extras, like `(From "…")` or ` - Piano Version`, are set aside when that leaves only one song. A `Path` column works as an `.m3u` does. A song you don't have that the file gives a Spotify track for plays from Spotify instead (marked "Spotify" on the playlist's page), so the whole playlist plays as long as Spotify is set up and you're online; without Spotify those songs are skipped. Underscores in the file's name become spaces in the playlist's.

## Podcasts

With Spotify signed in, the **Podcasts** tab lists the podcasts on your Spotify account: **Continue** (episodes you are part way through, on any device), **Shows** (the podcasts you follow) and **Your Episodes** (the ones you saved). L2 and R2 change between them. A plays an episode from where you left off; Y adds it to the queue. Your place goes back to Spotify every 30 seconds while an episode plays, when you pause and when you move on, so the Spotify app on your phone carries on from there. On Now Playing, Left and Right jump 30 seconds back and forward during an episode.

Only episodes Spotify hosts itself play; the ones it only links to, on the podcast's own host, show "Not playable yet".

## Playlists

Put `.m3u`, `.m3u8` or `.csv` files in a `Playlists` folder inside your music folder (`Music/Playlists/` on the SD card). They show under **Playlists** in the Library (called **Lists** when Genres also shows), in the order the file lists them. Each line can be a path relative to the playlist (`../Artist/Album/01 Song.flac`), relative to the music folder (`Artist/Album/01 Song.flac`), or absolute; paths written on a PC (`C:\Users\me\Music\...`) or another device (`/storage/.../Music/...`) work too, from their `Music` folder on. Songs the library doesn't have are counted as "not found". Playlists are read again on every launch and rescan; on an empty Playlists filter, Y rescans.

## Controls

| Button | Action |
|---|---|
| D-pad | Move |
| A | Play or open |
| B | Back |
| X | Download (YouTube Music); search (Library) |
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

On Knulli, Power is Knulli's own: a press puts the Brick to sleep and holding it shuts down. Turn the screen off from the quick menu instead; any button other than A, L1 and R1 turns it back on. Menu and Start together quit, as they do Knulli's other ports.

## Files

On the SD card:

| What | NextUI | CrossMix | Knulli (SHARE) |
|---|---|---|---|
| Your music | `Music/` | `Music/` | `music/` |
| Downloads | `Music/Boombrick/<Artist>/<Album>/` | `Music/Boombrick/<Artist>/<Album>/` | `music/Boombrick/<Artist>/<Album>/` |
| Library, covers, lyrics, downloader, Spotify sign-in | `.userdata/tg5040/Boombrick/` | `Apps/Boombrick/data/` | `roms/ports/boombrick/data/` |
| Log | `.userdata/tg5040/logs/Boombrick.txt` | `Apps/Boombrick/logs/Boombrick.txt` | `roms/ports/boombrick/logs/Boombrick.txt` |

Settings → Storage shows how much room the caches take and clears them. Your music and downloads are never touched.

An empty file named `perf` in Boombrick's folder (`Tools/tg5040/Boombrick.pak/`, `Apps/Boombrick/` or `roms/ports/boombrick/`) turns on perf mode, which logs launch times and, every 10 seconds, memory, CPU and frame times.

## Building

Development happens on Linux. On Windows, use WSL with Ubuntu 24.04.

```
bash scripts/setup-wsl.sh   # once: Go, SDL2 headers, build tools
make test
make run                    # desktop window at 1024x768
```

The device build needs Docker:

```
make pak VERSION=0.1.1      # dist/Boombrick.pak.zip, for NextUI
make crossmix VERSION=0.1.1 # dist/Boombrick-CrossMix.zip, for CrossMix
make knulli VERSION=0.1.1   # dist/Boombrick-Knulli.zip, for Knulli
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

Add a section for the version to [CHANGELOG.md](CHANGELOG.md), then push a tag such as `v0.1.2`. The Release workflow builds the zips with that version and publishes them, with their `SHA256SUMS` and the changelog section as notes. Bricks running an older version are offered it in Settings.

A tag with a suffix, such as `v0.2.0-beta.1`, is published as a pre-release, which Settings never offers: it is for testers, who install it by hand.

## Support

Boombrick is free. If you'd like to help keep it going, you can [buy me a coffee](https://buymeacoffee.com/ujjwal_).

## Licence

GPL-3.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
