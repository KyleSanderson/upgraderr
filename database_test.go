package main

import (
	"os"
	"sync"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestInitDatabase(t *testing.T) {
	// Save the original db
	origDB := db

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "upgraderr-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Set up a function to initialize the database with a specific path
	initTestDB := func(path string) error {
		var err error
		db, err = bolt.Open(path, 0600, nil)
		if err != nil {
			return err
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

	// Test with a valid path
	if err := initTestDB(tmpPath); err != nil {
		t.Errorf("Failed to initialize database with valid path: %v", err)
	}

	// Check if buckets were created
	err = db.View(func(tx *bolt.Tx) error {
		buckets := []string{"enclosures", "titles", "torrents", "queries"}
		for _, bucket := range buckets {
			b := tx.Bucket([]byte(bucket))
			if b == nil {
				t.Errorf("Bucket %s was not created", bucket)
			}
		}
		return nil
	})

	if err != nil {
		t.Errorf("Error checking buckets: %v", err)
	}

	// Close the test database
	if db != nil {
		db.Close()
	}

	// Restore the original db
	db = origDB
}

func TestGetOrUpdate(t *testing.T) {
	var data string
	var mutex sync.RWMutex

	// Test when data is already set
	data = "test data"
	err := GetOrUpdate(
		func() *sync.RWMutex { return &mutex },
		func() bool { return len(data) > 0 },
		func() error {
			data = "updated data"
			return nil
		},
	)

	if err != nil {
		t.Errorf("GetOrUpdate returned error: %v", err)
	}

	if data != "test data" {
		t.Errorf("GetOrUpdate updated data when it shouldn't have: got %s, want %s", data, "test data")
	}

	// Test when data needs to be updated
	data = ""
	err = GetOrUpdate(
		func() *sync.RWMutex { return &mutex },
		func() bool { return len(data) > 0 },
		func() error {
			data = "updated data"
			return nil
		},
	)

	if err != nil {
		t.Errorf("GetOrUpdate returned error: %v", err)
	}

	if data != "updated data" {
		t.Errorf("GetOrUpdate didn't update data when it should have: got %s, want %s", data, "updated data")
	}
}
