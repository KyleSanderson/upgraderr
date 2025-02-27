package database

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/autobrr/go-qbittorrent"
	"github.com/kylesanderson/go-jackett"
	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB
var dbMutex sync.RWMutex

// InitDatabase initializes the database
func InitDatabase() error {
	var err error
	db, err = bolt.Open("/config/upgraderr.db", 0600, nil)
	if err != nil {
		fmt.Printf("WARNING: Unable to open Torznab database on /config. %q\n", err)
		db, err = bolt.Open("upgraderr.db", 0600, nil)
		if err != nil {
			db, err = bolt.Open("/tmp/upgraderr.db", 0600, nil)
			if err != nil {
				fmt.Printf("WARNING: Unable to open Torznab database /tmp. %q\n", err)
				return err
			}
		}
	}

	if db == nil {
		return fmt.Errorf("failed to initialize database")
	}

	return db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("enclosures")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("titles")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("torrents")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("queries")); err != nil {
			return err
		}

		return nil
	})
}

// GetDB returns the database instance
func GetDB() *bolt.DB {
	dbMutex.RLock()
	defer dbMutex.RUnlock()
	return db
}

// CreateIndexerBuckets creates buckets for an indexer
func CreateIndexerBuckets(db *bolt.DB, indexerID string) error {
	return db.Update(func(tx *bolt.Tx) error {
		for _, bucketName := range []string{"enclosures", "titles", "torrents", "queries"} {
			bucket := tx.Bucket([]byte(bucketName))
			if bucket == nil {
				return fmt.Errorf("bucket %s not found", bucketName)
			}

			if _, err := bucket.CreateBucketIfNotExists([]byte(indexerID)); err != nil {
				return err
			}
		}

		return nil
	})
}

// IsQueryCached checks if a query is cached within the time window
func IsQueryCached(db *bolt.DB, indexerID, query string, currentTime int64) bool {
	var cached bool

	_ = db.View(func(tx *bolt.Tx) error {
		pb := tx.Bucket([]byte("queries"))
		if pb == nil {
			return nil
		}

		b := pb.Bucket([]byte(indexerID))
		if b == nil {
			return nil
		}

		stamp := b.Get([]byte(query))
		if stamp == nil {
			return nil
		}

		if currentTime-720 < int64(binary.LittleEndian.Uint64(stamp)) {
			cached = true
			return fmt.Errorf("cache found")
		}

		return nil
	})

	return cached
}

// StoreTorznabResults stores the results from Torznab
func StoreTorznabResults(db *bolt.DB, indexerID string, res jackett.Rss, query string, currentTime int64) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Store titles and enclosures
		{
			tb := tx.Bucket([]byte("titles"))
			if tb == nil {
				return fmt.Errorf("titles: Failed to find bucket")
			}

			b := tb.Bucket([]byte(indexerID))
			if b == nil {
				return fmt.Errorf("%q: Failed to find title bucket", indexerID)
			}

			eb := tx.Bucket([]byte("enclosures"))
			if eb == nil {
				return fmt.Errorf("enclosures: Failed to find bucket")
			}

			c := eb.Bucket([]byte(indexerID))
			if c == nil {
				return fmt.Errorf("%q: Failed to find enclosure bucket", indexerID)
			}

			for _, ch := range res.Channel.Item {
				if err := b.Put([]byte(ch.Title), []byte(ch.Guid)); err != nil {
					return err
				}

				if err := c.Put([]byte(ch.Guid), []byte(ch.Enclosure.URL)); err != nil {
					return err
				}
			}
		}

		// Store query timestamp
		{
			pb := tx.Bucket([]byte("queries"))
			if pb == nil {
				return fmt.Errorf("queries: Failed to find bucket")
			}

			b := pb.Bucket([]byte(indexerID))
			if b == nil {
				return fmt.Errorf("%q: Failed to find queries bucket", indexerID)
			}

			if err := b.Put([]byte(query), binary.LittleEndian.AppendUint64(nil, uint64(currentTime))); err != nil {
				return err
			}
		}

		return nil
	})
}

// ProcessTorrentEnclosures processes enclosures and updates torrents
func ProcessTorrentEnclosures(db *bolt.DB, mp interface{}, client *qbittorrent.Client) error {
	// This would contain the logic to process enclosures and update torrents
	// Simplified for this example as it depends on the specific mp structure
	return nil
}
