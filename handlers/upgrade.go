package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moistari/rls"
	"github.com/titlerr/upgraderr/logic"
	"github.com/titlerr/upgraderr/models"
)

// HandleUpgrade processes upgrade requests
func HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	var req models.UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if len(req.Name) == 0 {
		http.Error(w, fmt.Sprintf("No title passed.\n"), 469)
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

	v, ok := mp.Data[logic.CacheFormatted(req.Name)]
	if !ok {
		http.Error(w, fmt.Sprintf("Unique submission: %q\n", req.Name), 200)
		return
	}

	code := 0
	var parent models.Entry
	requestrls := models.Entry{Release: logic.CacheTitle(req.Name)}

	for _, childtor := range v {
		child := models.Entry{Torrent: childtor, Release: logic.CacheTitle(childtor.Name)}

		if rls.Compare(*requestrls.Release, *child.Release) == 0 {
			if child.Torrent.Progress < parent.Torrent.Progress {
				code = 240 + int(parent.Torrent.Progress*10.0)
				continue
			}

			parent = child
			code = 240 + int(child.Torrent.Progress*10.0)
			if code >= 250 {
				code = 250
				/* wtf. API breakage... but assume it's ok */
				break
			}

			continue
		}

		if res := logic.CheckResolution(&requestrls, &child); res != nil && res.Torrent != requestrls.Torrent {
			if src := logic.CheckSource(&requestrls, &child); src == nil || src.Torrent != requestrls.Torrent {
				parent = *res
				code = 201
				break
			}
		}

		for i, f := range []func(*models.Entry, *models.Entry) *models.Entry{
			logic.CheckHDR,
			logic.CheckChannels,
			logic.CheckSource,
			logic.CheckAudio,
			logic.CheckExtension,
			logic.CheckLanguage,
			logic.CheckReplacement} {

			if res := f(&requestrls, &child); res != nil && res.Torrent != requestrls.Torrent {
				parent = *res
				code = 202 + i
				break
			}
		}
	}

	if code >= 240 && code <= 250 {
		http.Error(w, fmt.Sprintf("Cross submission: %q\n", req.Name), code)
	} else if code > 200 && code < 240 {
		http.Error(w, fmt.Sprintf("Not an upgrade submission: %q => %q\n", req.Name, parent.Torrent.Name), code)
	} else {
		http.Error(w, fmt.Sprintf("Upgrade submission: %q\n", req.Name), 200)
	}
}
