package models

import (
	"time"
)

// Video represents a video that can be played on a channel
type Video struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Duration     float64   `json:"duration"` // Duration in seconds
	S3Key        string    `json:"s3Key"`    // S3 key for the video file - Placeholder for now
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"createdAt"`
	URL          string    `json:"url"`      // Actual video URL (e.g., pre-signed S3 URL or placeholder)
	ThumbnailURL string    `json:"thumbnailUrl,omitempty"` // URL for the video thumbnail
}

// Channel represents a broadcast channel
type Channel struct {
	Number      int    `json:"number"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Theme       string `json:"theme"`
}

// ChannelState represents the current state of a channel
type ChannelState struct {
	Channel     *Channel  `json:"channel"`
	CurrentVideo *Video    `json:"currentVideo"`
	// NextVideo   *Video    `json:"nextVideo"` // We can add scheduling later
	VideoStartTime time.Time `json:"videoStartTime"` // When the current video started playing globally
	// Playlist   []*Video // Optional: Could hold a list of videos for the channel
}

// ChannelGuideInfo represents information shown in the channel guide
type ChannelGuideInfo struct {
	Number       int    `json:"number"`
	Name         string `json:"name"`
	Theme        string `json:"theme"`
	CurrentVideoTitle string `json:"currentVideoTitle"`
	// NextVideoTitle string `json:"nextVideoTitle"` // Add later if needed
}

// PredefinedChannels returns the list of predefined channels
func PredefinedChannels() []*Channel {
	return []*Channel{
		{
			Number:      1,
			Name:        "News Channel",
			Description: "Latest news from around the world",
			Theme:       "news",
		},
		{
			Number:      2,
			Name:        "Sports Zone",
			Description: "All things sports and athletics",
			Theme:       "sports",
		},
		{
			Number:      3,
			Name:        "Nature & Wildlife",
			Description: "Explore the natural world",
			Theme:       "nature",
		},
		{
			Number:      4,
			Name:        "Tech Today",
			Description: "Latest in technology and innovation",
			Theme:       "technology",
		},
		{
			Number:      5,
			Name:        "Entertainment",
			Description: "Movies, music, and entertainment",
			Theme:       "entertainment",
		},
	}
}

// SampleVideos provides some sample video data for testing
// In a real app, this would come from a database and S3
func SampleVideos() map[string]*Video {
	return map[string]*Video{
		"vid001": {
			ID:           "vid001",
			Title:        "Global Headlines",
			Description:  "Breaking news updates.",
			Duration:     180, // 3 minutes
			S3Key:        "news/global_headlines.mp4",
			URL:          "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4", // Placeholder URL
			Tags:         []string{"news", "world"},
			CreatedAt:    time.Now(),
			ThumbnailURL: "/thumbnails/thumbnail_vid001.jpg",
		},
		"vid002": {
			ID:           "vid002",
			Title:        "Market Watch",
			Description:  "Financial news and analysis.",
			Duration:     240, // 4 minutes
			S3Key:        "news/market_watch.mp4",
			URL:          "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ElephantsDream.mp4", // Placeholder URL
			Tags:         []string{"news", "finance"},
			CreatedAt:    time.Now(),
			ThumbnailURL: "/thumbnails/thumbnail_vid002.jpg",
		},
		"vid003": {
			ID:           "vid003",
			Title:        "Championship Highlights",
			Description:  "Top plays from the finals.",
			Duration:     300, // 5 minutes
			S3Key:        "sports/championship.mp4",
			URL:          "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4", // Placeholder URL
			Tags:         []string{"sports", "basketball"},
			CreatedAt:    time.Now(),
			ThumbnailURL: "/thumbnails/thumbnail_vid003.jpg",
		},
        "vid004": {
			ID:           "vid004",
			Title:        "Training Day",
			Description:  "Behind the scenes with athletes.",
			Duration:     210, // 3.5 minutes
			S3Key:        "sports/training_day.mp4",
			URL:          "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerEscapes.mp4", // Placeholder URL
			Tags:         []string{"sports", "training"},
			CreatedAt:    time.Now(),
			ThumbnailURL: "/thumbnails/thumbnail_vid004.jpg",
		},
        "vid005": {
			ID:           "vid005",
			Title:        "Ocean Wonders",
			Description:  "Exploring marine life.",
			Duration:     360, // 6 minutes
			S3Key:        "nature/ocean_wonders.mp4",
			URL:          "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerFun.mp4", // Placeholder URL
			Tags:         []string{"nature", "ocean"},
			CreatedAt:    time.Now(),
			ThumbnailURL: "/thumbnails/thumbnail_vid005.jpg",
		},
         "vid006": {
			ID:           "vid006",
			Title:        "Forest Giants",
			Description:  "Majestic trees of the world.",
			Duration:     270, // 4.5 minutes
			S3Key:        "nature/forest_giants.mp4",
			URL:          "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerJoyrides.mp4", // Placeholder URL
			Tags:         []string{"nature", "forest"},
			CreatedAt:    time.Now(),
			ThumbnailURL: "/thumbnails/thumbnail_vid006.jpg",
		},
	}
} 