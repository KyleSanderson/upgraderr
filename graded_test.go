package main

import (
	"testing"

	"github.com/autobrr/go-qbittorrent"
)

// --- Graded ladders: every codec/attribute, asserted monotonic ------------

// gradedCase is one step in a quality ladder. Each entry must be strictly
// better than the previous one under the relevant attribute, and decideBetter
// must agree.
type gradedCase struct {
	name string
	hash string
}

// assertLadder verifies that for a list ordered best-first, decideBetter
// always returns the better (earlier) entry, and that the best entry wins
// against every later entry.
func assertLadder(t *testing.T, ladder []gradedCase) {
	t.Helper()
	entries := make([]Entry, len(ladder))
	for i, c := range ladder {
		entries[i] = mkEntry(c.name, c.hash)
	}

	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			got := decideBetter(&entries[i], &entries[j])
			if got == nil {
				t.Fatalf("[%s] expected %q to beat %q, got nil (equal?)", t.Name(), ladder[i].hash, ladder[j].hash)
			}
			if got.t.Hash != ladder[i].hash {
				t.Fatalf("[%s] expected %q to beat %q, got %q", t.Name(), ladder[i].hash, ladder[j].hash, got.t.Hash)
			}
			// Symmetric: the worse one must not win.
			gotRev := decideBetter(&entries[j], &entries[i])
			if gotRev == nil || gotRev.t.Hash != ladder[i].hash {
				t.Fatalf("[%s] reverse: expected %q to beat %q, got %v", t.Name(), ladder[i].hash, ladder[j].hash, gotRev)
			}
		}
	}
}

func TestGradedResolution(t *testing.T) {
	assertLadder(t, []gradedCase{
		{"M.2023.2160p.x264-GRP", "r2160"},
		{"M.2023.1440p.x264-GRP", "r1440"},
		{"M.2023.1080p.x264-GRP", "r1080"},
		{"M.2023.720p.x264-GRP", "r720"},
		{"M.2023.576p.x264-GRP", "r576"},
		{"M.2023.480p.x264-GRP", "r480"},
	})
}

func TestGradedSource(t *testing.T) {
	// WEB-DL is superior to BluRay by design.
	assertLadder(t, []gradedCase{
		{"M.2023.1080p.WEB-DL.x264-GRP", "sWEBDL"},
		{"M.2023.1080p.UHD.BluRay.x264-GRP", "sUHDBR"},
		{"M.2023.1080p.BluRay.x264-GRP", "sBR"},
		{"M.2023.1080p.WEB.x264-GRP", "sWEB"},
		{"M.2023.1080p.WEBRiP.x264-GRP", "sWEBRIP"},
		{"M.2023.1080p.BDRiP.x264-GRP", "sBDRIP"},
		{"M.2023.1080p.HDRiP.x264-GRP", "sHDRIP"},
		{"M.2023.1080p.HDTV.x264-GRP", "sHDTV"},
		{"M.2023.1080p.DVDRiP.x264-GRP", "sDVDRIP"},
		{"M.2023.1080p.HDTC.x264-GRP", "sHDTC"},
		{"M.2023.1080p.HDTS.x264-GRP", "sHDTS"},
		{"M.2023.1080p.TC.x264-GRP", "sTC"},
		{"M.2023.1080p.VHSRiP.x264-GRP", "sVHS"},
		{"M.2023.1080p.WORKPRiNT.x264-GRP", "sWP"},
		{"M.2023.1080p.TS.x264-GRP", "sTS"},
		{"M.2023.1080p.HDCAM.x264-GRP", "sHDCAM"},
		{"M.2023.1080p.CAM.x264-GRP", "sCAM"},
	})
}

func TestGradedHDR(t *testing.T) {
	// DoVi and DV are the same format and must score identically (not be
	// graded against each other).
	assertLadder(t, []gradedCase{
		{"M.2023.2160p.WEB-DL.HDR10+.x264-GRP", "hHDR10p"},
		{"M.2023.2160p.WEB-DL.HDR10.x264-GRP", "hHDR10"},
		{"M.2023.2160p.WEB-DL.HDR+.x264-GRP", "hHDRp"},
		{"M.2023.2160p.WEB-DL.HDR.x264-GRP", "hHDR"},
		{"M.2023.2160p.WEB-DL.HLG.x264-GRP", "hHLG"},
		{"M.2023.2160p.WEB-DL.SDR.x264-GRP", "hSDR"},
	})
}

func TestHDRDoViEqualsDV(t *testing.T) {
	a := mkEntry("M.2023.2160p.WEB-DL.DoVi.x264-GRP", "a")
	b := mkEntry("M.2023.2160p.WEB-DL.DV.x264-GRP", "b")
	if got := decideBetter(&a, &b); got != nil {
		t.Fatalf("DoVi and DV must be equal, got %v", got)
	}
}

func TestGradedAudio(t *testing.T) {
	// All 2.0 channel so channels don't interfere. Atmos is additive (small
	// bonus), so a codec with Atmos outranks the same codec without it but
	// stays below the lossless tier (FLAC/LPCM/PCM/DTS-X).
	assertLadder(t, []gradedCase{
		{"M.2023.1080p.WEB-DL.FLAC.2.0.x264-GRP", "aFLAC"},
		{"M.2023.1080p.WEB-DL.LPCM.2.0.x264-GRP", "aLPCM"},
		{"M.2023.1080p.WEB-DL.DTS-X.2.0.x264-GRP", "aDTSX"},
		{"M.2023.1080p.WEB-DL.TrueHD.Atmos.2.0.x264-GRP", "aTHDA"},
		{"M.2023.1080p.WEB-DL.TrueHD.2.0.x264-GRP", "aTHD"},
		{"M.2023.1080p.WEB-DL.DTS-HD.HRA.2.0.x264-GRP", "aDTSHRA"},
		{"M.2023.1080p.WEB-DL.DDPA.2.0.x264-GRP", "aDDPA"},
		{"M.2023.1080p.WEB-DL.DTS-HD.MA.2.0.x264-GRP", "aDTSHDMA"},
		{"M.2023.1080p.WEB-DL.DTS-MA.2.0.x264-GRP", "aDTSMA"},
		{"M.2023.1080p.WEB-DL.DTS-HD.HR.2.0.x264-GRP", "aDTSHR"},
		{"M.2023.1080p.WEB-DL.Atmos.2.0.x264-GRP", "aAtmos"},
		{"M.2023.1080p.WEB-DL.DTS-HD.2.0.x264-GRP", "aDTSHD"},
		{"M.2023.1080p.WEB-DL.DDP.Atmos.2.0.x264-GRP", "aDDPA2"},
		{"M.2023.1080p.WEB-DL.DDP.2.0.x264-GRP", "aDDP"},
		{"M.2023.1080p.WEB-DL.DTS.2.0.x264-GRP", "aDTS"},
		{"M.2023.1080p.WEB-DL.DD.Atmos.2.0.x264-GRP", "aDDA"},
		{"M.2023.1080p.WEB-DL.DD.2.0.x264-GRP", "aDD"},
		{"M.2023.1080p.WEB-DL.OPUS.2.0.x264-GRP", "aOPUS"},
		{"M.2023.1080p.WEB-DL.AAC.2.0.x264-GRP", "aAAC"},
		{"M.2023.1080p.WEB-DL.MP3.2.0.x264-GRP", "aMP3"},
		{"M.2023.1080p.WEB-DL.DUAL.AUDIO.2.0.x264-GRP", "aDUAL"},
	})
}

func TestGradedChannels(t *testing.T) {
	assertLadder(t, []gradedCase{
		{"M.2023.1080p.WEB-DL.DDP7.1.x264-GRP", "c71"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264-GRP", "c51"},
		{"M.2023.1080p.WEB-DL.DDP2.0.x264-GRP", "c20"},
		{"M.2023.1080p.WEB-DL.DDP1.0.x264-GRP", "c10"},
	})
}

func TestGradedExtension(t *testing.T) {
	// rls only populates Ext when the container is at the very end of the
	// name (after the group), which is how announces typically arrive.
	// (webp is not recognized by rls as an extension, so it is excluded.)
	assertLadder(t, []gradedCase{
		{"M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP.mkv", "eMKV"},
		{"M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP.mp4", "eMP4"},
		{"M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP.ts", "eTS"},
		{"M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP.wmv", "eWMV"},
		{"M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP.xvid", "eXVID"},
		{"M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP.divx", "eDIVX"},
	})
}

func TestGradedLanguage(t *testing.T) {
	assertLadder(t, []gradedCase{
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.ENGLiSH-GRP", "lENG"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.MULTi-GRP", "lMULTI"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.FRENCH-GRP", "lFR"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.SWEDiSH-GRP", "lSWE"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.SWESUB-GRP", "lSWESUB"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.NORWEGiAN-GRP", "lNOR"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.NORDiCSUBS-GRP", "lNORDSUB"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.DUBBED-GRP", "lDUB"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.DANiSH-GRP", "lDAN"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.HiNDI-GRP", "lHIN"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.NORDiC-GRP", "lNORD"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.GERMAN-GRP", "lGER"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.SUBBED-GRP", "lSUB"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.CZECH-GRP", "lCZE"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.RUSSiAN-GRP", "lRUS"},
	})
}

func TestGradedReplacement(t *testing.T) {
	// Replacement only compares within the same group. Higher map value =
	// better (INTERNAL=8 ... COMPLETE=1). (EXTENDED is not parsed by rls as
	// an Other token, so it is excluded.)
	assertLadder(t, []gradedCase{
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.INTERNAL-GRP", "oINT"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.REPACK-GRP", "oREP"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.PROPER-GRP", "oPRO"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.REMASTERED-GRP", "oREM"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.FS-GRP", "oFS"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.REMUX-GRP", "oREMUX"},
		{"M.2023.1080p.WEB-DL.DDP5.1.x264.COMPLETE-GRP", "oCOMP"},
	})
}

// --- Aggressive: cross-attribute priority must hold ------------------------

// TestPrioritySourceDominates verifies source beats every lower attribute
// even when the lower attribute is maximally superior. A 4K VHS rip must NOT
// beat a 1080p WEB-DL.
func TestPrioritySourceDominates(t *testing.T) {
	goodSrc := mkEntry("M.2023.1080p.WEB-DL.DoVi.TrueHD.Atmos.7.1.FLAC.mkv.ENGLiSH.REMUX-GRP", "good")
	badSrc := mkEntry("M.2023.2160p.VHSRiP.SDR.DDP1.0.AAC.divx.RUSSiAN-GRP", "bad")

	if got := decideBetter(&badSrc, &goodSrc); got == nil || got.t.Hash != "good" {
		t.Fatalf("WEB-DL must beat VHSRiP regardless of resolution/other attrs, got %v", got)
	}
}

func TestPriorityResolutionWithinSource(t *testing.T) {
	// When sources are equal, resolution decides: 4K WEB-DL beats 1080p
	// WEB-DL even though the 1080p has better audio.
	hiRes := mkEntry("M.2023.2160p.WEB-DL.DDP2.0.H.264-GRP", "hi")
	loRes := mkEntry("M.2023.1080p.WEB-DL.FLAC.7.1.H.264-GRP", "lo")

	if got := decideBetter(&loRes, &hiRes); got == nil || got.t.Hash != "hi" {
		t.Fatalf("4K WEB-DL must beat 1080p WEB-DL when source is equal, got %v", got)
	}
}

func TestPriorityChannelsDominatesAudio(t *testing.T) {
	// 7.1 channels (higher priority) must beat 2.0 with better audio.
	moreChan := mkEntry("M.2023.1080p.WEB-DL.DDP7.1.H.264-GRP", "mc")
	betterAudio := mkEntry("M.2023.1080p.WEB-DL.FLAC.2.0.H.264-GRP", "ba")

	if got := decideBetter(&betterAudio, &moreChan); got == nil || got.t.Hash != "mc" {
		t.Fatalf("7.1 channels must beat 2.0 FLAC, got %v", got)
	}
}

func TestPrioritySourceDominatesAudio(t *testing.T) {
	// WEB-DL (higher priority) must beat BluRay with better audio.
	web := mkEntry("M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "web")
	br := mkEntry("M.2023.1080p.BluRay.TrueHD.Atmos.H.264-GRP", "br")

	if got := decideBetter(&br, &web); got == nil || got.t.Hash != "web" {
		t.Fatalf("WEB-DL must beat BluRay even with worse audio, got %v", got)
	}
}

// --- Aggressive: clean must never delete the best across full ladders ------

func TestCleanKeepsTopOfEveryLadder(t *testing.T) {
	ladders := [][]gradedCase{
		{
			{"M.2023.2160p.x264-GRP", "best"},
			{"M.2023.1080p.x264-GRP", "a"},
			{"M.2023.720p.x264-GRP", "b"},
		},
		{
			{"M.2023.1080p.WEB-DL.x264-GRP", "best"},
			{"M.2023.1080p.BluRay.x264-GRP", "a"},
			{"M.2023.1080p.HDTV.x264-GRP", "b"},
		},
		{
			{"M.2023.2160p.WEB-DL.DoVi.x264-GRP", "best"},
			{"M.2023.2160p.WEB-DL.HDR.x264-GRP", "a"},
			{"M.2023.2160p.WEB-DL.SDR.x264-GRP", "b"},
		},
		{
			{"M.2023.1080p.WEB-DL.FLAC.2.0.x264-GRP", "best"},
			{"M.2023.1080p.WEB-DL.DDP.2.0.x264-GRP", "a"},
			{"M.2023.1080p.WEB-DL.AAC.2.0.x264-GRP", "b"},
		},
		{
			{"M.2023.1080p.WEB-DL.DDP7.1.x264-GRP", "best"},
			{"M.2023.1080p.WEB-DL.DDP5.1.x264-GRP", "a"},
			{"M.2023.1080p.WEB-DL.DDP2.0.x264-GRP", "b"},
		},
	}

	for _, ladder := range ladders {
		v := make([]qbittorrent.Torrent, 0, len(ladder))
		for _, c := range ladder {
			v = append(v, mkTorrent(c.name, c.hash, 30))
		}

		removed := simulateClean(v)
		if contains(removed, "best") {
			t.Fatalf("BUG: best of ladder %v was removed: %v", ladder, removed)
		}
		for _, c := range ladder[1:] {
			if !contains(removed, c.hash) {
				t.Fatalf("expected inferior %q to be removed: %v", c.hash, removed)
			}
		}
	}
}

// --- Aggressive: classifyNotUpgrade codes for every attribute --------------

func TestClassifyNotUpgradeCodes(t *testing.T) {
	cases := []struct {
		name     string
		req      string
		existing string
		wantCode int
	}{
		{"source", "M.2023.1080p.BluRay.DDP5.1.H.264-GRP", "M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", 201},
		{"resolution", "M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "M.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", 202},
		{"hdr", "M.2023.1080p.WEB-DL.SDR.DDP5.1.H.264-GRP", "M.2023.1080p.WEB-DL.HDR.DDP5.1.H.264-GRP", 203},
		{"channels", "M.2023.1080p.WEB-DL.DDP2.0.H.264-GRP", "M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", 204},
		{"audio", "M.2023.1080p.WEB-DL.AAC.2.0.H.264-GRP", "M.2023.1080p.WEB-DL.FLAC.2.0.H.264-GRP", 205},
		{"extension", "M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP.divx", "M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP.mkv", 206},
		{"language", "M.2023.1080p.WEB-DL.DDP5.1.x264.RUSSiAN-GRP", "M.2023.1080p.WEB-DL.DDP5.1.x264.ENGLiSH-GRP", 207},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := mkEntry(c.req, "req")
			existing := mkEntry(c.existing, "ex")
			code, parent := classifyNotUpgrade(&req, &existing)
			if code != c.wantCode {
				t.Fatalf("[%s] expected code %d, got %d", c.name, c.wantCode, code)
			}
			if parent.t.Hash != "ex" {
				t.Fatalf("[%s] expected existing to be parent, got %q", c.name, parent.t.Hash)
			}
		})
	}
}

// --- Aggressive: replacement only within same group ------------------------

func TestReplacementDifferentGroupIgnored(t *testing.T) {
	// Different groups: replacement must not decide, so the better source
	// (WEB-DL) wins overall.
	a := mkEntry("M.2023.1080p.WEB-DL.DDP5.1.H.264-GROUPA", "a")
	b := mkEntry("M.2023.1080p.BluRay.DDP5.1.H.264-GROUPB", "b")

	if got := decideBetter(&a, &b); got == nil || got.t.Hash != "a" {
		t.Fatalf("expected WEB-DL (hash a) to win across groups, got %v", got)
	}
}

func TestReplacementSameGroupRespected(t *testing.T) {
	// Same group: a REPACK must beat a plain release.
	plain := mkEntry("M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "plain")
	repack := mkEntry("M.2023.1080p.WEB-DL.DDP5.1.H.264.REPACK-GRP", "repack")

	if got := decideBetter(&plain, &repack); got == nil || got.t.Hash != "repack" {
		t.Fatalf("expected REPACK (hash repack) to win within group, got %v", got)
	}
}

// --- Aggressive: unknown tokens fall back gracefully -----------------------

func TestUnknownAudioFallsBackToDualAudio(t *testing.T) {
	// An unrecognized audio token should not outrank a known good one.
	known := mkEntry("M.2023.1080p.WEB-DL.FLAC.2.0.H.264-GRP", "known")
	unknown := mkEntry("M.2023.1080p.WEB-DL.WHOKNOWS.2.0.H.264-GRP", "unknown")

	if got := decideBetter(&unknown, &known); got == nil || got.t.Hash != "known" {
		t.Fatalf("expected known FLAC (hash known) to beat unknown audio, got %v", got)
	}
}

func TestUnknownSourceFallsBackToTS(t *testing.T) {
	// An unrecognized source should not outrank a known good one.
	known := mkEntry("M.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "known")
	unknown := mkEntry("M.2023.1080p.WHOKNOWS.DDP5.1.H.264-GRP", "unknown")

	if got := decideBetter(&unknown, &known); got == nil || got.t.Hash != "known" {
		t.Fatalf("expected known WEB-DL (hash known) to beat unknown source, got %v", got)
	}
}
