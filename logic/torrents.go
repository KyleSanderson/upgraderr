package logic

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/autobrr/autobrr/pkg/ttlcache"
	"github.com/autobrr/go-qbittorrent"
	"github.com/titlerr/upgraderr/cache"
	"github.com/titlerr/upgraderr/models"
)

// GetClient initializes or retrieves a cached qBittorrent client
func GetClient(req *models.UpgradeRequest) error {
	s := qbittorrent.Config{
		Host:     req.Host,
		Username: req.User,
		Password: req.Password,
	}

	c, ok := cache.ClientMap.Get(s)
	if !ok {
		c = qbittorrent.NewClient(s)

		if err := c.Login(); err != nil {
			return err
		}

		cache.ClientMap.Set(s, c, ttlcache.DefaultTTL)
	}

	req.Client = c
	return nil
}

// GetAllTorrents retrieves all torrents for a client with caching
func GetAllTorrents(req *models.UpgradeRequest) (*models.TimeEntry, error) {
	set := qbittorrent.Config{
		Host:     req.Host,
		Username: req.User,
		Password: req.Password,
	}

	getOrInitialize := func() ttlcache.Item[*models.TimeEntry] {
		it, ok := cache.TorrentMap.GetOrSetItem(set, &models.TimeEntry{}, ttlcache.DefaultTTL)
		if !ok {
			return cache.TorrentMap.SetItem(set, &models.TimeEntry{}, ttlcache.DefaultTTL)
		}

		if req.CacheBypass == 1 {
			val := it.GetValue()
			val.Mu.RLock()
			defer val.Mu.RUnlock()
			if val.Data != nil {
				return cache.TorrentMap.SetItem(set, &models.TimeEntry{}, ttlcache.DefaultTTL)
			}
		}

		return it
	}

	var te ttlcache.Item[*models.TimeEntry]
	var val *models.TimeEntry

	err := cache.GetOrUpdate(func() *sync.RWMutex {
		te = getOrInitialize()
		val = te.GetValue()
		return &val.Mu
	}, func() bool {
		return val.Data != nil
	}, func() error {
		if val.Data != nil {
			return nil
		}

		torrents, err := req.Client.GetTorrents(qbittorrent.TorrentFilterOptions{})
		if err != nil {
			return err
		}

		val.Data = make(map[string][]qbittorrent.Torrent)

		for _, t := range torrents {
			s := CacheFormatted(t.Name)
			val.Data[s] = append(val.Data[s], t)
		}

		dur := cache.GlobalTime.Now().Sub(te.GetTime().Add(-te.GetDuration()))
		if dur < time.Second*1 {
			dur = time.Second * 1
		}

		cache.TorrentMap.Set(set, val, dur)
		return nil
	})

	return val, err
}

// SubmitTorrent adds a new torrent to the client
func SubmitTorrent(req *models.UpgradeRequest, opts *qbittorrent.TorrentAddOptions) error {
	f, err := os.CreateTemp("", "upgraderr-sub.")
	if err != nil {
		return fmt.Errorf("Unable to tmpfile: %q", err)
	}

	defer f.Close()
	defer os.Remove(f.Name())

	if _, err = f.Write(req.Torrent); err != nil {
		return fmt.Errorf("Unable to write (%q): %q", err, f.Name())
	}

	if err = f.Sync(); err != nil {
		return fmt.Errorf("Unable to sync (%q): %q", err, f.Name())
	}

	return req.Client.AddTorrentFromFile(f.Name(), opts.Prepare())
}

// GetTorrent retrieves a specific torrent by hash or name
func GetTorrent(req *models.UpgradeRequest) (qbittorrent.Torrent, error) {
	if len(req.Hash) != 0 {
		torrents, err := req.Client.GetTorrents(qbittorrent.TorrentFilterOptions{Hashes: []string{req.Hash}})
		if err != nil {
			return qbittorrent.Torrent{}, err
		} else if len(torrents) == 0 {
			return qbittorrent.Torrent{}, fmt.Errorf("Unable to find Hash: %q", req.Hash)
		}

		for _, t := range torrents {
			if t.Hash == req.Hash {
				return t, nil
			}
		}

		return qbittorrent.Torrent{}, fmt.Errorf("Unable to find Hash after lookup: %q", req.Hash)
	}

	t, err := req.Client.GetTorrents(qbittorrent.TorrentFilterOptions{})
	if err != nil {
		return qbittorrent.Torrent{}, err
	}

	for _, v := range t {
		switch v.State {
		case qbittorrent.TorrentStateError, qbittorrent.TorrentStateMissingFiles,
			qbittorrent.TorrentStatePausedDl, qbittorrent.TorrentStatePausedUp, // <qb5.0
			qbittorrent.TorrentStateStoppedDl, qbittorrent.TorrentStateStoppedUp, // +qb5.0 webapi 2.11
			qbittorrent.TorrentStateCheckingDl, qbittorrent.TorrentStateCheckingUp, qbittorrent.TorrentStateCheckingResumeData:
			if req.Name == v.Name {
				req.Hash = v.Hash
				return v, nil
			}
		default:
			if req.Name == v.Name {
				fmt.Printf("Found non-conforming: %q | %q\n", v.Name, v.State)
			}
		}
	}

	return qbittorrent.Torrent{}, fmt.Errorf("Unable to find %q", req.Name)
}
