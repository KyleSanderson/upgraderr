package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/titlerr/upgraderr/logic"
	"github.com/titlerr/upgraderr/models"
)

// ExpressionRequest extends UpgradeRequest with expression-specific fields
type ExpressionRequest struct {
	models.UpgradeRequest
	Expression     string `json:"expression"`
	Priority       int64  `json:"priority"`
	Action         string `json:"action"`
	Subject        string `json:"subject"`
	ResultLimit    int    `json:"resultLimit"`
	ResultSkip     int    `json:"resultSkip"`
	ResultMinCount int    `json:"resultMinCount"`
}

// HandleExpression processes expression-based filtering of torrents
func HandleExpression(w http.ResponseWriter, r *http.Request) {
	var req ExpressionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 470)
		return
	}

	if len(req.Expression) == 0 {
		http.Error(w, fmt.Sprintf("No expression passed.\n"), 469)
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

	// Compile expression
	program, err := expr.Compile(req.Expression)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to compile expression: %q\n", err), 472)
		return
	}

	resultLimit := req.ResultLimit
	if resultLimit == 0 {
		resultLimit = -1
	}

	resultSkip := req.ResultSkip
	if resultSkip == 0 {
		resultSkip = -1
	}

	resultMinimumCount := req.ResultMinCount
	if resultMinimumCount == 0 {
		resultMinimumCount = -1
	}

	// Store results by priority
	hashmap := make(map[int64][]string)
	priority := req.Priority
	if priority == 0 {
		priority = 100
	}

	// Process torrents
	for _, torrents := range mp.Data {
		filterhash := make([]string, 0)

		for _, torrent := range torrents {
			// Create environment for expression evaluation
			env := map[string]interface{}{
				"name":         torrent.Name,
				"hash":         torrent.Hash,
				"category":     torrent.Category,
				"tags":         torrent.Tags,
				"size":         torrent.Size,
				"progress":     torrent.Progress,
				"completionOn": torrent.CompletionOn,
				"ratio":        torrent.Ratio,
				"state":        torrent.State,
				"path":         torrent.ContentPath,
				// Add any other fields needed for evaluation
			}

			// Run the expression
			result, err := expr.Run(program, env)
			if err != nil {
				continue
			}

			// If expression returns true, add to filter hashes
			if b, ok := result.(bool); ok && b {
				filterhash = append(filterhash, torrent.Hash)
			}
		}

		if len(filterhash) == 0 {
			continue
		} else if _, ok := hashmap[priority]; ok {
			hashmap[priority] = append(hashmap[priority], filterhash...)
		} else {
			hashmap[priority] = filterhash
		}
	}

	// Sort keys by priority (highest first)
	keys := make([]int64, 0, len(hashmap))
	for k := range hashmap {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[j] < keys[i] })

	// Combine results in priority order
	hashes := make([]string, 0)
	for _, k := range keys {
		hashes = append(hashes, hashmap[k]...)
	}

	// Apply result filtering
	if resultMinimumCount > -1 && len(hashes) < resultMinimumCount {
		hashes = nil
	}

	if resultSkip > -1 {
		if len(hashes) > resultSkip {
			hashes = hashes[resultSkip:]
		} else {
			hashes = nil
		}
	}

	if resultLimit > -1 && len(hashes) > resultLimit {
		hashes = hashes[:resultLimit]
	}

	// Process requested action
	switch strings.Trim(strings.ToLower(req.Action), `"' `) {
	case "delete":
		if err := req.Client.DeleteTorrents(hashes, false); err != nil {
			http.Error(w, fmt.Sprintf("Unable to delete torrents: %q\n", err), 419)
			return
		}
	case "deletedata":
		if err := req.Client.DeleteTorrents(hashes, true); err != nil {
			http.Error(w, fmt.Sprintf("Unable to deletedata torrents: %q\n", err), 418)
			return
		}
	case "forcestart":
		if err := req.Client.SetForceStart(hashes, true); err != nil {
			http.Error(w, fmt.Sprintf("Unable to forcestart torrents: %q\n", err), 417)
			return
		}
	case "normalstart":
		if err := req.Client.SetForceStart(hashes, false); err != nil {
			http.Error(w, fmt.Sprintf("Unable to normalstart torrents: %q\n", err), 416)
			return
		}
	case "start":
		if err := req.Client.Resume(hashes); err != nil {
			http.Error(w, fmt.Sprintf("Unable to resume torrents: %q\n", err), 415)
			return
		}
	case "pause":
		if err := req.Client.Pause(hashes); err != nil {
			http.Error(w, fmt.Sprintf("Unable to pause torrents: %q\n", err), 414)
			return
		}
	case "reannounce":
		if err := req.Client.ReAnnounceTorrents(hashes); err != nil {
			http.Error(w, fmt.Sprintf("Unable to reannounce torrents: %q\n", err), 413)
			return
		}
	case "recheck":
		if err := req.Client.Recheck(hashes); err != nil {
			http.Error(w, fmt.Sprintf("Unable to recheck torrents: %q\n", err), 412)
			return
		}
	case "category":
		if err := req.Client.SetCategory(hashes, req.Subject); err != nil {
			http.Error(w, fmt.Sprintf("Unable to category torrents %q: %q\n", req.Subject, err), 411)
			return
		}
	case "tagadd":
		if err := req.Client.AddTags(hashes, req.Subject); err != nil {
			http.Error(w, fmt.Sprintf("Unable to addtag torrents %q: %q\n", req.Subject, err), 410)
			return
		}
	case "tagdel":
		if err := req.Client.RemoveTags(hashes, req.Subject); err != nil {
			http.Error(w, fmt.Sprintf("Unable to tagdel torrents %q: %q\n", req.Subject, err), 409)
			return
		}
	default:
		for _, h := range hashes {
			req.Hash = h
			t, _ := logic.GetTorrent(&req.UpgradeRequest)
			fmt.Printf("Matched: %q\n", t.Name)
		}
		fmt.Printf("TEST count: %d\n", len(hashes))
	}

	http.Error(w, fmt.Sprintf("Processed: %d\n", len(hashes)), 200)
}
