package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"regexp"
	"sort"
	"strings"

	"github.com/titlerr/upgraderr/logic"
	"github.com/titlerr/upgraderr/models"
)

var saneFilter = regexp.MustCompile(`(\?+\?)`)
var replaceFilter = regexp.MustCompile("([\\x00-\\/\\:-@\\[-\\`\\{-\\~])")

// AutobrrFilterRequest extends UpgradeRequest with Autobrr-specific fields
type AutobrrFilterRequest struct {
	models.UpgradeRequest
	AutobrrHost string `json:"autobrrHost"`
	APIKey      string `json:"apiKey"`
	FilterID    int    `json:"filterId"`
}

// HandleAutobrrFilterUpdate handles requests to update Autobrr filters
func HandleAutobrrFilterUpdate(w http.ResponseWriter, r *http.Request) {
	var req AutobrrFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if req.FilterID == 0 {
		http.Error(w, "Missing FilterID\n", 473)
		return
	}

	if err := logic.GetClient(&req.UpgradeRequest); err != nil {
		http.Error(w, fmt.Sprintf("Unable to get client: %q\n", err), 471)
		return
	}

	mp, err := logic.GetAllTorrents(&req.UpgradeRequest)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to get result: %q\n", err), 468)
		return
	}

	singlemap := make(map[string]struct{})

	// Extract and sanitize all torrent titles
	for _, entries := range mp.Data {
		for _, t := range entries {
			title := logic.CacheTitle(t.Name).Title
			sanitized := saneFilter.ReplaceAllString(
				replaceFilter.ReplaceAllString(
					strings.ToValidUTF8(
						strings.ToLower(title),
						"?"),
					"?"),
				"*")

			if len(sanitized) > 0 {
				singlemap[sanitized] = struct{}{}
			}
		}
	}

	// Convert map to sorted slice
	buf := make([]string, 0, len(singlemap))
	for k := range singlemap {
		buf = append(buf, k)
	}
	sort.Strings(buf)

	// Prepare data for sending to Autobrr
	submit := struct {
		Shows string
	}{
		Shows: strings.Trim(strings.Join(buf, ","), " ,"),
	}

	body := &bytes.Buffer{}
	{
		enc := json.NewEncoder(body)
		enc.SetEscapeHTML(false)

		if err := enc.Encode(submit); err != nil {
			http.Error(w, fmt.Sprintf("Unable to marshall qbittorrent data: %q\n", err), 465)
			return
		}
	}

	// Create request to update Autobrr filter
	newreq, err := http.NewRequestWithContext(context.Background(), http.MethodPatch,
		req.AutobrrHost+"/api/filters/"+fmt.Sprintf("%d", req.FilterID), body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to create new http request: %q\n", err), 463)
		return
	}

	newreq.Header.Add("X-API-Token", req.APIKey)

	client := &http.Client{}
	res, err := client.Do(newreq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to send to autobrr request: %q\n", err), 452)
		return
	}

	defer res.Body.Close()
	if _, err := httputil.DumpResponse(res, true); err != nil {
		http.Error(w, fmt.Sprintf("Unable to dump filter response: %q\n", err), 443)
		return
	}

	if res.StatusCode != http.StatusNoContent {
		http.Error(w, fmt.Sprintf("Bad code from Autobrr: %d\n", res.StatusCode), 442)
		return
	}

	http.Error(w, fmt.Sprintf("Success: %d\n", len(submit.Shows)), 200)
}
