package logic

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/autobrr/go-qbittorrent"
	"github.com/avast/retry-go"
	"github.com/moistari/rls"
	"github.com/titlerr/upgraderr/models"
)

// ProcessCrossSeed handles the core logic of cross-seeding
func ProcessCrossSeed(req *models.UpgradeRequest) (string, int, error) {
	// Get all torrents to find matches
	mp, err := GetAllTorrents(req)
	if err != nil {
		return fmt.Sprintf("Unable to get result: %q\n", err), 497, err
	}

	requestrls := models.Entry{Release: CacheTitle(req.Name)}
	v, ok := mp.Data[CacheFormatted(req.Name)]
	if !ok {
		return fmt.Sprintf("Not a cross-submission: %q\n", req.Name), 420, errors.New("not a cross submission")
	}

	// Decode torrent data
	decodeTorrentData(req)

	// Process matching torrents
	for _, childtor := range v {
		child := models.Entry{Torrent: childtor, Release: CacheTitle(childtor.Name)}
		if rls.Compare(*requestrls.Release, *child.Release) != 0 || child.Torrent.Progress != 1.0 {
			continue
		}

		result, code, err := processSingleCrossSeed(req, child)
		if err == nil {
			return result, code, nil
		}

		// If we get here with a specific error code, return it
		if code >= 400 {
			return result, code, err
		}
	}

	return fmt.Sprintf("Failed to cross: %q\n", req.Name), 414, errors.New("no matching torrents found")
}

// Helper functions
func decodeTorrentData(req *models.UpgradeRequest) {
	if t, err := base64.StdEncoding.DecodeString(strings.Trim(strings.TrimSpace(string(req.Torrent)), `"`)); err == nil {
		req.Torrent = t
	} else {
		t := strings.Trim(strings.TrimSpace(string(req.Torrent)), `\"[`)
		b := make([]byte, 0, len(t)/3)

		for {
			r, valid, z := Atoi(t)
			if !valid {
				break
			}

			b = append(b, byte(r))
			t = z
		}

		if len(b) != 0 {
			req.Torrent = b
		}
	}
}

// processSingleCrossSeed handles a single cross-seeding operation
func processSingleCrossSeed(req *models.UpgradeRequest, child models.Entry) (string, int, error) {
	// Get files to check layout
	m, err := req.Client.GetFilesInformation(child.Torrent.Hash)
	if err != nil {
		fmt.Printf("Failed to get Files %q: %q\n", req.Name, err)
		return "", 0, errors.New("failed to get files information")
	}

	// Check directory layout
	dirLayout := false
	if m != nil && len(*m) > 0 {
		for _, v := range *m {
			dirLayout = strings.HasPrefix(v.Name, child.Torrent.Name)
			break
		}
	}

	// Handle category
	cat := child.Torrent.Category
	if !strings.Contains(cat, ".cross-seed") {
		cats, err := req.Client.GetCategories()
		if err != nil {
			return fmt.Sprintf("Failed to get categories (%q): %q\n", child.Torrent.Name, err), 496, err
		}

		if v, ok := cats[cat]; ok {
			save := v.SavePath
			if len(save) == 0 {
				save = cat
			}

			cat += ".cross-seed"

			if _, ok := cats[cat]; !ok {
				if err := req.Client.CreateCategory(cat, save); err != nil {
					return fmt.Sprintf("Failed to create new category (%q): %q\n", cat, err), 495, err
				}
			}
		}
	}

	// Set up options
	opts := &qbittorrent.TorrentAddOptions{
		SkipHashCheck: true,
		Category:      cat,
		Tags:          "upgraderr",
		Paused:        true,
	}

	if dirLayout {
		opts.ContentLayout = qbittorrent.ContentLayoutSubfolderCreate
	} else {
		opts.ContentLayout = qbittorrent.ContentLayoutSubfolderNone
	}

	// Add the torrent
	optionsMap := map[string]string{
		"skip_checking": fmt.Sprintf("%t", opts.SkipHashCheck),
		"category":      opts.Category,
		"tags":          opts.Tags,
		"paused":        fmt.Sprintf("%t", opts.Paused),
	}
	if opts.SavePath != "" {
		optionsMap["savepath"] = opts.SavePath
	}

	if err = retry.Do(func() error {
		return req.Client.AddTorrentFromMemory(req.Torrent, optionsMap)
	},
		retry.OnRetry(func(n uint, err error) { fmt.Printf("%q: submission attempt %d - %v\n", err, n, req.Name) }),
		retry.Delay(time.Second*1),
		retry.Attempts(7),
		retry.MaxJitter(time.Second*1)); err != nil {
		return fmt.Sprintf("Failed to cross: %q\n", req.Name), 490, err
	}

	// Monitor torrent state
	err = retry.Do(func() error {
		t, err := GetTorrent(req)
		if err != nil {
			return errors.New("423 Unable to find torrent")
		}

		switch t.State {
		case qbittorrent.TorrentStateStalledUp, qbittorrent.TorrentStateUploading:
			req.Client.ReAnnounceTorrents([]string{req.Hash})
			return nil // Success case
		case qbittorrent.TorrentStateStalledDl, qbittorrent.TorrentStateDownloading:
			req.Client.ReAnnounceTorrents([]string{req.Hash})
			fmt.Printf("Considering successful. Downloading: %q", req.Name)
			return nil // Success case
		case qbittorrent.TorrentStateMissingFiles:
			req.Client.Recheck([]string{req.Hash})
			return errors.New("469 Rechecking")
		case qbittorrent.TorrentStatePausedUp, qbittorrent.TorrentStateStoppedUp:
			if err := req.Client.Resume([]string{req.Hash}); err != nil {
				return errors.New("468 Unable to resume torrent")
			}
			return errors.New("467 PausedUp")
		case qbittorrent.TorrentStatePausedDl, qbittorrent.TorrentStateStoppedDl:
			if t.Progress < 0.8 {
				return retry.Unrecoverable(errors.New("466 Name matched, data did not on cross"))
			}

			files, err := req.Client.GetFilesInformation(req.Hash)
			if err != nil {
				return errors.New("465 Unable to get Files")
			}

			damage := false
			for _, f := range *files {
				if f.Progress == 1.0 {
					continue
				}

				damage = true
				break
			}

			if damage == false {
				if err := req.Client.Resume([]string{req.Hash}); err != nil {
					return errors.New("464 Unable to resume valid cross")
				}

				req.Client.ReAnnounceTorrents([]string{req.Hash})
				return nil // Success case
			}

			if err := req.Client.DeleteTorrents([]string{req.Hash}, false); err != nil {
				return errors.New("463 Unable to delete existing torrent")
			}

			// This is still the old Torrent
			atm := t.AutoManaged
			oldpath := t.SavePath
			opts.SavePath = t.SavePath + "/.tmp"
			optionsMap := map[string]string{
				"skip_checking": fmt.Sprintf("%t", opts.SkipHashCheck),
				"category":      opts.Category,
				"tags":          opts.Tags,
				"paused":        fmt.Sprintf("%t", opts.Paused),
				"savepath":      opts.SavePath,
			}
			if err := req.Client.AddTorrentFromMemory(req.Torrent, optionsMap); err != nil {
				req.Client.DeleteTorrents([]string{req.Hash}, false)
				return errors.New("450 Failed to adv cross")
			}

			for t.State = "check"; strings.Contains(string(t.State), "check"); {
				t, err = GetTorrent(req)
				if err != nil {
					t.State = "check"
				}
			}

			for _, f := range *files {
				if f.Progress == 1.0 {
					continue
				}

				for _, pf := range *m {
					if pf.Name != f.Name {
						continue
					}

					var np string
					if idx := strings.LastIndex(f.Name, "/"); idx != -1 {
						np = f.Name[:idx]
						if len(f.Name) > idx+1 {
							np += "/" + req.Hash + "_" + f.Name[idx+1:]
						}
					} else {
						np = req.Hash + "/" + f.Name
					}

					req.Client.RenameFile(req.Hash, f.Name, np) // if it fails, so be it
				}
			}

			if err := req.Client.SetLocation([]string{req.Hash}, oldpath); err != nil {
				return errors.New("435 Failed to change save location")
			}

			if t.AutoManaged != atm {
				if err := req.Client.SetAutoManagement([]string{req.Hash}, atm); err != nil {
					return errors.New("433 Failed to ATM")
				}
			}

			if err := req.Client.Recheck([]string{req.Hash}); err != nil {
				return errors.New("431 Failed to Recheck")
			}

			if err := req.Client.Resume([]string{req.Hash}); err != nil {
				return errors.New("429 Failed to Resume")
			}

			req.Client.ReAnnounceTorrents([]string{req.Hash})
			return nil // Success case
		case qbittorrent.TorrentStateCheckingUp, qbittorrent.TorrentStateCheckingDl, qbittorrent.TorrentStateCheckingResumeData:
			return fmt.Errorf("412 Still Checking: %q", t.State)
		}

		return fmt.Errorf("410 End of loop. Continuing: %q", t.State)
	},
		retry.OnRetry(func(n uint, err error) { fmt.Printf("%q: attempt %d - %v\n", err, n, req.Name) }),
		retry.Delay(time.Second*1),
		retry.Attempts(47),
		retry.MaxJitter(time.Second*1),
	)

	if err == nil {
		return fmt.Sprintf("Crossed Successfully: %q", req.Name), 200, nil
	}

	req.Client.DeleteTorrents([]string{req.Hash}, false)
	if ret, _, _ := Atoi(fmt.Sprintf("%s", err)); ret >= 400 {
		return fmt.Sprintf("Failed to cross %q %q", req.Name, err), ret, err
	} else {
		return fmt.Sprintf("Failed to cross generic %q %q", req.Name, err), 415, err
	}
}
