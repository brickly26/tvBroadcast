package state

import (
	"fmt"
	"live-broadcast-backend/models"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// VideoProvider defines the interface for components that can provide video functionality
type VideoProvider interface {
	// DownloadVideo downloads a video from storage
	DownloadVideo(s3Key string) (string, error)
	// IsVideoDownloaded checks if a video is downloaded
	IsVideoDownloaded(s3Key string) bool
	// DeleteVideo removes a downloaded video
	DeleteVideo(s3Key string) error
}

// DBProvider defines the interface for database operations
type DBProvider interface {
	// GetAllChannels retrieves all channels from the database
	GetAllChannels() ([]*models.Channel, error)
	// GetChannel retrieves a channel by its number
	GetChannel(channelNumber int) (*models.Channel, error)
	// GetChannelVideos retrieves all videos for a specific channel
	GetChannelVideos(channelNumber int) ([]*models.Video, error)
}

// ChannelManager manages the state of all broadcast channels.
type ChannelManager struct {
	mu               sync.RWMutex
	channelStates    map[int]*models.ChannelState
	// Maps from video ID to video object
	videos          map[string]*models.Video
	// Store valid S3 keys to ensure only authorized videos can be streamed
	validS3Keys     []string
	// Reference to the video provider for JIT downloads
	videoProvider   VideoProvider
	// Reference to the database provider
	dbProvider      DBProvider
	// Map from channel number to videos for that channel
	channelVideoMap map[int][]*models.Video
	// Percentage of current video playback to start downloading next video
	prefetchThreshold float64
	// Next video indexed by channel number
	nextVideoByChannel map[int]*models.Video
	// Flag to track if we've initialized from the database
	initialized     bool
}

// NewChannelManager creates and initializes a new ChannelManager.
func NewChannelManager() *ChannelManager {
	manager := &ChannelManager{
		channelStates: make(map[int]*models.ChannelState),
		videos:       make(map[string]*models.Video),
		validS3Keys:  make([]string, 0),
		channelVideoMap: make(map[int][]*models.Video),
		prefetchThreshold: 0.8, // Start downloading next video when current is 80% complete
		nextVideoByChannel: make(map[int]*models.Video),
		initialized: false,
	}
	// Channels will be initialized later when S3 videos are ready
	go manager.videoScheduler() // Start background task to switch videos
	return manager
}

// SetVideoProvider sets the video provider for the channel manager
func (cm *ChannelManager) SetVideoProvider(provider VideoProvider) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.videoProvider = provider
}

// SetDBProvider sets the database provider for the channel manager
func (cm *ChannelManager) SetDBProvider(provider DBProvider) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.dbProvider = provider
}

// InitializeWithS3Content sets up channels and videos from S3 content
func (cm *ChannelManager) InitializeWithS3Content(channels []*models.Channel, videos map[string]*models.Video) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Store the videos
	cm.videos = videos

	// Extract S3 keys for authorized access
	cm.validS3Keys = make([]string, 0, len(videos))
	for _, video := range videos {
		cm.validS3Keys = append(cm.validS3Keys, video.S3Key)
	}

	// Create channel states with assigned videos
	availableVideos := make([]*models.Video, 0, len(cm.videos))
	for _, v := range cm.videos {
		availableVideos = append(availableVideos, v)
	}

	if len(availableVideos) == 0 {
		fmt.Println("Warning: No videos available from S3 to initialize channels.")
		return // Cannot initialize without videos
	}

	// Organize videos by channel
	cm.channelVideoMap = make(map[int][]*models.Video)
	for _, video := range availableVideos {
		// Extract channel number from video tags
		for _, tag := range video.Tags {
			if strings.HasPrefix(tag, "channel_") {
				var channelNum int
				if _, err := fmt.Sscanf(tag, "channel_%d", &channelNum); err == nil {
					// Add video to this channel's list
					cm.channelVideoMap[channelNum] = append(cm.channelVideoMap[channelNum], video)
					break
				}
			}
		}
	}

	// Initialize each channel with its first video
	for _, channel := range channels {
		channelNum := channel.Number
		
		// Get videos for this channel or use a random selection
		channelVideos, ok := cm.channelVideoMap[channelNum]
		if !ok || len(channelVideos) == 0 {
			// If no videos specifically for this channel, use any available video
			if len(availableVideos) > 0 {
				randIndex := rand.Intn(len(availableVideos))
				startVideo := availableVideos[randIndex]
				cm.channelStates[channelNum] = &models.ChannelState{
					Channel:        channel,
					CurrentVideo:   startVideo,
					VideoStartTime: time.Now(),
				}
				fmt.Printf("Initialized Channel %d (%s) with random video: %s\n", channelNum, channel.Name, startVideo.Title)
			} else {
				fmt.Printf("Warning: No videos available for Channel %d\n", channelNum)
				continue
			}
		} else {
			// Use the first video for this channel
			startVideo := channelVideos[0]
			cm.channelStates[channelNum] = &models.ChannelState{
				Channel:        channel,
				CurrentVideo:   startVideo,
				VideoStartTime: time.Now(),
			}
			fmt.Printf("Initialized Channel %d (%s) with first video: %s\n", channelNum, channel.Name, startVideo.Title)
			
			// Set up the next video if there are more than one
			if len(channelVideos) > 1 {
				cm.nextVideoByChannel[channelNum] = channelVideos[1]
			}
		}
	}
}

// InitializeFromDatabase initializes the channel manager from the database
func (cm *ChannelManager) InitializeFromDatabase() error {
	channels, err := cm.dbProvider.GetAllChannels()
	if err != nil {
		return err
	}

	for _, channel := range channels {
		videos, err := cm.dbProvider.GetChannelVideos(channel.Number)
		if err != nil {
			return err
		}

		// Create channel state with assigned videos
		channelVideos := make([]*models.Video, 0, len(videos))
		for _, v := range videos {
			channelVideos = append(channelVideos, v)
		}

		if len(channelVideos) == 0 {
			log.Printf("Warning: No videos available from database for Channel %d\n", channel.Number)
			continue
		}

		// Initialize each channel with its first video
		startVideo := channelVideos[0]
		cm.channelStates[channel.Number] = &models.ChannelState{
			Channel:        channel,
			CurrentVideo:   startVideo,
			VideoStartTime: time.Now(),
		}
		log.Printf("Initialized Channel %d (%s) with first video: %s\n", channel.Number, channel.Name, startVideo.Title)

		// Set up the next video if there are more than one
		if len(channelVideos) > 1 {
			cm.nextVideoByChannel[channel.Number] = channelVideos[1]
		}
	}

	cm.initialized = true
	return nil
}

// createChannelStateFromDB creates a channel state from the database
func (cm *ChannelManager) createChannelStateFromDB(channelNumber int) error {
	channel, err := cm.dbProvider.GetChannel(channelNumber)
	if err != nil {
		return err
	}

	videos, err := cm.dbProvider.GetChannelVideos(channelNumber)
	if err != nil {
		return err
	}

	// Create channel state with assigned videos
	channelVideos := make([]*models.Video, 0, len(videos))
	for _, v := range videos {
		channelVideos = append(channelVideos, v)
	}

	if len(channelVideos) == 0 {
		log.Printf("Warning: No videos available from database for Channel %d\n", channelNumber)
		return fmt.Errorf("no videos available for channel %d", channelNumber)
	}

	// Initialize each channel with its first video
	startVideo := channelVideos[0]
	cm.channelStates[channelNumber] = &models.ChannelState{
		Channel:        channel,
		CurrentVideo:   startVideo,
		VideoStartTime: time.Now(),
	}
	log.Printf("Initialized Channel %d (%s) with first video: %s\n", channelNumber, channel.Name, startVideo.Title)

	// Set up the next video if there are more than one
	if len(channelVideos) > 1 {
		cm.nextVideoByChannel[channelNumber] = channelVideos[1]
	}

	return nil
}

// GetChannelState retrieves the current state for a specific channel number.
func (cm *ChannelManager) GetChannelState(channelNumber int) (*models.ChannelState, error) {
	// First try to initialize from DB if we haven't already
	if !cm.initialized {
		if err := cm.InitializeFromDatabase(); err != nil {
			log.Printf("Error initializing from database: %v", err)
		}
	}

	cm.mu.RLock()
	state, exists := cm.channelStates[channelNumber]
	cm.mu.RUnlock()

	if !exists {
		// If state doesn't exist in memory, try to create it from the database
		if err := cm.createChannelStateFromDB(channelNumber); err != nil {
			return nil, fmt.Errorf("channel %d not found: %v", channelNumber, err)
		}
		
		// Try again after creating from DB
		cm.mu.RLock()
		state, exists = cm.channelStates[channelNumber]
		cm.mu.RUnlock()
		
		if !exists {
			return nil, fmt.Errorf("channel %d not found after DB lookup", channelNumber)
		}
	}

	// Create a copy to avoid race conditions if the caller modifies it
	// (though the current design makes this less likely)
	stateCopy := *state
	stateCopy.Channel = state.Channel         // Ensure pointers are copied correctly if needed
	stateCopy.CurrentVideo = state.CurrentVideo

	return &stateCopy, nil
}

// GetAllChannelGuideInfo retrieves basic info for all channels for the guide.
func (cm *ChannelManager) GetAllChannelGuideInfo() []models.ChannelGuideInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	guideInfoList := make([]models.ChannelGuideInfo, 0, len(cm.channelStates))
	for num, state := range cm.channelStates {
		info := models.ChannelGuideInfo{
			Number: num,
			Name:   state.Channel.Name,
			Theme:  state.Channel.Theme,
		}
		if state.CurrentVideo != nil {
			info.CurrentVideoTitle = state.CurrentVideo.Title
		}
		// Add NextVideo logic here if implemented
		guideInfoList = append(guideInfoList, info)
	}
	return guideInfoList
}

// videoScheduler runs in the background to switch videos when they end.
func (cm *ChannelManager) videoScheduler() {
	// Simple ticker to check every few seconds
	// A more sophisticated approach might use exact video end times
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Keep checking if videos have been initialized
	for range ticker.C {
		cm.mu.Lock()
		
		// Skip if we don't have a video provider set up yet
		if cm.videoProvider == nil {
			cm.mu.Unlock()
			continue
		}
		
		// Check if we have videos to work with
		availableVideos := make([]*models.Video, 0, len(cm.videos))
		for _, v := range cm.videos {
			availableVideos = append(availableVideos, v)
		}
		
		if len(availableVideos) == 0 {
			cm.mu.Unlock()
			continue // No videos yet, keep waiting
		}
		
		now := time.Now()
		
		// Process each channel
		for channelNum, state := range cm.channelStates {
			if state.CurrentVideo == nil {
				// If a channel somehow has no video, assign one from its channel list
				channelVideos, ok := cm.channelVideoMap[channelNum]
				
				if ok && len(channelVideos) > 0 {
					// Use first video from channel's list
					state.CurrentVideo = channelVideos[0]
					
					// Ensure it's downloaded
					if !cm.videoProvider.IsVideoDownloaded(state.CurrentVideo.S3Key) {
						_, err := cm.videoProvider.DownloadVideo(state.CurrentVideo.S3Key)
						if err != nil {
							fmt.Printf("Error downloading video for channel %d: %v\n", channelNum, err)
						}
					}
					
					// Set up next video if available
					if len(channelVideos) > 1 {
						cm.nextVideoByChannel[channelNum] = channelVideos[1]
					}
				} else {
					// Fallback to any available video
					randIndex := rand.Intn(len(availableVideos))
					state.CurrentVideo = availableVideos[randIndex]
					
					// Ensure it's downloaded
					if !cm.videoProvider.IsVideoDownloaded(state.CurrentVideo.S3Key) {
						_, err := cm.videoProvider.DownloadVideo(state.CurrentVideo.S3Key)
						if err != nil {
							fmt.Printf("Error downloading video for channel %d: %v\n", channelNum, err)
						}
					}
				}
				
				state.VideoStartTime = now
				fmt.Printf("Channel %d: Assigned video %s\n", channelNum, state.CurrentVideo.Title)
				continue
			}

			// Calculate how far we are through the current video
			elapsedTime := now.Sub(state.VideoStartTime).Seconds()
			currentVideoProgress := elapsedTime / state.CurrentVideo.Duration
			
			// If we're past the prefetch threshold and don't have the next video ready,
			// start downloading the next video
			if currentVideoProgress >= cm.prefetchThreshold {
				nextVideo, hasNext := cm.nextVideoByChannel[channelNum]
				
				if hasNext && !cm.videoProvider.IsVideoDownloaded(nextVideo.S3Key) {
					// Download the next video in the background
					go func(s3Key string) {
						_, err := cm.videoProvider.DownloadVideo(s3Key)
						if err != nil {
							fmt.Printf("Error pre-downloading next video for channel %d: %v\n", channelNum, err)
						} else {
							fmt.Printf("Successfully pre-downloaded next video for channel %d\n", channelNum)
						}
					}(nextVideo.S3Key)
				}
			}
			
			// Check if current video is complete and needs to be switched
			if elapsedTime >= state.CurrentVideo.Duration {
				// Keep track of the video that just finished to delete it
				finishedVideoS3Key := state.CurrentVideo.S3Key
				oldVideoTitle := state.CurrentVideo.Title

				// Choose the next video
				var nextVideo *models.Video
				channelVideos, ok := cm.channelVideoMap[channelNum]
				
				if !ok || len(channelVideos) == 0 {
					// No specific videos for this channel, use random video
					randIndex := rand.Intn(len(availableVideos))
					nextVideo = availableVideos[randIndex]
				} else {
					// Find the next video in the channel's video list, or loop back to start
					currentIndex := -1
					for i, v := range channelVideos {
						if v.ID == state.CurrentVideo.ID {
							currentIndex = i
							break
						}
					}
					
					// Get next video or loop back to first
					if currentIndex >= 0 && currentIndex+1 < len(channelVideos) {
						nextVideo = channelVideos[currentIndex+1]
					} else {
						// Loop back to first video
						nextVideo = channelVideos[0]
					}
					
					// Set up the next video after this one
					if len(channelVideos) > 1 {
						nextIndex := -1
						for i, v := range channelVideos {
							if v.ID == nextVideo.ID {
								nextIndex = i
								break
							}
						}
						
						// Set up the one after this
						if nextIndex >= 0 {
							if nextIndex+1 < len(channelVideos) {
								cm.nextVideoByChannel[channelNum] = channelVideos[nextIndex+1]
							} else {
								// Loop back to first
								cm.nextVideoByChannel[channelNum] = channelVideos[0]
							}
						}
					}
				}
				
				// Make sure the next video is downloaded
				if !cm.videoProvider.IsVideoDownloaded(nextVideo.S3Key) {
					_, err := cm.videoProvider.DownloadVideo(nextVideo.S3Key)
					if err != nil {
						fmt.Printf("Error downloading next video for channel %d: %v\n", channelNum, err)
					}
				}
				
				// Update the channel state
				state.CurrentVideo = nextVideo
				state.VideoStartTime = now
				fmt.Printf("Channel %d: Switched from '%s' to '%s'\n", channelNum, oldVideoTitle, nextVideo.Title)
				
				// Delete the old video in the background
				go func(s3Key string) {
					// Small delay to ensure the old video is not being accessed
					time.Sleep(2 * time.Second)
					err := cm.videoProvider.DeleteVideo(s3Key)
					if err != nil {
						fmt.Printf("Error deleting old video %s: %v\n", s3Key, err)
					} else {
						fmt.Printf("Successfully deleted old video: %s\n", s3Key)
					}
				}(finishedVideoS3Key)
			}
		}
		cm.mu.Unlock()
	}
}

// GetAllValidS3Keys returns a list of all S3 keys that are currently valid for streaming
func (cm *ChannelManager) GetAllValidS3Keys() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// Return a copy to prevent modification
	keys := make([]string, len(cm.validS3Keys))
	copy(keys, cm.validS3Keys)
	return keys
} 