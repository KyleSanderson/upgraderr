package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/titlerr/upgraderr/logic"
	"github.com/titlerr/upgraderr/models"
)

// HandleCross handles cross-seeding torrents to the same client with special handling
func HandleCross(w http.ResponseWriter, r *http.Request) {
	var req models.UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if len(req.Name) == 0 {
		http.Error(w, "No title passed.\n", 499)
		return
	}

	if err := logic.GetClient(&req); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 498)
		return
	}

	// Use the shared logic function
	result, code, _ := logic.ProcessCrossSeed(&req)
	http.Error(w, result, code)
}
