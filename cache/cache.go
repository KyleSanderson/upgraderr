package cache

import (
	"sync"
	"time"

	"github.com/autobrr/autobrr/pkg/timecache"
	"github.com/autobrr/autobrr/pkg/ttlcache"
	"github.com/autobrr/go-qbittorrent"
	"github.com/moistari/rls"
	"github.com/titlerr/upgraderr/models"
)

var (
	// Global time cache for consistent time operations
	GlobalTime = timecache.New(timecache.Options{})

	// ClientMap caches qBittorrent clients
	ClientMap = ttlcache.New[qbittorrent.Config, *qbittorrent.Client](
		ttlcache.Options[qbittorrent.Config, *qbittorrent.Client]{}.
			SetDefaultTTL(time.Minute * 5).
			SetTimerResolution(time.Minute * 1))

	// TorrentMap caches torrents per client
	TorrentMap = ttlcache.New[qbittorrent.Config, *models.TimeEntry](
		ttlcache.Options[qbittorrent.Config, *models.TimeEntry]{}.
			SetDefaultTTL(time.Minute * 5).
			SetTimerResolution(time.Second * 1).
			DisableUpdateTime(true))

	// TitleMap caches parsed release information
	TitleMap = ttlcache.New[string, *rls.Release](
		ttlcache.Options[string, *rls.Release]{}.
			SetDefaultTTL(time.Minute * 15).
			SetTimerResolution(time.Minute * 5))

	// FormattedMap caches formatted titles
	FormattedMap = ttlcache.New[string, string](
		ttlcache.Options[string, string]{}.
			SetDefaultTTL(time.Minute * 15).
			SetTimerResolution(time.Minute * 5))
)

// GetOrUpdate safely handles a read-then-update pattern with proper locking
func GetOrUpdate(mutex func() *sync.RWMutex, read func() bool, update func() error) error {
	m := mutex()
	m.RLock()
	if read() {
		m.RUnlock()
		return nil
	}
	m.RUnlock()

	m = mutex()
	m.Lock()
	defer m.Unlock()
	return update()
}
