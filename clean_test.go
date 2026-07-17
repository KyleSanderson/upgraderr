package main

import (
	"testing"
	"time"

	"github.com/autobrr/go-qbittorrent"
	"github.com/moistari/rls"
)

// mkEntry builds an Entry from a release title and a torrent hash.
func mkEntry(name, hash string) Entry {
	return Entry{
		t: qbittorrent.Torrent{Name: name, Hash: hash, Progress: 1.0, CompletionOn: 1},
		r: CacheTitle(name),
	}
}

// mkTorrent builds a qbittorrent.Torrent with a completion time `ageDays` ago.
func mkTorrent(name, hash string, ageDays int) qbittorrent.Torrent {
	completion := globalTime.Now().Unix() - int64(ageDays)*86400
	return qbittorrent.Torrent{
		Name:         name,
		Hash:         hash,
		Progress:     1.0,
		CompletionOn: completion,
	}
}

// --- decideBetter: fixed priority order -------------------------------------

func TestDecideBetterSourceWinsOverResolution(t *testing.T) {
	// Source is the highest priority. A 4K VHS rip must NOT beat a 1080p
	// WEB-DL, because VHSRiP is a far inferior source to WEB-DL.
	a := mkEntry("Movie.2023.2160p.VHSRiP.DDP5.1.H.264-GRP", "a")
	b := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "b")

	if got := decideBetter(&a, &b); got == nil || got.t.Hash != "b" {
		t.Fatalf("expected 1080p WEB-DL (hash b) to win on source, got %v", got)
	}
	if got := decideBetter(&b, &a); got == nil || got.t.Hash != "b" {
		t.Fatalf("expected 1080p WEB-DL (hash b) to win on source, got %v", got)
	}
}

func TestDecideBetterResolutionWithinSameSource(t *testing.T) {
	// When the source is equal, resolution decides. 4K WEB-DL beats 1080p
	// WEB-DL.
	a := mkEntry("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "a")
	b := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "b")

	if got := decideBetter(&a, &b); got == nil || got.t.Hash != "a" {
		t.Fatalf("expected 4K WEB-DL (hash a) to win on resolution, got %v", got)
	}
}

func TestDecideBetterSourceWinsOverAudio(t *testing.T) {
	// Same resolution; better source (WEB-DL) must beat better audio (BluRay).
	a := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "a")
	b := mkEntry("Movie.2023.1080p.BluRay.TrueHD.Atmos.H.264-GRP", "b")

	if got := decideBetter(&a, &b); got == nil || got.t.Hash != "a" {
		t.Fatalf("expected WEB-DL (hash a) to win on source, got %v", got)
	}
}

func TestDecideBetterAudioWinsWithinSameSource(t *testing.T) {
	// Same resolution, source and channels: better audio must win. Use a
	// 2.0 channel title for both so channels don't decide.
	a := mkEntry("Movie.2023.1080p.WEB-DL.DDP2.0.H.264-GRP", "a")
	b := mkEntry("Movie.2023.1080p.WEB-DL.TrueHD.Atmos.H.264-GRP", "b")

	if got := decideBetter(&a, &b); got == nil || got.t.Hash != "b" {
		t.Fatalf("expected TrueHD Atmos (hash b) to win on audio, got %v", got)
	}
}

func TestDecideBetterTrueHDAtmosBeatsDDP(t *testing.T) {
	// Regression: TrueHD Atmos must outrank plain DDP (the old map scored the
	// combined token as DUAL.AUDIO, below DDP).
	a := mkEntry("Movie.2023.1080p.WEB-DL.DDP2.0.H.264-GRP", "a")
	b := mkEntry("Movie.2023.1080p.WEB-DL.TrueHD.Atmos.H.264-GRP", "b")

	if got := decideBetter(&a, &b); got == nil || got.t.Hash != "b" {
		t.Fatalf("expected TrueHD Atmos (hash b) to beat DDP (hash a), got %v", got)
	}
}

func TestDecideBetterHDRWinsOverChannels(t *testing.T) {
	a := mkEntry("Movie.2023.1080p.BluRay.TrueHD.Atmos.H.264-GRP", "a") // no HDR
	b := mkEntry("Movie.2023.1080p.BluRay.HDR.DDP5.1.H.264-GRP", "b")   // HDR

	if got := decideBetter(&a, &b); got == nil || got.t.Hash != "b" {
		t.Fatalf("expected HDR (hash b) to win, got %v", got)
	}
}

func TestDecideBetterEqualReturnsNil(t *testing.T) {
	a := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "a")
	b := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP2", "b")

	if got := decideBetter(&a, &b); got != nil {
		t.Fatalf("expected nil for equal releases, got %v", got)
	}
}

func TestDecideBetterAudioPriority(t *testing.T) {
	// Better audio must beat better extension/language/replacement.
	a := mkEntry("Movie.2023.1080p.WEB-DL.DDP2.0.H.264-GRP", "a")       // DDP audio
	b := mkEntry("Movie.2023.1080p.WEB-DL.TrueHD.Atmos.H.264-GRP", "b") // TrueHD Atmos

	if got := decideBetter(&a, &b); got == nil || got.t.Hash != "b" {
		t.Fatalf("expected TrueHD Atmos (hash b) to win on audio, got %v", got)
	}
}

// --- classifyNotUpgrade (handleUpgrade) ------------------------------------

func TestClassifyNotUpgradeRejectsWorseResolution(t *testing.T) {
	req := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "req")
	existing := mkEntry("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "ex")

	code, parent := classifyNotUpgrade(&req, &existing)
	if code != 202 { // resolution is index 1 => 201+1
		t.Fatalf("expected code 202 (worse resolution), got %d", code)
	}
	if parent.t.Hash != "ex" {
		t.Fatalf("expected existing 4K to be parent, got %q", parent.t.Hash)
	}
}

func TestClassifyNotUpgradeAllowsWEBOverBluRay(t *testing.T) {
	// WEB-DL is superior to BluRay, so a WEB-DL request over an existing
	// BluRay is a genuine upgrade (not rejected).
	req := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "req")
	existing := mkEntry("Movie.2023.1080p.BluRay.DDP5.1.H.264-GRP", "ex")

	code, _ := classifyNotUpgrade(&req, &existing)
	if code != 0 {
		t.Fatalf("expected 0 (genuine upgrade: WEB-DL beats BluRay), got %d", code)
	}
}

func TestClassifyNotUpgradeRejectsWorseSource(t *testing.T) {
	// A BluRay request over an existing WEB-DL is rejected (WEB-DL superior).
	req := mkEntry("Movie.2023.1080p.BluRay.DDP5.1.H.264-GRP", "req")
	existing := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "ex")

	code, parent := classifyNotUpgrade(&req, &existing)
	if code != 201 { // checkSource is index 0 in the list => 201+0
		t.Fatalf("expected code 201 (worse source), got %d", code)
	}
	if parent.t.Hash != "ex" {
		t.Fatalf("expected existing WEB-DL to be parent, got %q", parent.t.Hash)
	}
}

func TestClassifyNotUpgradeAllows4KOverBluRay(t *testing.T) {
	// A 4K WEB-DL request is a genuine upgrade over an existing 1080p
	// BluRay: WEB-DL is a superior source to BluRay, so it is accepted
	// regardless of resolution.
	req := mkEntry("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "req")
	existing := mkEntry("Movie.2023.1080p.BluRay.DDP5.1.H.264-GRP", "ex")

	code, _ := classifyNotUpgrade(&req, &existing)
	if code != 0 {
		t.Fatalf("expected 0 (genuine upgrade: WEB-DL beats BluRay), got %d", code)
	}
}

func TestClassifyNotUpgradeRejectsBluRayOverWEB(t *testing.T) {
	// BluRay is NOT superior to WEB-DL, so a BluRay request over an existing
	// WEB-DL is rejected (WEB-DL wins on source).
	req := mkEntry("Movie.2023.1080p.BluRay.DDP5.1.H.264-GRP", "req")
	existing := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "ex")

	code, _ := classifyNotUpgrade(&req, &existing)
	if code != 201 {
		t.Fatalf("expected 201 (WEB-DL superior to BluRay), got %d", code)
	}
}

func TestClassifyNotUpgradeEqualRelease(t *testing.T) {
	req := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "req")
	existing := mkEntry("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP2", "ex")

	code, _ := classifyNotUpgrade(&req, &existing)
	if code != 0 {
		t.Fatalf("expected 0 (same release, not an upgrade), got %d", code)
	}
}

// --- collectCleanTargets / selectBest --------------------------------------

func TestSelectBestPicksHighestQuality(t *testing.T) {
	v := []qbittorrent.Torrent{
		mkTorrent("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "low", 30),
		mkTorrent("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "high", 30),
		mkTorrent("Movie.2023.1080p.BluRay.DDP5.1.H.264-GRP", "mid", 30),
	}

	targets := collectCleanTargets(v)
	best := selectBest(targets)
	if best == nil || best.t.Hash != "high" {
		t.Fatalf("expected 4K (hash high) to be best, got %v", best)
	}
}

func TestSelectBestIgnoresUnparseable(t *testing.T) {
	v := []qbittorrent.Torrent{
		mkTorrent("not a real release name at all", "junk", 30),
		mkTorrent("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "good", 30),
	}

	targets := collectCleanTargets(v)
	best := selectBest(targets)
	if best == nil || best.t.Hash != "good" {
		t.Fatalf("expected parseable (hash good) to be best, got %v", best)
	}
}

// --- isInferior ------------------------------------------------------------

func TestIsInferiorLowerQuality(t *testing.T) {
	best := mkEntry("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "best")
	low := mkTorrent("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "low", 30)

	if !isInferior(&low, &best) {
		t.Fatalf("expected 1080p to be inferior to 4K")
	}
}

func TestIsInferiorBestNotInferior(t *testing.T) {
	best := mkEntry("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "best")

	if isInferior(&best.t, &best) {
		t.Fatalf("the best torrent must never be inferior to itself")
	}
}

func TestIsInferiorDuplicateOfBest(t *testing.T) {
	best := mkEntry("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "best")
	dup := mkTorrent("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP2", "dup", 30)

	if isInferior(&dup, &best) {
		t.Fatalf("a duplicate of the best release must not be inferior")
	}
}

func TestIsInferiorEqualQualityDifferentRelease(t *testing.T) {
	// Two distinct releases of equal quality: neither is inferior to the
	// other, so both should be preserved.
	best := mkEntry("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "best")
	other := mkTorrent("Other.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "other", 30)

	if isInferior(&other, &best) {
		t.Fatalf("an equal-quality, distinct release must not be deleted")
	}
}

// --- handleClean end-to-end (via extracted helpers) ------------------------

// simulateClean returns the hashes that handleClean would delete for the
// given torrent set, using the same logic as handleClean.
func simulateClean(v []qbittorrent.Torrent) []string {
	targets := collectCleanTargets(v)
	best := selectBest(targets)
	t := globalTime.Now().Unix()

	hashes := make([]string, 0)
	for _, child := range v {
		if best != nil && rls.Compare(*CacheTitle(child.Name), *best.r) == 0 {
			continue
		}
		if !isInferior(&child, best) {
			continue
		}
		if child.CompletionOn < 1 || t-int64(child.CompletionOn) < 1209600 {
			continue
		}
		hashes = append(hashes, child.Hash)
	}
	return hashes
}

func TestCleanKeepsHigherQualityRegression(t *testing.T) {
	// Regression for "gets rid of higher quality stuff": a 4K WEB-DL and a
	// 1080p BluRay both exist. WEB-DL is the superior source, so the 4K
	// WEB-DL is kept and the 1080p BluRay is removed.
	v := []qbittorrent.Torrent{
		mkTorrent("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "uhd", 30),
		mkTorrent("Movie.2023.1080p.BluRay.DDP5.1.H.264-GRP", "hd", 30),
	}

	removed := simulateClean(v)
	if contains(removed, "uhd") {
		t.Fatalf("BUG: higher-quality 4K WEB-DL (uhd) was marked for removal: %v", removed)
	}
	if !contains(removed, "hd") {
		t.Fatalf("expected lower-source 1080p BluRay (hd) to be removed: %v", removed)
	}
}

func TestCleanSourceBeatsResolution(t *testing.T) {
	// Source is the highest priority: a 4K VHS rip must NOT be kept over a
	// 1080p WEB-DL. The 4K VHS is removed; the 1080p WEB-DL is kept.
	v := []qbittorrent.Torrent{
		mkTorrent("Movie.2023.2160p.VHSRiP.DDP5.1.H.264-GRP", "vhs4k", 30),
		mkTorrent("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "web1080", 30),
	}

	removed := simulateClean(v)
	if contains(removed, "web1080") {
		t.Fatalf("BUG: 1080p WEB-DL (web1080) was marked for removal: %v", removed)
	}
	if !contains(removed, "vhs4k") {
		t.Fatalf("expected inferior-source 4K VHS (vhs4k) to be removed: %v", removed)
	}
}

func TestCleanKeepsBestWhenMultipleInferior(t *testing.T) {
	v := []qbittorrent.Torrent{
		mkTorrent("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "best", 30),
		mkTorrent("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "a", 30),
		mkTorrent("Movie.2023.720p.WEB-DL.DDP5.1.H.264-GRP", "b", 30),
		mkTorrent("Movie.2023.1080p.HDTV.H.264-GRP", "c", 30),
	}

	removed := simulateClean(v)
	if contains(removed, "best") {
		t.Fatalf("BUG: best torrent removed: %v", removed)
	}
	for _, h := range []string{"a", "b", "c"} {
		if !contains(removed, h) {
			t.Fatalf("expected inferior %q to be removed: %v", h, removed)
		}
	}
}

func TestCleanRespectsAgeGuard(t *testing.T) {
	// Inferior torrent completed only 5 days ago: must NOT be removed.
	v := []qbittorrent.Torrent{
		mkTorrent("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "best", 30),
		mkTorrent("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "young", 5),
	}

	removed := simulateClean(v)
	if contains(removed, "young") {
		t.Fatalf("BUG: torrent younger than 14 days was removed: %v", removed)
	}
}

func TestCleanRemovesAfterAgeGuard(t *testing.T) {
	v := []qbittorrent.Torrent{
		mkTorrent("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "best", 30),
		mkTorrent("Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "old", 20),
	}

	removed := simulateClean(v)
	if !contains(removed, "old") {
		t.Fatalf("expected old inferior torrent to be removed: %v", removed)
	}
}

func TestCleanKeepsEqualQualityDistinctReleases(t *testing.T) {
	// Two different movies, both 1080p: neither is inferior, both kept.
	v := []qbittorrent.Torrent{
		mkTorrent("Alpha.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "a", 30),
		mkTorrent("Beta.2023.1080p.WEB-DL.DDP5.1.H.264-GRP", "b", 30),
	}

	removed := simulateClean(v)
	if len(removed) != 0 {
		t.Fatalf("expected no removals for distinct equal-quality releases, got %v", removed)
	}
}

func TestCleanKeepsDuplicateOfBest(t *testing.T) {
	// Two copies of the same best release: both kept (no false deletion).
	v := []qbittorrent.Torrent{
		mkTorrent("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP", "best1", 30),
		mkTorrent("Movie.2023.2160p.WEB-DL.DDP5.1.H.264-GRP2", "best2", 30),
	}

	removed := simulateClean(v)
	if len(removed) != 0 {
		t.Fatalf("expected no removals for duplicates of the best release, got %v", removed)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// Ensure the package still compiles with the time import used by tests.
var _ = time.Now
