package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moistari/rls"
	"github.com/titlerr/upgraderr/cache"
	"github.com/titlerr/upgraderr/logic"
	"github.com/titlerr/upgraderr/models"
)

// HandleClean processes clean requests to remove outdated torrents
func HandleClean(w http.ResponseWriter, r *http.Request) {
	var req models.UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if err := logic.GetClient(&req); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 471)
		return
	}

	mp, err := logic.GetAllTorrents(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", err), 468)
		return
	}

	t := cache.GlobalTime.Now().Unix()
	hashes := make([]string, 0)

	for _, v := range mp.Data {
		if len(v) == 0 {
			continue
		}

		parent := models.Entry{Release: logic.CacheTitle(v[0].Name), Torrent: v[0]}
		parentMap := make(map[string]int)

		for _, t := range v {
			child := models.Entry{Torrent: t, Release: logic.CacheTitle(t.Name)}
			if rls.Compare(*parent.Release, *child.Release) == 0 {
				parentMap[child.Torrent.Name]++
				continue
			}

			if res := logic.CheckResolution(&parent, &child); res != nil {
				src := logic.CheckSource(&parent, &child)
				if src == nil {
					parent = *res
					parentMap = map[string]int{parent.Torrent.Name: 1}
					continue
				} else if src.Torrent.Hash == res.Torrent.Hash {
					parent = *src
					parentMap = map[string]int{parent.Torrent.Name: 1}
					continue
				}
			}

			bFailed := false
			for _, f := range []func(*models.Entry, *models.Entry) *models.Entry{
				logic.CheckHDR, logic.CheckChannels, logic.CheckSource,
				logic.CheckAudio, logic.CheckExtension, logic.CheckLanguage,
				logic.CheckReplacement} {

				if res := f(&parent, &child); res != nil && res.Torrent.Hash != parent.Torrent.Hash {
					parent = *res
					parentMap = map[string]int{parent.Torrent.Name: 1}
					bFailed = true
					break
				}
			}

			if !bFailed {
				parentMap[child.Torrent.Name]++
			}
		}

		if len(parentMap) == 0 {
			continue
		}

		var parentName string
		parentNumber := 0
		for k, i := range parentMap {
			if i > parentNumber {
				parentNumber = i
				parentName = k
			}
		}

		fmt.Printf("Parent: %q\n", parentName)

		parentrls := *logic.CacheTitle(parentName)
		for _, child := range v {
			childrls := *logic.CacheTitle(child.Name)
			if rls.Compare(childrls, parentrls) == 0 {
				continue
			}

			bContinue := false
			childHashes := make([]string, 0, len(v))
			for _, subChild := range v {
				if rls.Compare(*logic.CacheTitle(subChild.Name), childrls) != 0 {
					continue
				}

				if subChild.CompletionOn < 1 || t-int64(subChild.CompletionOn) < 1209600 {
					bContinue = true
					break
				}

				fmt.Printf("Removing: %q\n", subChild.Name)
				childHashes = append(childHashes, subChild.Hash)
			}

			if bContinue {
				continue
			}

			hashes = append(hashes, childHashes...)
		}
	}

	if len(hashes) == 0 {
		http.Error(w, "No eligible torrents to remove.", 205)
		return
	}

	if err := req.Client.DeleteTorrents(hashes, true); err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit %d torrents to remove: %s", len(hashes), err), 420)
		return
	}

	http.Error(w, fmt.Sprintf("Removed %d torrents.", len(hashes)), 200)
}
