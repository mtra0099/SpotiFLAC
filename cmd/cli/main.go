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

func main() {
	if len(os.Args) < 2 {
		fmt.Println("SpotiFLAC CLI - Download Spotify tracks as FLAC")
		fmt.Println()
		fmt.Println("Usage: spotiflac-cli <spotify-url> [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --service <tidal|qobuz|amazon|deezer>  (default: tidal)")
		fmt.Println("  --output <dir>                         (default: ./downloads)")
		fmt.Println("  --format <LOSSLESS|HI_RES>             (default: LOSSLESS)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  spotiflac-cli https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT")
		fmt.Println("  spotiflac-cli https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT --service qobuz")
		os.Exit(1)
	}

	spotifyURL := os.Args[1]
	service := "tidal"
	outputDir := "./downloads"
	audioFormat := "LOSSLESS"

	// Parse flags
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--service":
			if i+1 < len(os.Args) {
				service = os.Args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(os.Args) {
				outputDir = os.Args[i+1]
				i++
			}
		case "--format":
			if i+1 < len(os.Args) {
				audioFormat = os.Args[i+1]
				i++
			}
		}
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	fmt.Printf("🎵 SpotiFLAC CLI\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("URL:     %s\n", spotifyURL)
	fmt.Printf("Service: %s\n", service)
	fmt.Printf("Output:  %s\n", outputDir)
	fmt.Printf("Format:  %s\n", audioFormat)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Step 1: Fetch metadata from Spotify
	fmt.Println("📡 Fetching Spotify metadata...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	data, err := backend.GetFilteredSpotifyData(ctx, spotifyURL, false, 0)
	if err != nil {
		log.Fatalf("❌ Failed to fetch metadata: %v", err)
	}

	// Marshal then unmarshal to work with the data
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("❌ Failed to encode metadata: %v", err)
	}

	// Try parsing as a single track
	var trackResp struct {
		Track struct {
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
		} `json:"track"`
	}

	if err := json.Unmarshal(jsonData, &trackResp); err == nil && trackResp.Track.SpotifyID != "" {
		// Single track
		t := trackResp.Track
		fmt.Printf("✅ Track: %s\n", t.Name)
		fmt.Printf("   Artist: %s\n", t.Artists)
		fmt.Printf("   Album: %s\n", t.AlbumName)
		fmt.Printf("   Spotify ID: %s\n\n", t.SpotifyID)

		downloadTrack(service, t.SpotifyID, t.Name, t.Artists, t.AlbumName, t.AlbumArtist,
			t.ReleaseDate, t.CoverURL, outputDir, audioFormat, t.TrackNumber, t.DiscNumber,
			t.TotalTracks, t.TotalDiscs, t.Copyright, t.Publisher, t.Duration)
		return
	}

	// Try parsing as an album
	var albumResp struct {
		AlbumInfo struct {
			Name        string `json:"name"`
			Artists     string `json:"artists"`
			ReleaseDate string `json:"release_date"`
			Images      string `json:"images"`
			TotalTracks int    `json:"total_tracks"`
		} `json:"album_info"`
		TrackList []struct {
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
		} `json:"track_list"`
	}

	if err := json.Unmarshal(jsonData, &albumResp); err == nil && len(albumResp.TrackList) > 0 {
		fmt.Printf("✅ Album: %s by %s (%d tracks)\n\n", albumResp.AlbumInfo.Name, albumResp.AlbumInfo.Artists, len(albumResp.TrackList))

		for i, t := range albumResp.TrackList {
			fmt.Printf("━━━ Track %d/%d ━━━\n", i+1, len(albumResp.TrackList))
			fmt.Printf("   %s - %s\n", t.Name, t.Artists)

			downloadTrack(service, t.SpotifyID, t.Name, t.Artists, t.AlbumName, t.AlbumArtist,
				t.ReleaseDate, t.CoverURL, outputDir, audioFormat, t.TrackNumber, t.DiscNumber,
				t.TotalTracks, t.TotalDiscs, t.Copyright, t.Publisher, t.Duration)

			if i < len(albumResp.TrackList)-1 {
				fmt.Println()
				time.Sleep(1 * time.Second) // Be nice to APIs
			}
		}
		return
	}

	// Couldn't parse — dump metadata
	fmt.Println("ℹ️  Could not parse as track or album. Raw metadata:")
	prettyJSON, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(prettyJSON))
}

func downloadTrack(service, spotifyID, trackName, artistName, albumName, albumArtist,
	releaseDate, coverURL, outputDir, audioFormat string,
	trackNumber, discNumber, totalTracks, totalDiscs int,
	copyright, publisher string, durationMs int) {

	spotifyURL := fmt.Sprintf("https://open.spotify.com/track/%s", spotifyID)
	filenameFormat := "title-artist"

	fmt.Printf("⬇️  Downloading via %s...\n", strings.ToUpper(service))

	var filename string
	var err error

	switch service {
	case "tidal":
		downloader := backend.NewTidalDownloader("")
		filename, err = downloader.Download(spotifyID, outputDir, audioFormat, filenameFormat,
			false, 0, trackName, artistName, albumName, albumArtist, releaseDate,
			false, coverURL, false,
			trackNumber, discNumber, totalTracks, totalDiscs,
			copyright, publisher, spotifyURL, true, false, false, false)

	case "amazon":
		downloader := backend.NewAmazonDownloader()
		filename, err = downloader.DownloadBySpotifyID(spotifyID, outputDir, audioFormat, filenameFormat,
			"", "", false, 0, trackName, artistName, albumName, albumArtist, releaseDate,
			coverURL, trackNumber, discNumber, totalTracks, false, totalDiscs,
			copyright, publisher, spotifyURL, false, false, false)

	case "qobuz":
		// Need ISRC for Qobuz
		fmt.Println("   Fetching ISRC for Qobuz...")
		client := backend.NewSongLinkClient()
		isrc, _ := client.GetISRC(spotifyID)
		downloader := backend.NewQobuzDownloader()
		quality := audioFormat
		if quality == "" || quality == "LOSSLESS" {
			quality = "6"
		}
		filename, err = downloader.DownloadTrackWithISRC(isrc, spotifyID, outputDir, quality, filenameFormat,
			false, 0, trackName, artistName, albumName, albumArtist, releaseDate,
			false, coverURL, false,
			trackNumber, discNumber, totalTracks, totalDiscs,
			copyright, publisher, spotifyURL, true, false, false, false)

	case "deezer":
		downloader := backend.NewDeezerDownloader()
		filename, err = downloader.Download(spotifyID, outputDir, filenameFormat,
			"", "", false, 0, trackName, artistName, albumName, albumArtist, releaseDate,
			coverURL, trackNumber, discNumber, totalTracks, false, totalDiscs,
			copyright, publisher, spotifyURL, false, false, false)

	default:
		log.Fatalf("❌ Unknown service: %s", service)
	}

	if err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)

		// Clean up partial file
		if filename != "" && !strings.HasPrefix(filename, "EXISTS:") {
			if _, statErr := os.Stat(filename); statErr == nil {
				os.Remove(filename)
			}
		}
		return
	}

	alreadyExists := false
	if strings.HasPrefix(filename, "EXISTS:") {
		alreadyExists = true
		filename = strings.TrimPrefix(filename, "EXISTS:")
	}

	if alreadyExists {
		fmt.Printf("⏭️  Already exists: %s\n", filepath.Base(filename))
	} else {
		fmt.Printf("✅ Downloaded: %s\n", filepath.Base(filename))
	}
}
