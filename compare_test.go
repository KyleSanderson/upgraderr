package main

import (
	"testing"

	"github.com/autobrr/go-qbittorrent"
	"github.com/moistari/rls"
)

func TestCompareResults(t *testing.T) {
	// Create test entries
	requestRls := rls.ParseString("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP")
	childRls := rls.ParseString("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GROUP")

	requestEntry := &Entry{
		t: qbittorrent.Torrent{Name: "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP"},
		r: &requestRls,
	}

	childEntry := &Entry{
		t: qbittorrent.Torrent{Name: "Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GROUP"},
		r: &childRls,
	}

	// Test when child value is higher
	result := compareResults(requestEntry, childEntry, func(r *rls.Release) int {
		if r.Resolution == "2160p" {
			return 2160
		}
		return 1080
	})

	if result != childEntry {
		t.Errorf("compareResults should return childEntry when child value is higher")
	}

	// Test when request value is higher
	result = compareResults(requestEntry, childEntry, func(r *rls.Release) int {
		if r.Resolution == "1080p" {
			return 2000
		}
		return 1000
	})

	if result != requestEntry {
		t.Errorf("compareResults should return requestEntry when request value is higher")
	}

	// Test when values are equal
	result = compareResults(requestEntry, childEntry, func(r *rls.Release) int {
		return 1000
	})

	if result != nil {
		t.Errorf("compareResults should return nil when values are equal")
	}
}

func TestCheckResolution(t *testing.T) {
	tests := []struct {
		name          string
		request       string
		existing      string
		expectUpgrade bool
	}{
		{
			name:          "Higher resolution is upgrade",
			request:       "Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP",
			expectUpgrade: true,
		},
		{
			name:          "Lower resolution is not upgrade",
			request:       "Movie.2023.720p.WEB-DL.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP",
			expectUpgrade: false,
		},
		{
			name:          "Same resolution is not upgrade",
			request:       "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP2",
			expectUpgrade: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestRls := rls.ParseString(test.request)
			existingRls := rls.ParseString(test.existing)

			requestEntry := &Entry{
				t: qbittorrent.Torrent{Name: test.request},
				r: &requestRls,
			}

			existingEntry := &Entry{
				t: qbittorrent.Torrent{Name: test.existing},
				r: &existingRls,
			}

			result := checkResolution(requestEntry, existingEntry)

			if test.expectUpgrade && (result == nil || result.t.Name != test.request) {
				t.Errorf("Expected %s to be an upgrade over %s, but it wasn't", test.request, test.existing)
			} else if !test.expectUpgrade && result != nil && result.t.Name == test.request {
				t.Errorf("Expected %s NOT to be an upgrade over %s, but it was", test.request, test.existing)
			}
		})
	}
}

func TestCheckSource(t *testing.T) {
	tests := []struct {
		name          string
		request       string
		existing      string
		expectUpgrade bool
	}{
		{
			name:          "Better source is upgrade",
			request:       "Movie.2023.1080p.BluRay.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.1080p.WEBRiP.DDP5.1.H.264-GROUP",
			expectUpgrade: true,
		},
		{
			name:          "Worse source is not upgrade",
			request:       "Movie.2023.1080p.HDTV.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP",
			expectUpgrade: false,
		},
		{
			name:          "Same source is not upgrade",
			request:       "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP2",
			expectUpgrade: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestRls := rls.ParseString(test.request)
			existingRls := rls.ParseString(test.existing)

			requestEntry := &Entry{
				t: qbittorrent.Torrent{Name: test.request},
				r: &requestRls,
			}

			existingEntry := &Entry{
				t: qbittorrent.Torrent{Name: test.existing},
				r: &existingRls,
			}

			result := checkSource(requestEntry, existingEntry)

			if test.expectUpgrade && (result == nil || result.t.Name != test.request) {
				t.Errorf("Expected %s to be an upgrade over %s, but it wasn't", test.request, test.existing)
			} else if !test.expectUpgrade && result != nil && result.t.Name == test.request {
				t.Errorf("Expected %s NOT to be an upgrade over %s, but it was", test.request, test.existing)
			}
		})
	}
}

func TestCheckHDR(t *testing.T) {
	tests := []struct {
		name          string
		request       string
		existing      string
		requestHDR    []string
		existingHDR   []string
		expectUpgrade bool
	}{
		{
			name:          "HDR is upgrade over SDR",
			request:       "Movie.2023.2160p.WEB-DL.HDR.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GROUP",
			requestHDR:    []string{"HDR"},
			existingHDR:   []string{"SDR"},
			expectUpgrade: true,
		},
		{
			name:          "Dolby Vision is upgrade over HDR",
			request:       "Movie.2023.2160p.WEB-DL.DV.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.2160p.WEB-DL.HDR.DDP5.1.H.264-GROUP",
			requestHDR:    []string{"DV"},
			existingHDR:   []string{"HDR"},
			expectUpgrade: true,
		},
		{
			name:          "HDR10+ is upgrade over HDR",
			request:       "Movie.2023.2160p.WEB-DL.HDR10+.DDP5.1.H.264-GROUP",
			existing:      "Movie.2023.2160p.WEB-DL.HDR.DDP5.1.H.264-GROUP",
			requestHDR:    []string{"HDR10+"},
			existingHDR:   []string{"HDR"},
			expectUpgrade: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestRls := rls.ParseString(test.request)
			existingRls := rls.ParseString(test.existing)

			// Set HDR values manually
			requestRls.HDR = test.requestHDR
			existingRls.HDR = test.existingHDR

			requestEntry := &Entry{
				t: qbittorrent.Torrent{Name: test.request},
				r: &requestRls,
			}

			existingEntry := &Entry{
				t: qbittorrent.Torrent{Name: test.existing},
				r: &existingRls,
			}

			result := checkHDR(requestEntry, existingEntry)

			if test.expectUpgrade && (result == nil || result.t.Name != test.request) {
				t.Errorf("Expected %s to be an upgrade over %s, but it wasn't", test.request, test.existing)
			} else if !test.expectUpgrade && result != nil && result.t.Name == test.request {
				t.Errorf("Expected %s NOT to be an upgrade over %s, but it was", test.request, test.existing)
			}
		})
	}
}

func TestCheckAudio(t *testing.T) {
	tests := []struct {
		name          string
		request       string
		existing      string
		requestAudio  []string
		existingAudio []string
		expectUpgrade bool
	}{
		{
			name:          "TrueHD is upgrade over DTS",
			request:       "Movie.2023.1080p.BluRay.TrueHD.Atmos-GROUP",
			existing:      "Movie.2023.1080p.BluRay.DTS-GROUP",
			requestAudio:  []string{"TrueHD", "Atmos"},
			existingAudio: []string{"DTS"},
			expectUpgrade: true,
		},
		{
			name:          "DTS-HD.MA is upgrade over DDP",
			request:       "Movie.2023.1080p.BluRay.DTS-HD.MA-GROUP",
			existing:      "Movie.2023.1080p.BluRay.DDP5.1-GROUP",
			requestAudio:  []string{"DTS-HD.MA"},
			existingAudio: []string{"DDP"},
			expectUpgrade: true,
		},
		{
			name:          "Same audio is not upgrade",
			request:       "Movie.2023.1080p.BluRay.DTS-GROUP1",
			existing:      "Movie.2023.1080p.BluRay.DTS-GROUP2",
			requestAudio:  []string{"DTS"},
			existingAudio: []string{"DTS"},
			expectUpgrade: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestRls := rls.ParseString(test.request)
			existingRls := rls.ParseString(test.existing)

			// Set Audio values manually
			requestRls.Audio = test.requestAudio
			existingRls.Audio = test.existingAudio

			requestEntry := &Entry{
				t: qbittorrent.Torrent{Name: test.request},
				r: &requestRls,
			}

			existingEntry := &Entry{
				t: qbittorrent.Torrent{Name: test.existing},
				r: &existingRls,
			}

			result := checkAudio(requestEntry, existingEntry)

			if test.expectUpgrade && (result == nil || result.t.Name != test.request) {
				t.Errorf("Expected %s to be an upgrade over %s, but it wasn't", test.request, test.existing)
			} else if !test.expectUpgrade && result != nil && result.t.Name == test.request {
				t.Errorf("Expected %s NOT to be an upgrade over %s, but it was", test.request, test.existing)
			}
		})
	}
}
