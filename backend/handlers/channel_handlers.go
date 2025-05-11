package handlers

import (
	"encoding/json"
	"live-broadcast-backend/state"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// SetupChannelRoutes configures the routes related to channels.
func SetupChannelRoutes(router *mux.Router, cm *state.ChannelManager) {
	router.HandleFunc("/api/channels", GetChannelGuideHandler(cm)).Methods("GET")
	router.HandleFunc("/api/channels/{number:[0-9]+}", GetChannelStateHandler(cm)).Methods("GET")
}

// ChannelStateResponse is the structure returned by the GetChannelStateHandler.
// It includes the video URL and the calculated current time offset.
type ChannelStateResponse struct {
	ChannelNumber int     `json:"channelNumber"`
	ChannelName   string  `json:"channelName"`
	VideoTitle    string  `json:"videoTitle"`
	VideoURL      string  `json:"videoUrl"`
	CurrentTime   float64 `json:"currentTime"` // Current playback time in seconds
	VideoDuration float64 `json:"videoDuration"`
}

// GetChannelGuideHandler returns a handler function that provides the channel guide info.
func GetChannelGuideHandler(cm *state.ChannelManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		guideInfo := cm.GetAllChannelGuideInfo()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(guideInfo); err != nil {
			http.Error(w, "Failed to encode channel guide", http.StatusInternalServerError)
			return
		}
	}
}

// GetChannelStateHandler returns a handler function that provides the detailed current state of a specific channel.
func GetChannelStateHandler(cm *state.ChannelManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		channelNumberStr, ok := vars["number"]
		if !ok {
			http.Error(w, "Channel number missing in URL", http.StatusBadRequest)
			return
		}

		channelNumber, err := strconv.Atoi(channelNumberStr)
		if err != nil {
			http.Error(w, "Invalid channel number", http.StatusBadRequest)
			return
		}

		state, err := cm.GetChannelState(channelNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if state.CurrentVideo == nil {
			http.Error(w, "Channel is currently offline (no video assigned)", http.StatusNotFound)
			return
		}

		// Calculate the current playback time
		now := time.Now()
		elapsedTime := now.Sub(state.VideoStartTime).Seconds()
		currentTime := elapsedTime
		// Ensure currentTime doesn't exceed video duration (though scheduler should handle this)
		if currentTime > state.CurrentVideo.Duration {
			currentTime = state.CurrentVideo.Duration
		}
		if currentTime < 0 {
            currentTime = 0 // Should not happen, but defensive check
        }

		response := ChannelStateResponse{
			ChannelNumber: state.Channel.Number,
			ChannelName:   state.Channel.Name,
			VideoTitle:    state.CurrentVideo.Title,
			VideoURL:      state.CurrentVideo.URL, // Use the URL from the Video struct
			CurrentTime:   currentTime,
			VideoDuration: state.CurrentVideo.Duration,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode channel state", http.StatusInternalServerError)
			return
		}
	}
} 