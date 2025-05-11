package handlers

import (
	"encoding/json"
	"live-broadcast-backend/database"
	"live-broadcast-backend/services"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AdminVideoRequest is the request body for uploading a video
type AdminVideoRequest struct {
	YoutubeURL    string `json:"youtubeUrl"`
	ChannelNumber int    `json:"channelNumber"`
}

// VideoDeleteRequest is the request body for deleting a video
type VideoDeleteRequest struct {
	VideoID string `json:"videoId"`
}

// VideoOrderRequest is the request body for updating video order
type VideoOrderRequest struct {
	ChannelNumber int               `json:"channelNumber"`
	VideoOrders   map[string]int    `json:"videoOrders"`
}

// VideoThumbnailRequest is the request body for setting a video thumbnail
type VideoThumbnailRequest struct {
	VideoID      string `json:"videoId"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

// AdminHandler contains dependencies for admin handlers
type AdminHandler struct {
	db              *database.DB
	ytDownloader    *services.YouTubeDownloader
	videoService    *services.VideoService
	sessionDuration time.Duration
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *database.DB, ytDownloader *services.YouTubeDownloader, videoService *services.VideoService) *AdminHandler {
	return &AdminHandler{
		db:              db,
		ytDownloader:    ytDownloader,
		videoService:    videoService,
		sessionDuration: 24 * time.Hour, // Admin sessions last 24 hours
	}
}

// DeleteVideoHandler handles requests to delete a video from a channel
func (h *AdminHandler) DeleteVideoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify admin authentication
		userID, ok := h.isAuthenticated(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is admin
		isAdmin, err := h.db.IsUserAdmin(userID)
		if err != nil || !isAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Parse request body
		var req VideoDeleteRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			log.Printf("Failed to decode delete video request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.VideoID == "" {
			http.Error(w, "Video ID is required", http.StatusBadRequest)
			return
		}

		// Get the video details first to determine channel and position
		video, err := h.db.GetVideoByID(req.VideoID)
		if err != nil {
			log.Printf("Error fetching video %s for deletion: %v", req.VideoID, err)
			http.Error(w, "Failed to fetch video information: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Deleting video: %s, channel: %d, position: %d", req.VideoID, video.ChannelID, video.DisplayOrder)

		// Delete the video from S3 first
		err = h.videoService.DeleteVideo(video.ChannelID, req.VideoID, video.DisplayOrder)
		if err != nil {
			log.Printf("Warning: Error deleting video from S3: %v. Will continue with database deletion.", err)
			// Continue even if S3 deletion fails
		}

		// Delete the video from the database
		err = h.db.DeleteVideo(req.VideoID)
		if err != nil {
			log.Printf("Error deleting video %s from database: %v", req.VideoID, err)
			http.Error(w, "Failed to delete video from database: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Reorder remaining videos to ensure continuity
		err = h.db.ReorderVideosAfterDeletion(video.ChannelID)
		if err != nil {
			log.Printf("Warning: Error reordering videos after deletion: %v", err)
			// Continue even if reordering fails
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Video deleted successfully from S3 and database",
		})
	}
}

// UpdateVideoOrderHandler handles requests to update the display order of videos in a channel
func (h *AdminHandler) UpdateVideoOrderHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify admin authentication
		userID, ok := h.isAuthenticated(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is admin
		isAdmin, err := h.db.IsUserAdmin(userID)
		if err != nil || !isAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Parse request body
		var req VideoOrderRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			log.Printf("Failed to decode update video order request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.ChannelNumber < 1 || req.ChannelNumber > 5 {
			http.Error(w, "Invalid channel number", http.StatusBadRequest)
			return
		}
		if len(req.VideoOrders) == 0 {
			http.Error(w, "Video orders are required", http.StatusBadRequest)
			return
		}

		// Update the video order
		err = h.db.UpdateVideoOrder(req.ChannelNumber, req.VideoOrders)
		if err != nil {
			log.Printf("Error updating video order for channel %d: %v", req.ChannelNumber, err)
			http.Error(w, "Failed to update video order: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Video order updated successfully",
		})
	}
}

// Note: Thumbnail functionality removed as we now use S3 thumbnails directly with naming convention:
// thumbnails/channel_{channelID}_video{index}.jpg

// UploadVideoHandler handles requests to upload videos from YouTube to S3
func (h *AdminHandler) UploadVideoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("[UploadVideoHandler] Received request to upload video")
		// Verify admin authentication
		userID, ok := h.isAuthenticated(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is admin
		isAdmin, err := h.db.IsUserAdmin(userID)
		if err != nil || !isAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Parse request body
		var req AdminVideoRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			log.Printf("[UploadVideoHandler] Failed to decode request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		log.Printf("[UploadVideoHandler] Parsed request: YoutubeURL=%s ChannelNumber=%d", req.YoutubeURL, req.ChannelNumber)

		// Validate request
		if req.YoutubeURL == "" {
			log.Println("[UploadVideoHandler] Missing YouTube URL in request body")
			http.Error(w, "YouTube URL is required", http.StatusBadRequest)
			return
		}
		if req.ChannelNumber < 1 || req.ChannelNumber > 5 {
			log.Printf("[UploadVideoHandler] Invalid channel number: %d", req.ChannelNumber)
			http.Error(w, "Invalid channel number", http.StatusBadRequest)
			return
		}
		log.Println("[UploadVideoHandler] Request validated")

		// Process the video
		log.Printf("[UploadVideoHandler] Starting video processing for YouTube URL: %s", req.YoutubeURL)
		videoID, err := h.ytDownloader.ProcessVideo(req.YoutubeURL, req.ChannelNumber, userID)
		if err != nil {
			log.Printf("[UploadVideoHandler] Error processing video: %v", err)
			http.Error(w, "Failed to process video: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("[UploadVideoHandler] Video processing started successfully. VideoID: %s", videoID)

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Video processing has been queued",
			"videoId": videoID,
		})
		log.Println("[UploadVideoHandler] Success response sent to client")
	}
}

// GetChannelVideosHandler returns the list of videos for a channel
func (h *AdminHandler) GetChannelVideosHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify admin authentication
		userID, ok := h.isAuthenticated(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is admin
		isAdmin, err := h.db.IsUserAdmin(userID)
		if err != nil || !isAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Get channel ID from query parameter
		channelIDStr := r.URL.Query().Get("channel")
		if channelIDStr == "" {
			http.Error(w, "Channel ID is required", http.StatusBadRequest)
			return
		}

		channelID, err := strconv.Atoi(channelIDStr)
		if err != nil || channelID < 1 || channelID > 5 {
			http.Error(w, "Invalid channel ID", http.StatusBadRequest)
			return
		}

		// Get videos for the channel with detailed information
		videos, err := h.db.GetChannelVideosWithDetails(channelID)
		if err != nil {
			log.Printf("Error getting videos for channel %d: %v", channelID, err)
			http.Error(w, "Failed to get videos", http.StatusInternalServerError)
			return
		}

		// Return videos as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"videos": videos,
			"channel": channelID,
		})
	}
}

// LoginHandler handles admin login requests
func (h *AdminHandler) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate credentials
		user, err := h.db.ValidateUser(req.Username, req.Password)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Check if user is admin
		if user.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Set session cookie
		sessionToken := generateSessionToken()
		expiry := time.Now().Add(h.sessionDuration)
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Expires:  expiry,
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
			Secure:   r.TLS != nil, // Set Secure flag in production
		})

		// Store session in memory (in a real app, you'd use Redis or similar)
		sessions[sessionToken] = Session{
			UserID:    user.ID,
			ExpiresAt: expiry,
		}

		// Return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"user": map[string]interface{}{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
			},
		})
	}
}

// VerifyAdminHandler checks if the current user is an admin
func (h *AdminHandler) VerifyAdminHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify admin authentication
		userID, ok := h.isAuthenticated(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if user is admin
		isAdmin, err := h.db.IsUserAdmin(userID)
		if err != nil || !isAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// User is authenticated and is an admin
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"isAdmin": true,
		})
	}
}

// Simple in-memory session storage (use Redis or similar in production)
var sessions = make(map[string]Session)

// Session represents a user session
type Session struct {
	UserID    string
	ExpiresAt time.Time
}

// isAuthenticated checks if the request is authenticated
func (h *AdminHandler) isAuthenticated(r *http.Request) (string, bool) {
	// Get session token from cookie
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return "", false
	}

	sessionToken := cookie.Value
	session, exists := sessions[sessionToken]
	if !exists {
		return "", false
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		delete(sessions, sessionToken)
		return "", false
	}

	return session.UserID, true
}

// generateSessionToken generates a random session token
func generateSessionToken() string {
	// In a real app, use a secure crypto random method
	return "session_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

// ThumbnailHandler serves thumbnail images from S3
func (h *AdminHandler) ThumbnailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the thumbnail name from the URL path
		thumbnailPath := r.URL.Path
		thumbnailKey := strings.TrimPrefix(thumbnailPath, "/api/thumbnails/")
		
		if thumbnailKey == "" {
			http.Error(w, "Thumbnail key is required", http.StatusBadRequest)
			return
		}

		// Construct the complete S3 key for the thumbnail
		// The thumbnailKey is already in the correct format: thumbnail_{videoID}.jpg
		s3ThumbnailKey := "thumbnails/" + thumbnailKey
		
		log.Printf("Fetching thumbnail from S3: %s", s3ThumbnailKey)
		
		// Get a pre-signed URL for the thumbnail
		presignedURL, err := h.videoService.GetThumbnailURL(s3ThumbnailKey)
		if err != nil {
			log.Printf("Error getting thumbnail URL: %v", err)
			http.Error(w, "Thumbnail not found", http.StatusNotFound)
			return
		}
		
		// Redirect the client to the pre-signed URL
		http.Redirect(w, r, presignedURL, http.StatusTemporaryRedirect)
	}
}
