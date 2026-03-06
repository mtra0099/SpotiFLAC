# SpotiFLAC CLI

A command-line interface for downloading Spotify tracks as lossless FLAC files using Tidal, Qobuz, Amazon Music, and Deezer as audio sources — **no account required**.

This CLI wraps the SpotiFLAC backend package, allowing headless/server/programmatic use without the GUI.

---

## How It Works

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────────┐     ┌──────────┐
│ Spotify URL  │────▶│ Metadata Fetcher  │────▶│ Service Resolver │────▶│ Download │
│ (track/album │     │ (reverse-eng API) │     │ (Tidal/Qobuz/    │     │ as FLAC  │
│  /playlist)  │     │                   │     │  Amazon/Deezer)  │     │          │
└──────────────┘     └──────────────────┘     └──────────────────┘     └──────────┘
```

1. **Metadata fetch** — Uses Spotify's internal web API (reverse-engineered, no auth needed) to get track/album/playlist metadata: name, artist, album, cover art, ISRC, etc.
2. **Service resolution** — Maps the Spotify track to its equivalent on a lossless service (Tidal, Qobuz, Amazon, or Deezer) using song.link and MusicBrainz APIs.
3. **Download** — Downloads the lossless audio stream from the matched service, embeds metadata (artist, album, cover art, track number, copyright), and saves as FLAC.

---

## Build from Source

Requires **Go 1.22+**.

```bash
git clone https://github.com/afkarxyz/SpotiFLAC.git
cd SpotiFLAC
go build -o spotiflac-cli ./cmd/cli/

# Windows
go build -o spotiflac-cli.exe ./cmd/cli/
```

---

## Usage

```
spotiflac-cli <spotify-url> [OPTIONS]
```

### Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--service <name>` | `-s` | Download service | `tidal` |
| `--output <dir>` | `-o` | Output directory | `./downloads` |
| `--format <fmt>` | `-f` | Audio quality | `LOSSLESS` |
| `--metadata-only` | `-m` | Fetch metadata without downloading | off |
| `--json` | `-j` | Output structured JSON to stdout | off |
| `--help` | `-h` | Show help | — |

### Services

| Service | Quality | Notes |
|---------|---------|-------|
| `tidal` | 16-bit/44.1kHz FLAC | **Default.** Most reliable. |
| `qobuz` | Up to 24-bit/192kHz FLAC | Hi-Res available. Uses ISRC lookup. |
| `amazon` | 16-bit/44.1kHz FLAC | Uses Amazon Music catalog. |
| `deezer` | 16-bit/44.1kHz FLAC | Uses Deezer catalog. |

### Audio Format Values

| Value | Description |
|-------|-------------|
| `LOSSLESS` | CD quality (16-bit/44.1kHz) — default |
| `HI_RES` | High resolution (24-bit, where available) |
| `HI_RES_LOSSLESS` | Maximum quality available |

### Supported URLs

```
https://open.spotify.com/track/<id>
https://open.spotify.com/album/<id>
https://open.spotify.com/playlist/<id>
```

---

## Examples

### Download a single track

```bash
./spotiflac-cli "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT"
```

Output:
```
🎵 SpotiFLAC CLI
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
URL:     https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT
Service: tidal
Output:  ./downloads
Format:  LOSSLESS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📡 Fetching Spotify metadata...
✅ Track: Never Gonna Give You Up
   Artist: Rick Astley
   Album: Whenever You Need Somebody
   Spotify ID: 4cOdK2wGLETKBW3PvgPWqT

⬇️  Downloading via TIDAL...
✅ Downloaded: Never Gonna Give You Up - Rick Astley.flac
```

### Download via Qobuz with custom output

```bash
./spotiflac-cli "https://open.spotify.com/track/7qiZfU4dY1lWllzX7mPBI3" -s qobuz -o ~/music
```

### Download an entire album

```bash
./spotiflac-cli "https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3"
```

### Get metadata only (no download)

```bash
./spotiflac-cli "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT" -m
```

### JSON output for programmatic use

```bash
./spotiflac-cli "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT" -m -j
```

---

## JSON Output Format

When using `--json`, all output is structured JSON on stdout. Logs/progress go to stderr.

### Metadata-only response

```json
{
  "success": true,
  "type": "track",
  "metadata": {
    "spotify_id": "4cOdK2wGLETKBW3PvgPWqT",
    "name": "Never Gonna Give You Up",
    "artists": "Rick Astley",
    "album_name": "Whenever You Need Somebody",
    "duration_ms": 213573,
    "track_number": 1,
    "cover_url": "https://i.scdn.co/image/..."
  }
}
```

### Download response

```json
{
  "success": true,
  "type": "track",
  "tracks": [
    {
      "spotify_id": "4cOdK2wGLETKBW3PvgPWqT",
      "name": "Never Gonna Give You Up",
      "artist": "Rick Astley",
      "album": "Whenever You Need Somebody",
      "file_path": "./downloads/Never Gonna Give You Up - Rick Astley.flac",
      "status": "downloaded",
      "size_bytes": 26476800
    }
  ]
}
```

### Error response

```json
{
  "success": false,
  "error": "Failed to fetch metadata: context deadline exceeded"
}
```

### Track status values

| Status | Meaning |
|--------|---------|
| `downloaded` | Successfully downloaded |
| `exists` | File already existed, skipped |
| `failed` | Download failed (see `error` field) |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All tracks downloaded successfully |
| `1` | Some tracks failed (partial success) |
| `2` | Complete failure / fatal error |

---

## Programmatic Integration

The `--json` flag makes this CLI easy to call from other applications (Node.js, Python, etc.):

```typescript
// Node.js example
import { execFile } from 'child_process';

function downloadTrack(spotifyUrl: string, outputDir: string): Promise<any> {
  return new Promise((resolve, reject) => {
    execFile('./spotiflac-cli', [spotifyUrl, '-o', outputDir, '-j'], (err, stdout) => {
      if (err && err.code === 2) {
        reject(new Error('Download failed'));
        return;
      }
      resolve(JSON.parse(stdout));
    });
  });
}
```

---

## Disclaimer

This project is for **educational and private use only**. See the main [README](./README.md) for full disclaimer.
