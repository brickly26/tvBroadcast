package handlers

import (
	"live-broadcast-backend/state"
	"net/http"
	"path/filepath"
	"strings"
	
	"github.com/gorilla/mux"
)

// SetupVideoRoutes configures routes for secure video serving
func SetupVideoRoutes(router *mux.Router, cm *state.ChannelManager, videoDir string) {
	// Create a file server for the videos directory
	// But we'll wrap it with our own handler for security
	fileServer := http.FileServer(http.Dir(videoDir))
	
	// Register the secure video server handler
	router.PathPrefix("/videos/").Handler(SecureVideoHandler(fileServer, cm, videoDir))
}

// SecureVideoHandler creates a handler that serves videos securely
// This ensures clients can only access videos that are currently scheduled
func SecureVideoHandler(fileServer http.Handler, cm *state.ChannelManager, videoDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Clean the path to prevent directory traversal attacks
		path := r.URL.Path
		cleanPath := filepath.Clean(strings.TrimPrefix(path, "/videos/"))
		
		// Get the list of valid S3 keys from the channel manager
		validKeys := cm.GetAllValidS3Keys()
		
		// Check if the requested path is a valid video
		isValid := false
		for _, key := range validKeys {
			if cleanPath == key {
				isValid = true
				break
			}
		}
		
		if !isValid {
			http.Error(w, "Forbidden: Video not currently scheduled", http.StatusForbidden)
			return
		}
		
		// Rewrite the request path to point to the actual file
		r.URL.Path = "/" + cleanPath
		
		// Let the file server handle the actual serving
		fileServer.ServeHTTP(w, r)
	}
}
