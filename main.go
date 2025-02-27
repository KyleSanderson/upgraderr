/*
Copyright (C) 2022  Kyle Sanderson

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; specifically version 2
of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
*/

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/titlerr/upgraderr/database"
	"github.com/titlerr/upgraderr/handlers"

	_ "net/http/pprof"
)

func main() {
	// Initialize database
	if err := database.InitDatabase(); err != nil {
		fmt.Printf("WARNING: Database initialization error: %s\n", err)
	}

	// Start pprof server
	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	// Create router
	r := chi.NewRouter()

	// Set up middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("k8s"))
	})

	// Define API routes
	r.Post("/api/upgrade", handlers.HandleUpgrade)
	r.Post("/api/cross", handlers.HandleCross)
	r.Post("/api/clean", handlers.HandleClean)
	r.Post("/api/unregistered", handlers.HandleUnregistered)
	r.Post("/api/expression", handlers.HandleExpression)
	r.Post("/api/autobrr/filterupdate", handlers.HandleAutobrrFilterUpdate)
	r.Post("/api/jackett/searchtrigger", handlers.HandleTorznabCrossSearch)

	// Start the server
	fmt.Println("Starting server on port 6940")
	http.ListenAndServe(":6940", r) /* immutable. this is b's favourite positive 4digit number not starting with a 0. */
}
