/*
Copyright (C) 2022  Kyle Sanderson

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; specifically version 2
of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
*/

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/antonmedv/expr"
	"github.com/autobrr/go-qbittorrent"
	"github.com/avast/retry-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/moistari/rls"
	"github.com/pkg/errors"
)

type Entry struct {
	t qbittorrent.Torrent
	r rls.Release
}

type upgradereq struct {
	Name string

	User     string
	Password string
	Host     string
	Port     uint

	Hash    string
	Torrent json.RawMessage
	Client  *qbittorrent.Client
}

type timeentry struct {
	e   map[string][]Entry
	d   map[string]rls.Release
	t   time.Time
	err error
	sync.Mutex
}

var clientmap sync.Map
var torrentmap sync.Map

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("k8s"))
	})

	r.Post("/api/upgrade", handleUpgrade)
	r.Post("/api/cross", handleCross)
	r.Post("/api/clean", handleClean)
	r.Post("/api/unregistered", handleUnregistered)
	r.Post("/api/expression", handleExpression)
	r.Post("/api/autobrr/filterupdate", handleAutobrrFilterUpdate)
	http.ListenAndServe(":6940", r) /* immutable. this is b's favourite positive 4digit number not starting with a 0. */
}

func getClient(req *upgradereq) error {
	s := qbittorrent.Config{
		Host:     req.Host,
		Username: req.User,
		Password: req.Password,
	}

	c, ok := clientmap.Load(s)
	if !ok {
		c = qbittorrent.NewClient(qbittorrent.Config{
			Host:     req.Host,
			Username: req.User,
			Password: req.Password,
		})

		if err := c.(*qbittorrent.Client).Login(); err != nil {
			return err
		}

		clientmap.Store(s, c)
	}

	req.Client = c.(*qbittorrent.Client)
	return nil
}

func heartbeat(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Alive", 200)
}

func (c *upgradereq) getAllTorrents() timeentry {
	set := qbittorrent.Config{
		Host:     c.Host,
		Username: c.User,
		Password: c.Password,
	}

	f := func() *timeentry {
		te, ok := torrentmap.Load(set)
		if ok {
			return te.(*timeentry)
		}

		res := &timeentry{d: make(map[string]rls.Release)}
		torrentmap.Store(set, res)
		return res
	}

	res := f()
	cur := time.Now()
	if res.t.After(cur) {
		return *res
	}

	res.Lock()
	defer res.Unlock()

	res = f()
	if res.t.After(cur) {
		return *res
	}

	torrents, err := c.Client.GetTorrents(qbittorrent.TorrentFilterOptions{})
	if err != nil {
		return timeentry{err: err}
	}

	nt := time.Now()
	res = &timeentry{e: make(map[string][]Entry), t: nt.Add(nt.Sub(cur)), d: res.d}

	for _, t := range torrents {
		r, ok := res.d[t.Name]
		if !ok {
			r = rls.ParseString(t.Name)
			res.d[t.Name] = r
		}

		s := getFormattedTitle(r)
		res.e[s] = append(res.e[s], Entry{t: t, r: r})
	}

	torrentmap.Store(set, res)
	return *res
}

func (c *upgradereq) getFiles(hash string) (*qbittorrent.TorrentFiles, error) {
	return c.Client.GetFilesInformation(hash)
}

func (c *upgradereq) getCategories() (map[string]qbittorrent.Category, error) {
	return c.Client.GetCategories()
}

func (c *upgradereq) createCategory(cat, savePath string) error {
	return c.Client.CreateCategory(cat, savePath)
}

func (c *upgradereq) recheckTorrent() error {
	return c.Client.Recheck([]string{c.Hash})
}

func (c *upgradereq) setTorrentManagement(enable bool) error {
	return c.Client.SetAutoManagement([]string{c.Hash}, enable)
}

func (c *upgradereq) resumeTorrent() error {
	return c.Client.Resume([]string{c.Hash})
}

func (c *upgradereq) pauseTorrent() error {
	return c.Client.Pause([]string{c.Hash})
}

func (c *upgradereq) setLocationTorrent(location string) error {
	return c.Client.SetLocation([]string{c.Hash}, location)
}

func (c *upgradereq) deleteTorrent() error {
	return c.Client.DeleteTorrents([]string{c.Hash}, false)
}

func (c *upgradereq) renameFile(hash, oldPath, newPath string) error {
	return c.Client.RenameFile(hash, oldPath, newPath)
}

func (c *upgradereq) getTrackers() ([]qbittorrent.TorrentTracker, error) {
	return c.Client.GetTorrentTrackers(c.Hash)
}

func (c *upgradereq) announceTrackers() error {
	return c.Client.ReAnnounceTorrents([]string{c.Hash})
}

func (c *upgradereq) submitTorrent(opts *qbittorrent.TorrentAddOptions) error {
	f, err := os.CreateTemp("", "upgraderr-sub.")
	if err != nil {
		return fmt.Errorf("Unable to tmpfile: %q", err)
	}

	defer f.Close()
	defer os.Remove(f.Name())

	if _, err = f.Write(c.Torrent); err != nil {
		return fmt.Errorf("Unable to write (%q): %q", err, f.Name())
	}

	if err = f.Sync(); err != nil {
		return fmt.Errorf("Unable to sync (%q): %q", err, f.Name())
	}

	return c.Client.AddTorrentFromFile(f.Name(), opts.Prepare())
}

func (c *upgradereq) getTorrent() (qbittorrent.Torrent, error) {
	if len(c.Hash) != 0 {
		torrents, err := c.Client.GetTorrents(qbittorrent.TorrentFilterOptions{Hashes: []string{c.Hash}})
		if err != nil {
			return qbittorrent.Torrent{}, err
		} else if len(torrents) == 0 {
			return qbittorrent.Torrent{}, fmt.Errorf("Unable to find Hash: %q", c.Hash)
		}

		for _, t := range torrents {
			if t.Hash == c.Hash {
				return t, nil
			}
		}

		return qbittorrent.Torrent{}, fmt.Errorf("Unable to find Hash after lookup: %q", c.Hash)
	}

	t, err := c.Client.GetTorrents(qbittorrent.TorrentFilterOptions{})
	if err != nil {
		return qbittorrent.Torrent{}, err
	}

	for _, v := range t {
		switch v.State {
		case qbittorrent.TorrentStateError, qbittorrent.TorrentStateMissingFiles,
			qbittorrent.TorrentStatePausedDl, qbittorrent.TorrentStatePausedUp,
			qbittorrent.TorrentStateCheckingDl, qbittorrent.TorrentStateCheckingUp, qbittorrent.TorrentStateCheckingResumeData:
			if c.Name == v.Name {
				c.Hash = v.Hash
				return v, nil
			}
		default:
			if c.Name == v.Name {
				fmt.Printf("Found non-conforming: %q | %q\n", v.Name, v.State)
			}
		}
	}

	return qbittorrent.Torrent{}, fmt.Errorf("Unable to find %q", c.Name)
}

func handleUpgrade(w http.ResponseWriter, r *http.Request) {
	var req upgradereq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if len(req.Name) == 0 {
		http.Error(w, fmt.Sprintf("No title passed.\n"), 469)
		return
	}

	if err := getClient(&req); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 471)
		return
	}

	mp := req.getAllTorrents()
	if mp.err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", mp.err), 468)
		return
	}

	requestrls := Entry{r: rls.ParseString(req.Name)}
	if v, ok := mp.e[getFormattedTitle(requestrls.r)]; ok {
		code := 0
		var parent Entry
		for _, child := range v {
			if rls.Compare(requestrls.r, child.r) == 0 {
				if child.t.Progress < parent.t.Progress {
					code = 240 + int(parent.t.Progress*10.0)
					continue
				}

				parent = child
				code = 240 + int(child.t.Progress*10.0)
				if code >= 250 {
					code = 250
					/* wtf. API breakage... but assume it's ok */
					break
				}

				continue
			}

			if res := checkResolution(&requestrls, &child); res != nil && res.t != requestrls.t {
				if src := checkSource(&requestrls, &child); src == nil || src.t != requestrls.t {
					parent = *res
					code = 201
					break
				}
			}

			if res := checkHDR(&requestrls, &child); res != nil && res.t != requestrls.t {
				parent = *res
				code = 202
				break
			}

			if res := checkChannels(&requestrls, &child); res != nil && res.t != requestrls.t {
				parent = *res
				code = 203
				break
			}

			if res := checkSource(&requestrls, &child); res != nil && res.t != requestrls.t {
				parent = *res
				code = 204
				break
			}

			if res := checkAudio(&requestrls, &child); res != nil && res.t != requestrls.t {
				parent = *res
				code = 205
				break
			}

			if res := checkExtension(&requestrls, &child); res != nil && res.t != requestrls.t {
				parent = *res
				code = 206
				break
			}

			if res := checkLanguage(&requestrls, &child); res != nil && res.t != requestrls.t {
				parent = *res
				code = 207
				break
			}

			if res := checkReplacement(&requestrls, &child); res != nil && res.t != requestrls.t {
				parent = *res
				code = 208
				break
			}
		}

		if code >= 240 && code <= 250 {
			http.Error(w, fmt.Sprintf("Cross submission: %q\n", req.Name), code)
		} else if code > 200 && code < 240 {
			http.Error(w, fmt.Sprintf("Not an upgrade submission: %q => %q\n", req.Name, parent.t.Name), code)
		} else {
			http.Error(w, fmt.Sprintf("Upgrade submission: %q\n", req.Name), 200)
		}
	} else {
		http.Error(w, fmt.Sprintf("Unique submission: %q\n", req.Name), 200)
	}
}

func handleClean(w http.ResponseWriter, r *http.Request) {
	var req upgradereq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if err := getClient(&req); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 471)
		return
	}

	mp := req.getAllTorrents()
	if mp.err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", mp.err), 468)
		return
	}

	t := time.Now().Unix()
	hashes := make([]string, 0)
	for _, v := range mp.e {
		var parent Entry
		parentMap := make(map[string]int)
		for _, child := range v {
			if len(parent.t.Hash) == 0 {
				parent = child
			}

			if rls.Compare(parent.r, child.r) == 0 {
				parentMap[child.t.Name]++
				continue
			}

			if res := checkResolution(&parent, &child); res != nil {
				src := checkSource(&parent, &child)
				if src == nil {
					parentMap = make(map[string]int)
					parent = *res
					continue
				} else if src.t.Hash == res.t.Hash {
					parentMap = make(map[string]int)
					parent = *src
					continue
				}
			}

			for _, f := range []func(*Entry, *Entry) *Entry{checkHDR, checkChannels, checkSource, checkAudio, checkExtension, checkLanguage, checkReplacement} {
				if res := f(&parent, &child); res != nil && res.t.Hash != parent.t.Hash {
					parent = *res
					parentMap = make(map[string]int)
					break
				}
			}

			// fmt.Printf("Made it to Loop: %q|%q\n", parent.t.Name, child.t.Name)
		}

		if len(parent.t.Hash) == 0 {
			continue
		}

		var parentName string
		if len(parentMap) == 0 {
			parentName = parent.t.Name
		} else {
			parentNumber := 0
			for k, i := range parentMap {
				if i > parentNumber {
					parentNumber = i
					parentName = k
				}
			}
		}

		fmt.Printf("Parent: %q\n", parentName)
		hashMap := make(map[string]struct{})
		for _, child := range v {
			if child.t.Name == parentName {
				continue
			}

			bContinue := false
			childHashes := make([]string, 0)
			for _, subChild := range v {
				if rls.Compare(subChild.r, child.r) != 0 {
					continue
				}

				if subChild.t.CompletionOn == 0 || t-int64(subChild.t.CompletionOn) < 1209600 {
					bContinue = true
					break
				}

				fmt.Printf("Removing: %q\n", subChild.t.Name)
				childHashes = append(childHashes, subChild.t.Hash)
			}

			if bContinue {
				continue
			}

			for _, h := range childHashes {
				hashMap[h] = struct{}{}
			}
		}

		if len(hashMap) == 0 {
			continue
		}

		for k := range hashMap {
			hashes = append(hashes, k)
		}
	}

	if len(hashes) == 0 {
		http.Error(w, fmt.Sprintf("No eligible torrents to remove."), 205)
		return
	}

	if err := req.Client.DeleteTorrents(hashes, true); err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit %d torrents to remove: %s", len(hashes), err), 420)
		return
	}

	http.Error(w, fmt.Sprintf("Removed %d torrents.", len(hashes)), 200)
}

func handleCross(w http.ResponseWriter, r *http.Request) {
	var req upgradereq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if len(req.Name) == 0 {
		http.Error(w, fmt.Sprintf("No title passed.\n"), 499)
		return
	}

	if err := getClient(&req); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 498)
		return
	}

	mp := req.getAllTorrents()
	if mp.err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", mp.err), 497)
		return
	}

	requestrls := Entry{r: rls.ParseString(req.Name)}
	v, ok := mp.e[getFormattedTitle(requestrls.r)]
	if !ok {
		http.Error(w, fmt.Sprintf("Not a cross-submission: %q\n", req.Name), 420)
		return
	}

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

	for _, child := range v {
		if rls.Compare(requestrls.r, child.r) != 0 || child.t.Progress != 1.0 {
			continue
		}

		m, err := req.getFiles(child.t.Hash)
		if err != nil {
			fmt.Printf("Failed to get Files %q: %q\n", req.Name, err)
			continue
		}

		dirLayout := false
		for _, v := range *m {
			dirLayout = strings.HasPrefix(v.Name, child.t.Name)
			break
		}

		cat := child.t.Category
		if strings.Contains(cat, ".cross-seed") == false {
			cats, err := req.getCategories()
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get categories (%q): %q\n", child.t.Name, err), 496)
				return
			}

			if v, ok := cats[cat]; ok {
				save := v.SavePath
				if len(save) == 0 {
					save = cat
				}

				cat += ".cross-seed"

				if _, ok := cats[cat]; !ok {
					if err := req.createCategory(cat, save); err != nil {
						http.Error(w, fmt.Sprintf("Failed to create new category (%q): %q\n", cat, err), 495)
						return
					}
				}
			}
		}

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

		if err = retry.Do(func() error {
			return req.submitTorrent(opts)
		},
			retry.OnRetry(func(n uint, err error) { fmt.Printf("%q: submission attempt %d - %v\n", err, n, req.Name) }),
			retry.Delay(time.Second*1),
			retry.Attempts(7),
			retry.MaxJitter(time.Second*1)); err != nil {
			http.Error(w, fmt.Sprintf("Failed to cross: %q\n", req.Name), 490)
			return
		}

		err = retry.Do(func() error {
			t, err := req.getTorrent()
			if err != nil {
				return errors.Wrap(err, "423 Unable to find torrent")
			}

			switch t.State {
			case qbittorrent.TorrentStateStalledUp, qbittorrent.TorrentStateUploading:
				req.announceTrackers()
				return nil /* Nice. */
			case qbittorrent.TorrentStateStalledDl, qbittorrent.TorrentStateDownloading:
				req.announceTrackers()
				fmt.Printf("Considering successful. Downloading: %q", req.Name)
				return nil
			case qbittorrent.TorrentStateMissingFiles:
				req.recheckTorrent()
				return errors.New("469 Rechecking")
			case qbittorrent.TorrentStatePausedUp:
				if err := req.resumeTorrent(); err != nil {
					return errors.Wrap(err, "468 Unable to resume torrent")
				}
				return errors.New("467 PausedUp")
			case qbittorrent.TorrentStatePausedDl:
				if t.Progress < 0.8 {
					return retry.Unrecoverable(errors.New("466 Name matched, data did not on cross"))
				}

				files, err := req.getFiles(req.Hash)
				if err != nil {
					return errors.Wrap(err, "465 Unable to get Files")
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
					if err := req.resumeTorrent(); err != nil {
						return errors.Wrap(err, "464 Unable to resume valid cross")
					}

					req.announceTrackers()
					return nil /* Nice! */
				}

				if err := req.deleteTorrent(); err != nil {
					return errors.Wrap(err, "463 Unable to delete existing torrent")
				}

				/* This is still the old Torrent. */
				atm := t.AutoManaged
				oldpath := t.SavePath
				opts.SavePath = t.SavePath + "/.tmp"
				if err := req.submitTorrent(opts); err != nil {
					req.deleteTorrent()
					return errors.Wrap(err, "450 Failed to adv cross")
				}

				for t.State = "check"; strings.Contains(string(t.State), "check"); t, err = req.getTorrent() {
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

						req.renameFile(req.Hash, f.Name, np) /* if it fails. so be it. */
					}
				}

				if err := req.setLocationTorrent(oldpath); err != nil {
					return errors.Wrap(err, "435 Failed to change save location")
				}

				if t.AutoManaged != atm {
					if err := req.setTorrentManagement(atm); err != nil {
						return errors.Wrap(err, "433 Failed to ATM")
					}
				}

				if err := req.recheckTorrent(); err != nil {
					return errors.Wrap(err, "431 Failed to Recheck")
				}

				if err := req.resumeTorrent(); err != nil {
					return errors.Wrap(err, "429 Failed to Resume")
				}

				req.announceTrackers()
				return nil
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
			http.Error(w, fmt.Sprintf("Crossed Successfully: %q", req.Name), 200)
			return
		}

		req.deleteTorrent()
		if ret, _, _ := Atoi(fmt.Sprintf("%s", err)); ret >= 400 {
			http.Error(w, fmt.Sprintf("Failed to cross %q %q", req.Name, err), ret)
		} else {
			http.Error(w, fmt.Sprintf("Failed to cross generic %q %q", req.Name, err), 415)
		}

		return
	}

	http.Error(w, fmt.Sprintf("Failed to cross: %q\n", req.Name), 414)
}

func handleUnregistered(w http.ResponseWriter, r *http.Request) {
	var req upgradereq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if err := getClient(&req); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 471)
		return
	}

	mp := req.getAllTorrents()
	if mp.err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", mp.err), 468)
		return
	}

	deadTracker := []string{
		"unregistered",
		"not registered",
		"not found",
		"not exist",
		"unknown",
		"uploaded",
		"upgraded",
		"season pack",
		"packs are available",
		"pack is available",
		"internal available",
		"season pack out",
		"dead",
		"dupe",
		"complete season uploaded",
		"problem with",
		"specifically banned",
		"trumped",
		"i'm sorry dave, i can't do that", // weird stuff from racingforme
	}

	count := 0
	for _, set := range mp.e {
		for _, t := range set {
			req.Hash = t.t.Hash
			if len(req.Hash) == 0 {
				continue
			}

			trackers, _ := req.getTrackers()
			alive := false

			for _, tracker := range trackers {
				if tracker.Status == qbittorrent.TrackerStatusDisabled {
					continue
				} else if tracker.Status == qbittorrent.TrackerStatusOK {
					alive = true
				} else if tracker.Status == qbittorrent.TrackerStatusNotWorking {
				} else {
					continue
				}

				for _, z := range deadTracker {
					if strings.Contains(strings.ToLower(tracker.Message), z) {
						count++
						req.deleteTorrent()
						alive = false
						break
					}
				}
			}

			if !alive {
				child := req
				go child.announceTrackers()
			}
		}
	}

	http.Error(w, fmt.Sprintf("Unregistered torrents deleted: %d", count), 200)
}

func getFormattedTitle(r rls.Release) string {
	s := fmt.Sprintf("%s%s%s%04d%02d%02d%02d%03d", rls.MustNormalize(r.Artist), rls.MustNormalize(r.Title), rls.MustNormalize(r.Subtitle), r.Year, r.Month, r.Day, r.Series, r.Episode)
	for _, a := range r.Cut {
		s += rls.MustNormalize(a)
	}

	for _, a := range r.Edition {
		s += rls.MustNormalize(a)
	}

	return s
}

func checkExtension(requestrls, child *Entry) *Entry {
	sm := map[string]int{
		"mkv":  90,
		"mp4":  89,
		"webp": 88,
		"ts":   87,
		"wmv":  86,
		"xvid": 85,
		"divx": 84,
	}

	return compareResults(requestrls, child, func(e rls.Release) int {
		i := sm[e.Ext]

		if i == 0 {
			if len(e.Ext) != 0 {
				fmt.Printf("UNKNOWNEXT: %q\n", e.Ext)
			}

			i = sm["divx"]
		}

		return i
	})
}

func checkLanguage(requestrls, child *Entry) *Entry {
	sm := map[string]int{
		"ENGLiSH":    20,
		"MULTi":      19,
		"FRENCH":     18,
		"SWEDiSH":    17,
		"SWESUB":     16,
		"NORWEGiAN":  15,
		"NORDiCSUBS": 14,
		"DUBBED":     13,
		"DANiSH":     12,
		"HiNDI":      11,
		"NORDiC":     10,
		"GERMAN":     9,
		"SUBBED":     8,
		"CZECH":      7,
		"RUSSiAN":    1,
	}

	return compareResults(requestrls, child, func(e rls.Release) int {
		i := 0
		for _, v := range e.Language {
			if i < sm[v] {
				i = sm[v]
			}
		}

		if i == 0 {
			if len(e.Language) != 0 {
				fmt.Printf("UNKNOWNLANGUAGE: %q\n", e.Language)
			} else {
				i = sm["ENGLiSH"]
			}
		}

		return i
	})
}

func checkReplacement(requestrls, child *Entry) *Entry {
	if rls.MustNormalize(child.r.Group) != rls.MustNormalize(requestrls.r.Group) {
		return nil
	}

	sm := map[string]int{
		"COMPLETE":   1,
		"REMUX":      2,
		"FS":         3,
		"EXTENDED":   4,
		"REMASTERED": 5,
		"PROPER":     6,
		"REPACK":     7,
		"INTERNAL":   8,
	}

	return compareResults(requestrls, child, func(e rls.Release) int {
		i := 0
		for _, v := range e.Other {
			if i < sm[v] {
				i = sm[v]
			}
		}

		if i == 0 && len(e.Other) != 0 {
			fmt.Printf("UNKNOWNOTHER: %q\n", e.Other)
		}

		return i
	})
}

func checkAudio(requestrls, child *Entry) *Entry {
	sm := map[string]int{
		"FLAC":       91,
		"DTS-HD.HRA": 90,
		"DDPA":       89,
		"TrueHD":     88,
		"DTS-HD.MA":  87,
		"DTS-HD.HR":  86,
		"Atmos":      85,
		"DTS-HD":     84,
		"DDP":        83,
		"DTS":        82,
		"DD":         81,
		"OPUS":       80,
		"AAC":        79,
		"DUAL.AUDIO": 70,
	}

	return compareResults(requestrls, child, func(e rls.Release) int {
		i := 0
		for _, v := range e.Audio {
			if i < sm[v] {
				i = sm[v]
			}
		}

		if i == 0 {
			if len(e.Audio) != 0 {
				fmt.Printf("UNKNOWNAUDIO: %q\n", e.Audio)
			}

			i = sm["DUAL.AUDIO"]
		}

		return i
	})
}

func checkSource(requestrls, child *Entry) *Entry {
	if child.r.Source == requestrls.r.Source {
		return nil
	}

	sm := map[string]int{
		"WEB-DL":     90,
		"UHD.BluRay": 89,
		"BluRay":     88,
		"WEB":        87,
		"WEBRiP":     86,
		"BDRiP":      85,
		"HDRiP":      84,
		"HDTV":       83,
		"DVDRiP":     82,
		"HDTC":       81,
		"HDTS":       80,
		"TC":         79,
		"VHSRiP":     78,
		"WORKPRiNT":  77,
		"TS":         76,
		"HDCAM":      75,
		"CAM":        74,
	}

	return compareResults(requestrls, child, func(e rls.Release) int {
		i := sm[e.Source]

		if i == 0 {
			if len(e.Source) != 0 {
				fmt.Printf("UNKNOWNSRC: %q\n", e.Source)
			}

			i = sm["TS"]
		}

		return i
	})
}

func checkChannels(requestrls, child *Entry) *Entry {
	if child.r.Channels == requestrls.r.Channels {
		return nil
	}

	return compareResults(requestrls, child, func(e rls.Release) int {
		i, _ := strconv.ParseFloat(e.Channels, 8)

		if i == 0.0 {
			i = 2.0
		}

		return int(i * 10)
	})
}

func checkHDR(requestrls, child *Entry) *Entry {
	sm := map[string]int{
		"DV":     90,
		"HDR10+": 89,
		"HDR10":  88,
		"HDR+":   87,
		"HDR":    86,
		"HLG":    85,
		"SDR":    84,
	}

	return compareResults(requestrls, child, func(e rls.Release) int {
		i := 0
		for _, v := range e.HDR {
			if i < sm[v] {
				i = sm[v]
			}
		}

		if i == 0 {
			if len(e.HDR) != 0 {
				fmt.Printf("UNKNOWNHDR: %q\n", e.HDR)
			}

			i = sm["SDR"]
		}

		return i
	})
}

func checkResolution(requestrls, child *Entry) *Entry {
	if child.r.Resolution == requestrls.r.Resolution {
		return nil
	}

	return compareResults(requestrls, child, func(e rls.Release) int {
		i, _, _ := Atoi(e.Resolution)
		if i == 0 {
			i = 480
		}

		return i
	})
}

func compareResults(requestrls, child *Entry, f func(rls.Release) int) *Entry {
	requestrlsv := f(requestrls.r)
	childv := f(child.r)

	if childv > requestrlsv {
		return child
	} else if requestrlsv > childv {
		return requestrls
	}

	return nil
}

func Normalize(buf string) string {
	return strings.ToLower(strings.TrimSpace(strings.ToValidUTF8(buf, "")))
}

func Atoi(buf string) (ret int, valid bool, pos string) {
	if len(buf) == 0 {
		return ret, false, buf
	}

	i := 0
	for ; unicode.IsSpace(rune(buf[i])); i++ {
	}

	r := buf[i]
	if r == '-' || r == '+' {
		i++
	}

	for ; i != len(buf); i++ {
		d := int(buf[i] - '0')
		if d < 0 || d > 9 {
			break
		}

		valid = true
		ret *= 10
		ret += d
	}

	if r == '-' {
		ret *= -1
	}

	return ret, valid, buf[i:]
}

type autobrrFilterUpdate struct {
	APIKey      string
	FilterID    int
	AutobrrHost string
	upgradereq
}

func handleAutobrrFilterUpdate(w http.ResponseWriter, r *http.Request) {
	var req autobrrFilterUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if req.FilterID == 0 {
		http.Error(w, fmt.Sprintf("Missing FilterID\n"), 473)
		return
	}

	tmp := upgradereq{
		Host:     req.Host,
		User:     req.User,
		Password: req.Password,
	}

	if err := getClient(&tmp); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 471)
		return
	}

	req.Client = tmp.Client
	mp := req.getAllTorrents()
	if mp.err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", mp.err), 468)
		return
	}

	singlemap := make(map[string]struct{})
	sane := regexp.MustCompile(`(\?+\?)`)
	replace := regexp.MustCompile("([\x00-\\/\\:-@\\[-\\`\\{-\\~])")

	for _, t := range mp.d {
		singlemap[sane.ReplaceAllString(
			replace.ReplaceAllString(
				strings.ToValidUTF8(
					strings.ToLower(t.Title),
					"?"),
				"?"),
			"*")] = struct{}{}
	}

	submit := struct {
		Shows string
	}{}

	for k := range singlemap {
		if len(k) < 1 {
			continue
		}

		submit.Shows += k + ","
	}

	submit.Shows = strings.Trim(submit.Shows, " ,")

	singlemap = nil
	body, err := json.Marshal(submit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to marshall qbittorrent data: %q\n", err), 465)
		return
	}

	newreq, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, req.AutobrrHost+"/api/filters/"+fmt.Sprintf("%d", req.FilterID), bytes.NewBuffer(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to create new http request: %q\n", err), 463)
		return
	}

	newreq.Header.Add("X-API-Token", req.APIKey)

	client := http.Client{
		Timeout: 90 * time.Second,
	}

	res, err := client.Do(newreq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to send to autobrr request: %q\n", err), 452)
		return
	}

	defer res.Body.Close()
	if _, err := httputil.DumpResponse(res, true); err != nil {
		http.Error(w, fmt.Sprintf("Unable to dump filter response: %q\n", err), 443)
		return
	}

	if res.StatusCode != http.StatusNoContent {
		http.Error(w, fmt.Sprintf("Bad code from Autobrr: %d\n", res.StatusCode), 442)
		return
	}

	http.Error(w, fmt.Sprintf("Success: %d\n", len(submit.Shows)), 200)
}

type upgraderrExpression struct {
	Query  string
	Action string
	upgradereq
}

func handleExpression(w http.ResponseWriter, r *http.Request) {
	var req upgraderrExpression
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	prog, err := expr.Compile(req.Query, expr.AsBool(),
		expr.Env(qbittorrent.Torrent{}),
		expr.Function(
			"Now",
			func(params ...any) (any, error) {
				return time.Now().Unix(), nil
			},
		),
		expr.Function(
			"State",
			func(params ...any) (any, error) {
				return string(params[0].(qbittorrent.TorrentState)), nil
			},
		),
	)

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to compile expression: %q\n", err), 472)
		return
	}

	tmp := upgradereq{
		Host:     req.Host,
		User:     req.User,
		Password: req.Password,
	}

	if err := getClient(&tmp); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 471)
		return
	}

	req.Client = tmp.Client

	mp := req.getAllTorrents()
	if mp.err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", mp.err), 468)
		return
	}

	hashes := make([]string, 0)
	for _, te := range mp.e {
		filterhash := make([]string, 0, len(te))
		for _, e := range te {
			res, err := expr.Run(prog, e.t)
			if err != nil {
				fmt.Printf("Error: %q\n", err)
				filterhash = []string{}
				break
			} else if res == false {
				filterhash = []string{}
				break
			}

			filterhash = append(filterhash, e.t.Hash)
		}

		hashes = append(hashes, filterhash...)
	}

	switch strings.Trim(strings.ToLower(req.Action), `"' `) {
	case "delete":
		if err := req.Client.DeleteTorrents(hashes, false); err != nil {
			http.Error(w, fmt.Sprintf("Unable to delete torrents: %q\n", mp.err), 419)
			return
		}
	case "deletedata":
		if err := req.Client.DeleteTorrents(hashes, true); err != nil {
			http.Error(w, fmt.Sprintf("Unable to deletedata torrents: %q\n", mp.err), 418)
			return
		}
	case "forcestart":
		if err := req.Client.SetForceStart(hashes, true); err != nil {
			http.Error(w, fmt.Sprintf("Unable to forcestart torrents: %q\n", mp.err), 417)
			return
		}
	case "normalstart":
		if err := req.Client.SetForceStart(hashes, false); err != nil {
			http.Error(w, fmt.Sprintf("Unable to normalstart torrents: %q\n", mp.err), 416)
			return
		}
	case "start":
		if err := req.Client.Resume(hashes); err != nil {
			http.Error(w, fmt.Sprintf("Unable to resume torrents: %q\n", mp.err), 415)
			return
		}
	case "pause":
		if err := req.Client.Pause(hashes); err != nil {
			http.Error(w, fmt.Sprintf("Unable to pause torrents: %q\n", mp.err), 414)
			return
		}
	case "reannounce":
		if err := req.Client.ReAnnounceTorrents(hashes); err != nil {
			http.Error(w, fmt.Sprintf("Unable to reannounce torrents: %q\n", mp.err), 413)
			return
		}
	case "recheck":
		if err := req.Client.Recheck(hashes); err != nil {
			http.Error(w, fmt.Sprintf("Unable to recheck torrents: %q\n", mp.err), 412)
			return
		}
	case "setcategory":
		//req.Client.SetCategory(hashes)
	default:
		for _, h := range hashes {
			req.Hash = h
			t, _ := req.getTorrent()
			fmt.Printf("Matched: %q\n", t.Name)
		}
		fmt.Printf("TEST count: %d\n", len(hashes))
	}

	fmt.Printf("Hashes: %d\n", len(hashes))
	http.Error(w, fmt.Sprintf("Processed: %q\n", len(hashes)), 200)
}
