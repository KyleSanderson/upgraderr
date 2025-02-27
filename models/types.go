package models

import (
	"encoding/json"
	"sync"

	"github.com/autobrr/go-qbittorrent"
	"github.com/moistari/rls"
)

// Entry represents a torrent with its release information
type Entry struct {
	Torrent qbittorrent.Torrent
	Release *rls.Release
}

// UpgradeRequest holds the data needed for an upgrade operation
type UpgradeRequest struct {
	Name        string
	User        string
	Password    string
	Host        string
	Port        uint
	CacheBypass uint
	Hash        string
	Torrent     json.RawMessage
	Client      *qbittorrent.Client
}

// TimeEntry holds torrent data with thread-safe access
type TimeEntry struct {
	Data map[string][]qbittorrent.Torrent
	Mu   sync.RWMutex
}

// TorznabRequest represents a request from a Torznab client
type TorznabRequest struct {
	Title       string          `json:"title"`
	TorrentData json.RawMessage `json:"torrentData"`
	ApiKey      string          `json:"apiKey"`
	Host        string          `json:"host"`
	Port        int             `json:"port"`
	Username    string          `json:"username"`
	Password    string          `json:"password"`
	Type        string          `json:"type"` // qBittorrent/Transmission/etc.
}
