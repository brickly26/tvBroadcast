package services

import (
	"live-broadcast-backend/models"
	"log"
	"time"
)

// ChannelManager represents any component that can manage video channels
type ChannelManager interface {
	// We don't define specific methods here to avoid import cycles
	// We'll use type assertions at runtime instead
}

// SyncService handles periodic synchronization with S3 to update available videos
type SyncService struct {
	s3Manager      *S3Manager
	channelManager ChannelManager
	syncInterval   time.Duration
}

// NewSyncService creates a new service to periodically sync S3 content
func NewSyncService(s3Manager *S3Manager, channelManager ChannelManager, syncIntervalMinutes int) *SyncService {
	if syncIntervalMinutes <= 0 {
		syncIntervalMinutes = 15 // Default to 15 minutes if invalid
	}
	
	return &SyncService{
		s3Manager:      s3Manager,
		channelManager: channelManager,
		syncInterval:   time.Duration(syncIntervalMinutes) * time.Minute,
	}
}

// Start begins the periodic synchronization process in the background
func (ss *SyncService) Start() {
	log.Printf("Starting S3 sync service with interval of %v", ss.syncInterval)
	
	// Immediately do an initial sync
	go ss.syncVideos()
	
	// Start ticker for regular syncs
	ticker := time.NewTicker(ss.syncInterval)
	go func() {
		for range ticker.C {
			log.Printf("Running scheduled S3 content sync")
			ss.syncVideos()
		}
	}()
}

// syncVideos updates the available videos from S3
func (ss *SyncService) syncVideos() {
	log.Println("Synchronizing S3 video content...")
	
	// Set the S3Manager as the VideoProvider for the ChannelManager using type assertion
	// This must be done before initializing content to enable JIT downloads
	if setter, ok := ss.channelManager.(interface{ SetVideoProvider(interface{}) }); ok {
		setter.SetVideoProvider(ss.s3Manager)
		log.Println("Connected S3Manager as video provider for ChannelManager")
	}
	
	// Get updated channels and videos from S3
	channels, videos, err := ss.s3Manager.GetChannelsAndVideos()
	if err != nil {
		log.Printf("Error syncing S3 content: %v", err)
		return
	}
	
	// Update the channel manager with the new content using reflection
	// We use type assertion to call the InitializeWithS3Content method without
	// needing to import the exact type
	if initializer, ok := ss.channelManager.(interface{ InitializeWithS3Content(channels []*models.Channel, videos map[string]*models.Video) }); ok {
		initializer.InitializeWithS3Content(channels, videos)
		log.Printf("Successfully synchronized %d channels with %d videos from S3 (JIT download enabled)", 
			len(channels), len(videos))
	} else {
		log.Printf("Error: ChannelManager does not implement required interface for initialization")
	}
}
