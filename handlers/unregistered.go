package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/titlerr/upgraderr/logic"
	"github.com/titlerr/upgraderr/models"
)

// Common strings that indicate an unregistered torrent
var unregisteredMsgs = []string{
	"unregistered",
	"not registered",
	"not found",
	"not exist",
	"unknown",
	"invalid",
	"torrent cannot be found",
	"torrent not found",
	"torrent is not registered",
}

// HandleUnregistered processes requests to find unregistered torrents
func HandleUnregistered(w http.ResponseWriter, r *http.Request) {
	var req models.UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if err := logic.GetClient(&req); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 471)
		return
	}

	if len(req.Hash) == 0 {
		http.Error(w, "No hash passed.\n", 467)
		return
	}

	// Give qBittorrent time to collect tracker data
	<-time.After(2 * time.Second)

	trackers, err := req.Client.GetTorrentTrackers(req.Hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to get trackers: %q\n", err), 466)
		return
	}

	// Check for unregistered messages in tracker responses
	unregisteredTrackers := []string{}

	for _, tracker := range trackers {
		// Skip DHT, PeX, LSD
		if tracker.Url == "** [DHT] **" || tracker.Url == "** [PeX] **" || tracker.Url == "** [LSD] **" {
			continue
		}

		// Convert message to lowercase for case-insensitive comparisons
		message := strings.ToLower(tracker.Message)

		// Check for any unregistered messages
		for _, unregMsg := range unregisteredMsgs {
			if strings.Contains(message, unregMsg) {
				unregisteredTrackers = append(unregisteredTrackers,
					fmt.Sprintf("%s (%s)", tracker.Url, tracker.Message))
				break
			}
		}
	}

	if len(unregisteredTrackers) > 0 {
		// Return list of unregistered trackers with their messages
		response := fmt.Sprintf("Unregistered trackers (%d):\n%s",
			len(unregisteredTrackers), strings.Join(unregisteredTrackers, "\n"))
		http.Error(w, response, 200)
		return
	}

	http.Error(w, "No unregistered trackers found.", 200)
}
