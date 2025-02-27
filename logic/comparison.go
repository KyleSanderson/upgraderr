package logic

import (
	"fmt"
	"strconv"

	"github.com/autobrr/autobrr/pkg/ttlcache"
	"github.com/moistari/rls"
	"github.com/titlerr/upgraderr/cache"
	"github.com/titlerr/upgraderr/models"
	"github.com/titlerr/upgraderr/utils"
)

// CacheTitle retrieves or parses a release title
func CacheTitle(title string) *rls.Release {
	r, ok := cache.TitleMap.Get(title)
	if !ok {
		local := rls.ParseString(title)
		r = &local

		cache.TitleMap.Set(title, r, ttlcache.DefaultTTL)
	}

	return r
}

// CacheFormatted retrieves or formats a title
func CacheFormatted(title string) string {
	r, ok := cache.FormattedMap.Get(title)
	if !ok {
		r = getFormattedTitle(title)
		cache.FormattedMap.Set(title, r, ttlcache.DefaultTTL)
	}

	return r
}

// getFormattedTitle returns a formatted title
func getFormattedTitle(title string) string {
	return getReleaseTitle(CacheTitle(title))
}

// getReleaseTitle creates a normalized title from a Release
func getReleaseTitle(r *rls.Release) string {
	s := fmt.Sprintf("%s%s%s%s%s%04d%02d%02d%02d%03d",
		rls.MustNormalize(r.Artist),
		rls.MustNormalize(r.Title),
		rls.MustNormalize(r.Subtitle),
		rls.MustNormalize(r.Alt),
		rls.MustNormalize(r.Version),
		r.Year, r.Month, r.Day, r.Series, r.Episode)

	for _, a := range r.Cut {
		s += rls.MustNormalize(a)
	}

	for _, a := range r.Edition {
		s += rls.MustNormalize(a)
	}

	return s
}

// CompareResults compares releases using the provided comparison function
func CompareResults(requestrls, child *models.Entry, f func(*rls.Release) int) *models.Entry {
	requestrlsv := f(requestrls.Release)
	childv := f(child.Release)

	if childv > requestrlsv {
		return child
	} else if requestrlsv > childv {
		return requestrls
	}

	return nil
}

// CheckResolution compares the resolution of releases
func CheckResolution(requestrls, child *models.Entry) *models.Entry {
	if child.Release.Resolution == requestrls.Release.Resolution {
		return nil
	}

	return CompareResults(requestrls, child, func(e *rls.Release) int {
		i, _, _ := Atoi(e.Resolution)
		if i == 0 {
			i = 480
		}

		return i
	})
}

// CheckHDR compares the HDR of releases
func CheckHDR(requestrls, child *models.Entry) *models.Entry {
	sm := map[string]int{
		"DoVi":   90,
		"DV":     90,
		"HDR10+": 89,
		"HDR10":  88,
		"HDR+":   87,
		"HDR":    86,
		"HLG":    85,
		"SDR":    84,
	}

	return CompareResults(requestrls, child, func(e *rls.Release) int {
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

// Atoi extracts an integer from a string within the logic package
func Atoi(buf string) (ret int, valid bool, pos string) {
	return utils.Atoi(buf)
}

// CheckChannels compares the audio channels of releases
func CheckChannels(requestrls, child *models.Entry) *models.Entry {
	if child.Release.Channels == requestrls.Release.Channels {
		return nil
	}

	return CompareResults(requestrls, child, func(e *rls.Release) int {
		i, _ := strconv.ParseFloat(e.Channels, 8)
		if i == 0.0 {
			i = 2.0
		}
		return int(i * 10)
	})
}

// CheckSource compares the source of releases
func CheckSource(requestrls, child *models.Entry) *models.Entry {
	if child.Release.Source == requestrls.Release.Source {
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

	return CompareResults(requestrls, child, func(e *rls.Release) int {
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

// CheckAudio compares the audio codecs of releases
func CheckAudio(requestrls, child *models.Entry) *models.Entry {
	sm := map[string]int{
		"FLAC":       94,
		"LPCM":       93,
		"DTS-X":      92,
		"DTS-HD.HRA": 91,
		"DDPA":       90,
		"TrueHD":     89,
		"DTS-HD.MA":  88,
		"DTS-MA":     87,
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

	return CompareResults(requestrls, child, func(e *rls.Release) int {
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

// CheckExtension compares file extensions of releases
func CheckExtension(requestrls, child *models.Entry) *models.Entry {
	sm := map[string]int{
		"mkv":  90,
		"mp4":  89,
		"webp": 88,
		"ts":   87,
		"wmv":  86,
		"xvid": 85,
		"divx": 84,
	}

	return CompareResults(requestrls, child, func(e *rls.Release) int {
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

// CheckLanguage compares languages of releases
func CheckLanguage(requestrls, child *models.Entry) *models.Entry {
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

	return CompareResults(requestrls, child, func(e *rls.Release) int {
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

// CheckReplacement compares replacement status of releases
func CheckReplacement(requestrls, child *models.Entry) *models.Entry {
	if rls.MustNormalize(child.Release.Group) != rls.MustNormalize(requestrls.Release.Group) {
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

	return CompareResults(requestrls, child, func(e *rls.Release) int {
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

// Other comparison functions would be similarly refactored...
