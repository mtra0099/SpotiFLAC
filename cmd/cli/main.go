package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/afkarxyz/SpotiFLAC/backend"
)

// CLIResult is the structured output for --json mode
type CLIResult struct {
	Success  bool          `json:"success"`
	Type     string        `json:"type"` // "track", "album", "playlist"
	Metadata interface{}   `json:"metadata,omitempty"`
	Tracks   []TrackResult `json:"tracks,omitempty"`
	Error    string        `json:"error,omitempty"`
}

type TrackResult struct {
	SpotifyID string `json:"spotify_id"`
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	FilePath  string `json:"file_path,omitempty"`
	Status    string `json:"status"` // "downloaded", "exists", "failed"
	Error     string `json:"error,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

type config struct {
	spotifyURL   string
	service      string
	outputDir    string
	audioFormat  string
	metadataOnly bool
	jsonOutput   bool
}

func parseArgs() config {
	cfg := config{
		service:     "tidal",
		outputDir:   "./downloads",
		audioFormat: "LOSSLESS",
	}

	if len(os.Args) < 2 || os.Args[1] == "--help" || os.Args[1] == "-h" {
		printUsage()
		os.Exit(0)
	}

	cfg.spotifyURL = os.Args[1]

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--service", "-s":
			if i+1 < len(os.Args) {
				cfg.service = os.Args[i+1]
				i++
			}
		case "--output", "-o":
			if i+1 < len(os.Args) {
				cfg.outputDir = os.Args[i+1]
				i++
			}
		case "--format", "-f":
			if i+1 < len(os.Args) {
				cfg.audioFormat = os.Args[i+1]
				i++
			}
		case "--metadata-only", "-m":
			cfg.metadataOnly = true
		case "--json", "-j":
			cfg.jsonOutput = true
		}
	}

	return cfg
}

	fmt.Println(`SpotiFLAC CLI — Download Spotify tracks as lossless FLAC

USAGE:
  spotiflac-cli <spotify-url> [OPTIONS]

OPTIONS:
  -s, --service <name>     Download service: tidal (default), qobuz, amazon, deezer
  -o, --output <dir>       Output directory (default: ./downloads)
  -f, --format <fmt>       Audio format: LOSSLESS (default), HI_RES, HI_RES_LOSSLESS
  -m, --metadata-only      Fetch and display metadata without downloading
  -j, --json               Output structured JSON (for programmatic use)
  -h, --help               Show this help

SUPPORTED URLS:
  Track:     https://open.spotify.com/track/<id>
  Album:     https://open.spotify.com/album/<id>
  Playlist:  https://open.spotify.com/playlist/<id>

EXAMPLES:
  # Download a track via Tidal (default)
  spotiflac-cli https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT

  # Download via Qobuz to a custom directory
  spotiflac-cli https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT -s qobuz -o ~/music

  # Get track metadata as JSON (no download)
  spotiflac-cli https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT -m -j

  # Download an entire album
  spotiflac-cli https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3

EXIT CODES:
  0  All tracks downloaded successfully
  1  Some tracks failed
  2  Complete failure (no tracks downloaded or fatal error)`)
}

func main() {
	cfg := parseArgs()

	if !cfg.jsonOutput {
		fmt.Printf("🎵 SpotiFLAC CLI\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("URL:     %s\n", cfg.spotifyURL)
		fmt.Printf("Service: %s\n", cfg.service)
		fmt.Printf("Output:  %s\n", cfg.outputDir)
		fmt.Printf("Format:  %s\n", cfg.audioFormat)
		if cfg.metadataOnly {
			fmt.Printf("Mode:    metadata-only\n")
		}
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	}

	if !cfg.metadataOnly {
		if err := os.MkdirAll(cfg.outputDir, 0755); err != nil {
			exitError(cfg, fmt.Sprintf("Failed to create output directory: %v", err))
		}
	}

	// Fetch Spotify metadata
	if !cfg.jsonOutput {
		fmt.Println("📡 Fetching Spotify metadata...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	data, err := backend.GetFilteredSpotifyData(ctx, cfg.spotifyURL, false, 0)
	if err != nil {
		exitError(cfg, fmt.Sprintf("Failed to fetch metadata: %v", err))
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		exitError(cfg, fmt.Sprintf("Failed to encode metadata: %v", err))
	}

	// Try single track
	var trackResp trackResponse
	if json.Unmarshal(jsonData, &trackResp) == nil && trackResp.Track.SpotifyID != "" {
		handleSingleTrack(cfg, trackResp.Track)
		return
	}

	// Try album
	var albumResp albumResponse
	if json.Unmarshal(jsonData, &albumResp) == nil && len(albumResp.TrackList) > 0 {
		handleAlbum(cfg, albumResp)
		return
	}

	// Try playlist
	var playlistResp playlistResponse
	if json.Unmarshal(jsonData, &playlistResp) == nil && len(playlistResp.TrackList) > 0 {
		handlePlaylist(cfg, playlistResp)
		return
	}

	// Unknown format — dump raw
	if cfg.jsonOutput {
		result := CLIResult{Success: false, Error: "Could not parse Spotify response as track, album, or playlist"}
		outputJSON(result)
	} else {
		fmt.Println("⚠️  Could not parse as track, album, or playlist. Raw metadata:")
		prettyJSON, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(prettyJSON))
	}
	os.Exit(2)
}

// ── Data types for Spotify responses ────────────────────────────────

type trackMetadata struct {
	SpotifyID   string `json:"spotify_id"`
	Name        string `json:"name"`
	Artists     string `json:"artists"`
	AlbumName   string `json:"album_name"`
	AlbumArtist string `json:"album_artist"`
	ReleaseDate string `json:"release_date"`
	CoverURL    string `json:"cover_url"`
	Duration    int    `json:"duration_ms"`
	TrackNumber int    `json:"track_number"`
	DiscNumber  int    `json:"disc_number"`
	TotalTracks int    `json:"total_tracks"`
	TotalDiscs  int    `json:"total_discs"`
	Copyright   string `json:"copyright"`
	Publisher   string `json:"publisher"`
}

type trackResponse struct {
	Track trackMetadata `json:"track"`
}

type albumResponse struct {
	AlbumInfo struct {
		Name        string `json:"name"`
		Artists     string `json:"artists"`
		ReleaseDate string `json:"release_date"`
		Images      string `json:"images"`
		TotalTracks int    `json:"total_tracks"`
	} `json:"album_info"`
	TrackList []trackMetadata `json:"track_list"`
}

type playlistResponse struct {
	PlaylistInfo struct {
		Tracks struct {
			Total int `json:"total"`
		} `json:"tracks"`
		Owner struct {
			DisplayName string `json:"display_name"`
		} `json:"owner"`
	} `json:"playlist_info"`
	TrackList []trackMetadata `json:"track_list"`
}

// ── Handlers ────────────────────────────────────────────────────────

func handleSingleTrack(cfg config, t trackMetadata) {
	if !cfg.jsonOutput {
		fmt.Printf("✅ Track: %s\n", t.Name)
		fmt.Printf("   Artist: %s\n", t.Artists)
		fmt.Printf("   Album: %s\n", t.AlbumName)
		fmt.Printf("   Spotify ID: %s\n\n", t.SpotifyID)
	}

	if cfg.metadataOnly {
		if cfg.jsonOutput {
			result := CLIResult{Success: true, Type: "track", Metadata: t}
			outputJSON(result)
		}
		return
	}

	tr := downloadAndReport(cfg, t)
	if cfg.jsonOutput {
		result := CLIResult{
			Success: tr.Status != "failed",
			Type:    "track",
			Tracks:  []TrackResult{tr},
		}
		outputJSON(result)
	}

	if tr.Status == "failed" {
		os.Exit(2)
	}
}

func handleAlbum(cfg config, album albumResponse) {
	if !cfg.jsonOutput {
		fmt.Printf("✅ Album: %s by %s (%d tracks)\n\n",
			album.AlbumInfo.Name, album.AlbumInfo.Artists, len(album.TrackList))
	}

	if cfg.metadataOnly {
		if cfg.jsonOutput {
			result := CLIResult{Success: true, Type: "album", Metadata: album}
			outputJSON(result)
		}
		return
	}

	results := downloadMultiple(cfg, album.TrackList, "album")

	if cfg.jsonOutput {
		failed := 0
		for _, r := range results {
			if r.Status == "failed" {
				failed++
			}
		}
		result := CLIResult{
			Success: failed == 0,
			Type:    "album",
			Tracks:  results,
		}
		outputJSON(result)
		exitWithCode(failed, len(results))
	} else {
		printSummary(results)
	}
}

func handlePlaylist(cfg config, playlist playlistResponse) {
	if !cfg.jsonOutput {
		fmt.Printf("✅ Playlist by %s (%d tracks)\n\n",
			playlist.PlaylistInfo.Owner.DisplayName, len(playlist.TrackList))
	}

	if cfg.metadataOnly {
		if cfg.jsonOutput {
			result := CLIResult{Success: true, Type: "playlist", Metadata: playlist}
			outputJSON(result)
		}
		return
	}

	results := downloadMultiple(cfg, playlist.TrackList, "playlist")

	if cfg.jsonOutput {
		failed := 0
		for _, r := range results {
			if r.Status == "failed" {
				failed++
			}
		}
		result := CLIResult{
			Success: failed == 0,
			Type:    "playlist",
			Tracks:  results,
		}
		outputJSON(result)
		exitWithCode(failed, len(results))
	} else {
		printSummary(results)
	}
}

// ── Download logic ──────────────────────────────────────────────────

func downloadMultiple(cfg config, tracks []trackMetadata, kind string) []TrackResult {
	var results []TrackResult

	for i, t := range tracks {
		if !cfg.jsonOutput {
			fmt.Printf("━━━ %s %d/%d ━━━\n", strings.Title(kind), i+1, len(tracks))
			fmt.Printf("   %s — %s\n", t.Name, t.Artists)
		}

		tr := downloadAndReport(cfg, t)
		results = append(results, tr)

		if !cfg.jsonOutput && i < len(tracks)-1 {
			fmt.Println()
		}
		if i < len(tracks)-1 {
			time.Sleep(1 * time.Second)
		}
	}
	return results
}

func downloadAndReport(cfg config, t trackMetadata) TrackResult {
	tr := TrackResult{
		SpotifyID: t.SpotifyID,
		Name:      t.Name,
		Artist:    t.Artists,
		Album:     t.AlbumName,
	}

	if !cfg.jsonOutput {
		fmt.Printf("⬇️  Downloading via %s...\n", strings.ToUpper(cfg.service))
	}

	filename, err := downloadTrack(cfg.service, t.SpotifyID, t.Name, t.Artists, t.AlbumName,
		t.AlbumArtist, t.ReleaseDate, t.CoverURL, cfg.outputDir, cfg.audioFormat,
		t.TrackNumber, t.DiscNumber, t.TotalTracks, t.TotalDiscs, t.Copyright, t.Publisher)

	if err != nil {
		tr.Status = "failed"
		tr.Error = err.Error()
		if !cfg.jsonOutput {
			fmt.Printf("❌ Download failed: %v\n", err)
		}
		// Clean up partial file
		if filename != "" && !strings.HasPrefix(filename, "EXISTS:") {
			if _, statErr := os.Stat(filename); statErr == nil {
				os.Remove(filename)
			}
		}
		return tr
	}

	if strings.HasPrefix(filename, "EXISTS:") {
		filename = strings.TrimPrefix(filename, "EXISTS:")
		tr.Status = "exists"
		if !cfg.jsonOutput {
			fmt.Printf("⏭️  Already exists: %s\n", filepath.Base(filename))
		}
	} else {
		tr.Status = "downloaded"
		if !cfg.jsonOutput {
			fmt.Printf("✅ Downloaded: %s\n", filepath.Base(filename))
		}
	}

	tr.FilePath = filename
	if info, err := os.Stat(filename); err == nil {
		tr.SizeBytes = info.Size()
	}

	return tr
}

func downloadTrack(service, spotifyID, trackName, artistName, albumName, albumArtist,
	releaseDate, coverURL, outputDir, audioFormat string,
	trackNumber, discNumber, totalTracks, totalDiscs int,
	copyright, publisher string) (string, error) {

	spotifyURL := fmt.Sprintf("https://open.spotify.com/track/%s", spotifyID)
	filenameFormat := "title-artist"

	switch service {
	case "tidal":
		downloader := backend.NewTidalDownloader("")
		return downloader.Download(spotifyID, outputDir, audioFormat, filenameFormat,
			false, 0, trackName, artistName, albumName, albumArtist, releaseDate,
			false, coverURL, false,
			trackNumber, discNumber, totalTracks, totalDiscs,
			copyright, publisher, spotifyURL, true, false, false, false)

	case "amazon":
		downloader := backend.NewAmazonDownloader()
		return downloader.DownloadBySpotifyID(spotifyID, outputDir, audioFormat, filenameFormat,
			"", "", false, 0, trackName, artistName, albumName, albumArtist, releaseDate,
			coverURL, trackNumber, discNumber, totalTracks, false, totalDiscs,
			copyright, publisher, spotifyURL, false, false, false)

	case "qobuz":
		client := backend.NewSongLinkClient()
		isrc, _ := client.GetISRC(spotifyID)
		downloader := backend.NewQobuzDownloader()
		quality := audioFormat
		if quality == "" || quality == "LOSSLESS" {
			quality = "6"
		}
		return downloader.DownloadTrackWithISRC(isrc, spotifyID, outputDir, quality, filenameFormat,
			false, 0, trackName, artistName, albumName, albumArtist, releaseDate,
			false, coverURL, false,
			trackNumber, discNumber, totalTracks, totalDiscs,
			copyright, publisher, spotifyURL, true, false, false, false)

	case "deezer":
		downloader := backend.NewDeezerDownloader()
		return downloader.Download(spotifyID, outputDir, filenameFormat,
			"", "", false, 0, trackName, artistName, albumName, albumArtist, releaseDate,
			coverURL, trackNumber, discNumber, totalTracks, false, totalDiscs,
			copyright, publisher, spotifyURL, false, false, false)

	default:
		return "", fmt.Errorf("unknown service: %s", service)
	}
}

// ── Output helpers ──────────────────────────────────────────────────

func outputJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func printSummary(results []TrackResult) {
	downloaded, exists, failed := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "downloaded":
			downloaded++
		case "exists":
			exists++
		case "failed":
			failed++
		}
	}

	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📊 Summary: %d downloaded, %d already existed, %d failed\n", downloaded, exists, failed)

	exitWithCode(failed, len(results))
}

func exitWithCode(failed, total int) {
	if failed == 0 {
		os.Exit(0)
	} else if failed < total {
		os.Exit(1)
	} else {
		os.Exit(2)
	}
}

func exitError(cfg config, msg string) {
	if cfg.jsonOutput {
		result := CLIResult{Success: false, Error: msg}
		outputJSON(result)
	} else {
		log.Fatal("❌ " + msg)
	}
	os.Exit(2)
}
