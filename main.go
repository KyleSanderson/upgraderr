package main

import (
	"fmt"
	"github.com/autobrr/autobrr/pkg/qbittorrent"
	"github.com/moistari/rls"
	"os"
	"strconv"
	"unicode"
	"time"
)

type Entry struct {
	t qbittorrent.Torrent
	r rls.Release
}

func main() {
	port, _ := strconv.ParseInt(os.Getenv("port"), 10, 16)

	c := qbittorrent.NewClient(qbittorrent.Settings{
		Hostname: os.Getenv("host"),
		Port:     uint(port),
		Username: os.Getenv("user"),
		Password: os.Getenv("password"),
	})

	if err := c.Login(); err != nil {
		fmt.Printf("Unable to login: %q\n", err)
		return
	}

	torrents, err := c.GetTorrentsFilter(qbittorrent.TorrentFilterStalledUploading)
	if err != nil {
		fmt.Printf("Unable to get Torrents: %q\n", err)
		return
	}

	m := make(map[string][]Entry)
	for _, t := range torrents {
		r := rls.ParseString(t.Name)
		s := fmt.Sprintf("%s%s%s%04d%02d%02d%02d%03d", rls.MustNormalize(r.Artist), rls.MustNormalize(r.Title), rls.MustNormalize(r.Subtitle), r.Year, r.Month, r.Day, r.Series, r.Episode)

		for _, a := range r.Cut {
			s += rls.MustNormalize(a)
		}

		for _, a := range r.Edition {
			s += rls.MustNormalize(a)
		}

		m[s] = append(m[s], Entry{t: t, r: r})
	}

	unix := int(time.Now().Unix() - (60*60*24*10))
	for _, v := range m {
		if len(v) < 2 {
			continue
		}
		
		var parent Entry
		for _, child := range v {
			if len(parent.t.Name) == 0 {
				parent = child
				continue
			}

			if rls.Compare(parent.r, child.r) == 0 {
				continue
			}

			if res := checkResolution(&parent, &child); res != nil {
				if src := checkSource(&parent, &child); src == res || src == nil {
					parent = *res
					continue
				}
			}

			if res := checkHDR(&parent, &child); res != nil {
				parent = *res
				continue
			}

			if res := checkChannels(&parent, &child); res != nil {
				parent = *res
				continue
			}

			if res := checkSource(&parent, &child); res != nil {
				parent = *res
				continue
			}

			if res := checkAudio(&parent, &child); res != nil {
				parent = *res
				continue
			}

			if res := checkExtension(&parent, &child); res != nil {
				parent = *res
				continue
			}

			if res := checkLanguage(&parent, &child); res != nil {
				parent = *res
				continue
			}

			if res := checkReplacement(&parent, &child); res != nil {
				parent = *res
				continue
			}
		}

		if len(parent.t.Name) == 0 {
			continue
		}

		for _, child := range v {
			if child.t.CompletionOn > unix {
				continue
			}

			if rls.Compare(child.r, parent.r) == 0 {
				continue
			}

			d := make([]qbittorrent.Torrent, 0, len(v))
			for _, n := range v {
				if rls.Compare(child.r, n.r) != 0 {
					continue
				}

				if n.t.CompletionOn > unix {
					d = nil
					break
				}

				d = append(d, n.t)
			}

			if len(d) == 0 {
				continue
			}

			fmt.Printf("Parent: %q\n", parent.t.Name)

			hashes := make([]string, 0, len(d))
			for _, t := range d {
				fmt.Printf("- Removing: %q\n", t.Name)
				hashes = append(hashes, t.Hash)
			}

			
			c.DeleteTorrents(hashes, true)
		}
	}
}

func checkExtension(parent, child *Entry) *Entry {
	sm := map[string]int {
		"mkv": 90,
		"mp4": 89,
		"webp": 88,
		"ts": 87,
		"wmv": 86,
		"xvid": 85,
		"divx": 84,
	}

	return compareResults(parent, child, func(e rls.Release) int {
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

func checkLanguage(parent, child *Entry) *Entry {
	sm := map[string]int {
		"ENGLiSH": 2,
		"MULTi": 1,
	}

	return compareResults(parent, child, func(e rls.Release) int {
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

func checkReplacement(parent, child *Entry) *Entry {
	if rls.MustNormalize(child.r.Group) != rls.MustNormalize(parent.r.Group) {
		return nil
	}

	sm := map[string]int {
		"COMPLETE": 0,
		"REMUX": 1,
		"EXTENDED": 2,
		"REMASTERED": 3,
		"PROPER": 4,
		"REPACK": 5,
		"INTERNAL": 6,
	}

	return compareResults(parent, child, func(e rls.Release) int {
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

func checkAudio(parent, child *Entry) *Entry {
	sm := map[string]int {
		"DTS-HD.HRA": 90,
		"DDPA": 89,
		"TrueHD": 88,
		"DTS-HD.MA": 87,
		"DTS-HD.HR": 86,
		"Atmos": 85,
		"DTS-HD": 84,
		"DDP": 83,
		"DD": 82,
		"OPUS": 81,
		"AAC": 80,
	}

	return compareResults(parent, child, func(e rls.Release) int {
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

func checkSource(parent, child *Entry) *Entry {
	if child.r.Source == parent.r.Source {
		return nil
	}

	sm := map[string]int {
		"WEB-DL": 90,
		"UHD.BluRay": 89,
		"BluRay": 88,
		"WEB": 87,
		"WEBRiP": 86,
		"BDRiP": 85,
		"HDRiP": 84,
		"HDTV": 83,
		"DVDRiP": 82,
		"HDTC": 81,
		"HDTS": 80,
		"TC": 79,
		"VHSRiP": 78,
		"WORKPRiNT": 77,
		"TS": 76,
	}

	return compareResults(parent, child, func(e rls.Release) int {
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

func checkChannels(parent, child *Entry) *Entry {
	if child.r.Channels == parent.r.Channels {
		return nil
	}

	return compareResults(parent, child, func(e rls.Release) int {
		i, _ := strconv.ParseFloat(e.Channels, 8)
		
		if i == 0.0 {
			i = 2.0
		}

		return int(i * 10)
	})
}

func checkHDR(parent, child *Entry) *Entry {
	sm := map[string]int {
		"DV": 90,
		"HDR10+": 89,
		"HDR10": 88,
		"HDR+": 87,
		"HDR": 86,
		"HLG": 85,
		"SDR": 84,
	}

	return compareResults(parent, child, func(e rls.Release) int {
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

func checkResolution(parent, child *Entry) *Entry {
	if child.r.Resolution == parent.r.Resolution {
		return nil
	}

	return compareResults(parent, child, func(e rls.Release) int {
		i := Atoi(e.Resolution)
		if i == 0 {
			i = 480
		}

		return i
	})
}

func compareResults(parent, child *Entry, f func(rls.Release)int) *Entry {
	parentv := f(parent.r)
	childv := f(child.r)

	if childv > parentv {
		return child
	} else if parentv > childv {
		return parent
	}

	return nil
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
