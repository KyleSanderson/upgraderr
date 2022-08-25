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
	"encoding/json"
	"fmt"
	"github.com/autobrr/autobrr/pkg/qbittorrent"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/moistari/rls"
	"github.com/sasha-s/go-deadlock"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
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
}

type timeentry struct {
	e   map[string][]Entry
	t   time.Time
	err error
}

type clientmap struct {
	c map[string]chan func(timeentry)
	deadlock.RWMutex
}

var gm = clientmap{c: make(map[string]chan func(timeentry))}

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		gm.RLock()
		gm.RUnlock()

		w.Write([]byte("k8s"))
	})

	r.Post("/api/upgrade", handleUpgrade)
	http.ListenAndServe(":6940", r) /* immutable. this is b's favourite positive 4digit number not starting with a 0. */
}

func heartbeat(w http.ResponseWriter, r *http.Request) {
	gm.RLock()
	defer gm.RUnlock()

	http.Error(w, "Alive", 200)
}

func (c *clientmap) getEntries(req upgradereq) chan func(timeentry) {
	client := req.Host + fmt.Sprintf("%d", req.Port) + req.User + req.Password

	c.RLock()
	if ch, ok := c.c[client]; ok {
		c.RUnlock()
		return ch
	}
	c.RUnlock()

	ch := make(chan func(timeentry), 500)

	c.Lock()
	c.c[client] = ch
	c.Unlock()

	go processReleasesLoop(ch, qbittorrent.Settings{
		Hostname: req.Host,
		Port:     req.Port,
		Username: req.User,
		Password: req.Password,
	})

	return ch
}

func processReleasesLoop(ch chan func(timeentry), s qbittorrent.Settings) {
	mp := timeentry{e: make(map[string][]Entry), t: time.Time{}}

	for {
		select {
		case f := <-ch:
			{
				if mp.t.Unix()+60 > time.Now().Unix() {
					go f(mp)
					continue
				}

				c := qbittorrent.NewClient(s)
				if err := c.Login(); err != nil {
					clone := mp
					clone.err = err
					go f(clone)
					continue
				}

				torrents, err := c.GetTorrents()
				if err != nil {
					clone := mp
					clone.err = err
					go f(clone)
					continue
				}

				mp = timeentry{e: make(map[string][]Entry), t: time.Now()}
				for _, t := range torrents {
					r := rls.ParseString(t.Name)
					s := getFormattedTitle(r)
					mp.e[s] = append(mp.e[s], Entry{t: t, r: r})
				}

				go f(mp)
			}
		}
	}
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

	ch := make(chan timeentry)
	gm.getEntries(req) <- func(e timeentry) {
		ch <- e
	}

	requestrls := Entry{r: rls.ParseString(req.Name)}
	mp := <-ch

	if mp.err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", mp.err), 468)
		return
	}

	if v, ok := mp.e[getFormattedTitle(requestrls.r)]; ok {
		code := 0
		var parent Entry
		for _, child := range v {
			if rls.Compare(requestrls.r, child.r) == 0 {
				parent = child
				code = -1
				break
			}

			if res := checkResolution(&requestrls, &child); res != nil {
				if src := checkSource(&requestrls, &child); src == res || src == nil {
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

		if code == -1 {
			http.Error(w, fmt.Sprintf("Cross submission: %q\n", req.Name), 250)
		} else if code != 0 {
			http.Error(w, fmt.Sprintf("Not an upgrade submission: %q => %q\n", req.Name, parent.t.Name), code)
		} else {
			http.Error(w, fmt.Sprintf("Upgrade submission: %q\n", req.Name), 200)
		}
	} else {
		http.Error(w, fmt.Sprintf("Unique submission: %q\n", req.Name), 200)
	}
}

func getFormattedTitle(r rls.Release) string {
	s := fmt.Sprintf("%s%s%s%04d%02d%02d%02d%03d", Normalize(r.Artist), Normalize(r.Title), Normalize(r.Subtitle), r.Year, r.Month, r.Day, r.Series, r.Episode)
	for _, a := range r.Cut {
		s += Normalize(a)
	}

	for _, a := range r.Edition {
		s += Normalize(a)
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
		"ENGLiSH": 2,
		"MULTi":   1,
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
	if Normalize(child.r.Group) != Normalize(requestrls.r.Group) {
		return nil
	}

	sm := map[string]int{
		"COMPLETE":   0,
		"REMUX":      1,
		"EXTENDED":   2,
		"REMASTERED": 3,
		"PROPER":     4,
		"REPACK":     5,
		"INTERNAL":   6,
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
		"DTS-HD.HRA": 90,
		"DDPA":       89,
		"TrueHD":     88,
		"DTS-HD.MA":  87,
		"DTS-HD.HR":  86,
		"Atmos":      85,
		"DTS-HD":     84,
		"DDP":        83,
		"DD":         82,
		"OPUS":       81,
		"AAC":        80,
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

			i = sm["AAC"]
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
		i := Atoi(e.Resolution)
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

func Atoi(buf string) (ret int) {
	if len(buf) == 0 {
		return ret
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

		ret *= 10
		ret += d
	}

	if r == '-' {
		ret *= -1
	}

	return ret
}