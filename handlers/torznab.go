package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/titlerr/upgraderr/logic"
	"github.com/titlerr/upgraderr/models"
)

// TorznabCrossSearch extends UpgradeRequest with Jackett-specific fields
type TorznabCrossSearch struct {
	models.UpgradeRequest
	APIKey      string `json:"apiKey"`
	JackettHost string `json:"jackettHost"`
	AgeLimit    uint   `json:"ageLimit"`
}

// HandleTorznabCrossSearch handles Torznab cross-search requests
func HandleTorznabCrossSearch(w http.ResponseWriter, r *http.Request) {
	var req models.TorznabRequest

	// Parse the request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	// Validate required fields
	if len(req.Title) == 0 {
		http.Error(w, "No title provided in request", 499)
		return
	}

	if len(req.TorrentData) == 0 {
		http.Error(w, "No torrent data provided in request", 498)
		return
	}

	// Convert to a standard UpgradeRequest with the necessary fields
	upgradeReq := models.UpgradeRequest{
		Name:     req.Title,
		Torrent:  req.TorrentData,
		Host:     req.Host,
		Port:     uint(req.Port),
		User:     req.Username,
		Password: req.Password,
	}

	// Initialize the client
	if err := logic.GetClient(&upgradeReq); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 498)
		return
	}

	// Use the shared logic function for cross-seeding
	result, code, _ := logic.ProcessCrossSeed(&upgradeReq)
	http.Error(w, result, code)
}
