package handlers

import (
	"live-broadcast-backend/state"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// SetupLiveStreamRoutes registers /live/{channel}
func SetupLiveStreamRoutes(r *mux.Router, cm *state.ChannelManager) {
	r.HandleFunc("/live/{number:[0-9]+}", LiveStreamHandler(cm))
}

func LiveStreamHandler(cm *state.ChannelManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		numStr := mux.Vars(r)["number"]
		channelNum, _ := strconv.Atoi(numStr)

		// grab broadcaster
		bc := cm.GetBroadcaster(channelNum)
		if bc == nil {
			http.Error(w, "channel offline", http.StatusNotFound)
			return
		}
		bc.AddClient(w, r)
	}
}