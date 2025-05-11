package handlers

import (
	"encoding/json"
	"fmt"
	"live-broadcast-backend/services"
	"live-broadcast-backend/state"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketClient represents a connected WebSocket client.
type WebSocketClient struct {
	conn           *websocket.Conn
	channelManager *state.ChannelManager
	videoService   *services.VideoService
	currentChannel int
	isActive       bool
	mu             sync.Mutex
	pingTimer      *time.Timer
}

// WebSocketMessage represents messages sent between client and server.
type WebSocketMessage struct {
	Type        string      `json:"type"`
	Channel     int         `json:"channel,omitempty"`
	URL         string      `json:"url,omitempty"`
	CurrentTime float64     `json:"currentTime,omitempty"`
	Duration    float64     `json:"duration,omitempty"`
	VideoId     string      `json:"videoId,omitempty"`
	Title       string      `json:"title,omitempty"`
	Data        interface{} `json:"data,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSocketHandler handles WebSocket connections.
func WebSocketHandler(cm *state.ChannelManager, videoService *services.VideoService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Error upgrading connection to WebSocket: %v", err)
			return
		}

		// Configure the WebSocket connection
		conn.SetReadLimit(512 * 1024) // 512KB limit for messages
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			// Reset read deadline when we receive a pong
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		client := &WebSocketClient{
			conn:           conn,
			channelManager: cm,
			videoService:   videoService,
			currentChannel: 1, // Default to channel 1
			isActive:       true,
		}

		// Start the ping ticker to keep the connection alive
		go client.pingPump()

		// Start a goroutine to handle incoming messages
		go client.readPump()

		// Start a goroutine to send periodic video updates
		go client.sendVideoUpdates()

		log.Println("New WebSocket client connected")
	}
}

// pingPump sends regular pings to the client to keep the connection alive
func (c *WebSocketClient) pingPump() {
	pingInterval := 25 * time.Second
	pingTimeout := 5 * time.Second
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			isActive := c.isActive
			c.mu.Unlock()

			if !isActive {
				return
			}

			// Send ping message
			c.mu.Lock()
			err := c.conn.SetWriteDeadline(time.Now().Add(pingTimeout))
			if err != nil {
				c.mu.Unlock()
				log.Printf("Error setting write deadline: %v", err)
				return
			}

			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				c.mu.Unlock()
				log.Printf("Error sending ping: %v", err)
				return
			}
			c.mu.Unlock()
		}
	}
}

// readPump handles messages from the client.
func (c *WebSocketClient) readPump() {
	defer func() {
		c.mu.Lock()
		c.isActive = false
		c.conn.Close()
		c.mu.Unlock()
		log.Println("WebSocket client disconnected")
	}()

	// Set an initial read deadline
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Reset the read deadline when we get a message
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg WebSocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error parsing WebSocket message: %v", err)
			continue
		}

		// Handle different message types
		switch msg.Type {
		case "joinChannel":
			c.handleJoinChannel(msg.Channel)
		case "getChannelGuide":
			c.handleGetChannelGuide()
		default:
			log.Printf("Unknown WebSocket message type: %s", msg.Type)
		}
	}
}

// handleJoinChannel processes a request to join a specific channel.
func (c *WebSocketClient) handleJoinChannel(channelNumber int) {
	if channelNumber < 1 || channelNumber > 5 {
		log.Printf("Invalid channel number: %d", channelNumber)
		return
	}

	c.mu.Lock()
	c.currentChannel = channelNumber
	c.mu.Unlock()

	// Immediately send current state of this channel
	c.sendCurrentChannelState()
}

// handleGetChannelGuide processes a request for the channel guide.
func (c *WebSocketClient) handleGetChannelGuide() {
	guideInfo := c.channelManager.GetAllChannelGuideInfo()

	response := WebSocketMessage{
		Type: "channelGuide",
		Data: guideInfo,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isActive {
		return
	}

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.conn.WriteJSON(response); err != nil {
		log.Printf("Error sending channel guide: %v", err)
	}
}

// sendCurrentChannelState sends the current state of the client's channel.
func (c *WebSocketClient) sendCurrentChannelState() {
	c.mu.Lock()
	isActive := c.isActive
	channelNumber := c.currentChannel
	c.mu.Unlock()

	// Check if client is still active before proceeding
	if !isActive {
		return
	}

	state, err := c.channelManager.GetChannelState(channelNumber)
	if err != nil {
		log.Printf("Error getting channel state for channel %d: %v", channelNumber, err)
		// Send an empty state instead of failing
		emptyResponse := WebSocketMessage{
			Type:        "videoUpdate",
			Channel:     channelNumber,
			URL:         "",
			CurrentTime: 0,
			Duration:    0,
		}
		
		c.mu.Lock()
		defer c.mu.Unlock()
		
		if !c.isActive {
			return
		}
		
		c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.conn.WriteJSON(emptyResponse); err != nil {
			log.Printf("Error sending empty video update: %v", err)
			// Don't close the connection here, let the error handling in readPump handle it
		}
		return
	}

	if state.CurrentVideo == nil {
		log.Printf("No current video for channel %d", channelNumber)
		// Send an empty state instead of failing
		emptyResponse := WebSocketMessage{
			Type:        "videoUpdate",
			Channel:     channelNumber,
			URL:         "",
			CurrentTime: 0,
			Duration:    0,
		}
		
		c.mu.Lock()
		defer c.mu.Unlock()
		
		if !c.isActive {
			return
		}
		
		c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.conn.WriteJSON(emptyResponse); err != nil {
			log.Printf("Error sending empty video update: %v", err)
			// Don't close the connection here, let the error handling in readPump handle it
		}
		return
	}

	// Calculate current time
	elapsedTime := time.Now().Sub(state.VideoStartTime).Seconds()
	currentTime := elapsedTime
	if currentTime > state.CurrentVideo.Duration {
		currentTime = state.CurrentVideo.Duration
	}
	if currentTime < 0 {
		currentTime = 0
	}

	// Ensure we have a proper video URL
	videoURL := state.CurrentVideo.URL
	
	// If URL is not absolute and doesn't start with / for a local path, prepend / to make it a local path
	if !strings.HasPrefix(videoURL, "http") && !strings.HasPrefix(videoURL, "/") {
		videoURL = "/" + videoURL
	}

	// Create videoUpdate message with enhanced metadata
	response := WebSocketMessage{
		Type:        "videoUpdate",
		Channel:     channelNumber,
		URL:         videoURL,
		CurrentTime: currentTime,
		Duration:    state.CurrentVideo.Duration,
		VideoId:     state.CurrentVideo.ID,
		Title:       state.CurrentVideo.Title,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isActive {
		return
	}

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.conn.WriteJSON(response); err != nil {
		log.Printf("Error sending video update: %v", err)
		// Don't attempt to close the connection here, let the readPump handle it
	}
}

// sendVideoUpdates sends periodic updates about the current video's state.
func (c *WebSocketClient) sendVideoUpdates() {
	// Send updates more frequently for better synchronization
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Track consecutive errors to avoid excessive disconnects
	consecutiveErrors := 0
	maxConsecutiveErrors := 3

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			isActive := c.isActive
			c.mu.Unlock()

			if !isActive {
				return
			}

			// Wrap sendCurrentChannelState in a recover to prevent panics
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Recovered from panic in sendVideoUpdates: %v", r)
						consecutiveErrors++
					}
				}()
				
				// Try to send the update
				err := c.trySendCurrentState()
				
				if err != nil {
					consecutiveErrors++
					log.Printf("Error in sendVideoUpdates (%d consecutive): %v", consecutiveErrors, err)
				} else {
					// Reset error counter on success
					consecutiveErrors = 0
				}
				
				// If we've had too many consecutive errors, stop the client
				if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("Too many consecutive errors (%d), stopping updates", consecutiveErrors)
					c.mu.Lock()
					c.isActive = false
					c.mu.Unlock()
					return
				}
			}()
		}
	}
}

// trySendCurrentState is a wrapper for sendCurrentChannelState that returns an error
func (c *WebSocketClient) trySendCurrentState() error {
	// Make this channel for communication
	done := make(chan error, 1)
	
	// Run the send in a goroutine with a timeout
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in sendCurrentChannelState: %v", r)
			}
		}()
		
		c.sendCurrentChannelState()
		done <- nil
	}()
	
	// Wait with a timeout
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		return fmt.Errorf("sendCurrentChannelState timed out")
	}
} 