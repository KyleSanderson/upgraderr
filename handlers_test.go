package main

import (
	"testing"
	"time"

	"github.com/autobrr/autobrr/pkg/ttlcache"
	"github.com/autobrr/go-qbittorrent"
	"github.com/moistari/rls"
)

// Mock client for testing
type mockClient struct {
	torrents map[string][]qbittorrent.Torrent
}

func (m *mockClient) GetTorrents(opts qbittorrent.TorrentFilterOptions) ([]qbittorrent.Torrent, error) {
	if len(opts.Hashes) > 0 {
		for _, hash := range opts.Hashes {
			for _, torrentSet := range m.torrents {
				for _, torrent := range torrentSet {
					if torrent.Hash == hash {
						return []qbittorrent.Torrent{torrent}, nil
					}
				}
			}
		}
		return []qbittorrent.Torrent{}, nil
	}

	result := []qbittorrent.Torrent{}
	for _, torrentSet := range m.torrents {
		result = append(result, torrentSet...)
	}
	return result, nil
}

func (m *mockClient) Login() error {
	return nil
}

// Setup mock client and data for tests
func setupMockClient() *mockClient {
	return &mockClient{
		torrents: map[string][]qbittorrent.Torrent{
			CacheFormatted("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP"): {
				{
					Name:     "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP",
					Hash:     "hash1",
					Progress: 1.0,
					State:    qbittorrent.TorrentStateUploading,
				},
			},
			CacheFormatted("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GROUP"): {
				{
					Name:     "Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GROUP",
					Hash:     "hash2",
					Progress: 1.0,
					State:    qbittorrent.TorrentStateUploading,
				},
			},
			CacheFormatted("TV.Show.S01E01.1080p.WEB-DL.DDP5.1.H.264-GROUP"): {
				{
					Name:     "TV.Show.S01E01.1080p.WEB-DL.DDP5.1.H.264-GROUP",
					Hash:     "hash3",
					Progress: 1.0,
					State:    qbittorrent.TorrentStateUploading,
				},
			},
		},
	}
}

// Helper function to create a mock timeentry
func createMockTimeEntry(client *mockClient) *timeentry {
	te := &timeentry{
		e: make(map[string][]qbittorrent.Torrent),
	}

	for k, v := range client.torrents {
		te.e[k] = v
	}

	return te
}

// Test the CacheTitle function
func TestCacheTitleFunction(t *testing.T) {
	// Clear the cache before testing
	titlemap = ttlcache.New[string, *rls.Release](
		ttlcache.Options[string, *rls.Release]{}.
			SetDefaultTTL(time.Minute * 15).
			SetTimerResolution(time.Minute * 5))

	title := "Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP"

	// First call should parse and cache
	result1 := CacheTitle(title)

	if result1 == nil {
		t.Fatalf("CacheTitle returned nil for %s", title)
	}

	if result1.Title != "Movie" {
		t.Errorf("CacheTitle parsed incorrect title: got %s want %s", result1.Title, "Movie")
	}

	// Second call should return from cache
	result2 := CacheTitle(title)

	// Both results should be the same object
	if result1 != result2 {
		t.Errorf("CacheTitle did not return cached value")
	}
}
