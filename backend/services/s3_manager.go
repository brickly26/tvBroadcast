package services

import (
	"context"
	"fmt"
	"io"
	"live-broadcast-backend/models"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Manager handles interaction with S3 for video management
type S3Manager struct {
	s3Client  *s3.Client
	bucket    string
	videoDir  string
	baseURL   string
	mutex     sync.Mutex
	downloads map[string]bool // Track which S3Keys are downloaded
}

// NewS3Manager creates a new S3 manager for video operations
func NewS3Manager(videoService *VideoService, videoDir string) (*S3Manager, error) {
	// Create videos directory if it doesn't exist
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create videos directory: %v", err)
	}

	return &S3Manager{
		s3Client: videoService.s3Client,
		bucket:   videoService.bucket,
		videoDir: videoDir,
		baseURL:  "/videos", // Local URL path for videos
		downloads: make(map[string]bool),
	}, nil
}

// ListChannelFolders lists all channel folders in the S3 bucket
func (sm *S3Manager) ListChannelFolders() ([]string, error) {
	// Get AWS region from environment
	awsRegion := os.Getenv("AWS_REGION")
	log.Printf("Using AWS region: %s for bucket: %s", awsRegion, sm.bucket)

	// List objects with delimiter to get "directories"
	resp, err := sm.s3Client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket:    &sm.bucket,
		Delimiter: aws.String("/"),
	})

	if err != nil {
		log.Printf("Error listing S3 folders: %v", err)
		return nil, fmt.Errorf("failed to list S3 folders: %v", err)
	}

	var folders []string
	for _, prefix := range resp.CommonPrefixes {
		if prefix.Prefix != nil {
			// Remove trailing slash
			folderName := strings.TrimSuffix(*prefix.Prefix, "/")
			if strings.HasPrefix(folderName, "channel_") {
				folders = append(folders, folderName)
			}
		}
	}

	return folders, nil
}

// ListVideosInFolder lists all videos in a specific S3 folder
func (sm *S3Manager) ListVideosInFolder(folder string) ([]string, error) {
	// Ensure folder name ends with a slash for prefix search
	if !strings.HasSuffix(folder, "/") {
		folder = folder + "/"
	}

	resp, err := sm.s3Client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: &sm.bucket,
		Prefix: &folder,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list videos in folder %s: %v", folder, err)
	}

	var videos []string
	for _, object := range resp.Contents {
		if object.Key != nil && *object.Key != folder {
			videos = append(videos, *object.Key)
		}
	}

	return videos, nil
}

// DownloadVideo downloads a video from S3 to the local file system
func (m *S3Manager) DownloadVideo(s3Key string) (string, error) {
	tmpPath, err := m.rawDownload(s3Key) // <- your existing internal downloader
	if err != nil {
		return "", err
	}

	/* validate first 3 s of media to catch corrupt transfers */
	if err := validateMP4(tmpPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("downloaded mp4 is corrupt: %w", err)
	}
	return tmpPath, nil
}

// CreateVideoObject creates a Video object from an S3 video
// IsVideoDownloaded checks if a video has been downloaded
func (sm *S3Manager) IsVideoDownloaded(s3Key string) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	_, exists := sm.downloads[s3Key]
	return exists
}

// DeleteVideo removes a downloaded video from the file system
func (sm *S3Manager) DeleteVideo(s3Key string) error {
	// Get the local path
	localPath := filepath.Join(sm.videoDir, s3Key)
	
	// Check if the file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		// File doesn't exist, nothing to delete
		return nil
	}
	
	// Delete the file
	err := os.Remove(localPath)
	if err != nil {
		return fmt.Errorf("failed to delete video %s: %v", s3Key, err)
	}
	
	// Update tracking
	sm.mutex.Lock()
	delete(sm.downloads, s3Key)
	sm.mutex.Unlock()
	
	log.Printf("Successfully deleted video: %s", s3Key)
	return nil
}

func (sm *S3Manager) CreateVideoObject(s3Key string, localPath string, channelNum int) *models.Video {
	// Extract video title from S3 key
	videoName := filepath.Base(s3Key)
	videoTitle := strings.TrimSuffix(videoName, filepath.Ext(videoName))

	// Create video URL (local path)
	videoURL := fmt.Sprintf("%s/%s", sm.baseURL, s3Key) 

	// Try to determine video duration (in production, you'd use a video processing library)
	// For now we'll set different durations based on filename to simulate varying lengths
	duration := 300.0 // Default 5 minutes
	
	// Extract numbers from filename to vary the durations
	var num int
	if _, err := fmt.Sscanf(videoTitle, "video%d", &num); err == nil && num > 0 {
		// Generate durations between 2-8 minutes based on video number
		duration = float64(120 + (num * 60))
		if duration > 480 { // Cap at 8 minutes
			duration = 480
		}
	}

	// Create Video object
	video := &models.Video{
		ID:          fmt.Sprintf("s3-%s-%d", videoTitle, channelNum),
		Title:       fmt.Sprintf("Channel %d - %s", channelNum, videoTitle),
		Description: fmt.Sprintf("Video from S3: %s", s3Key),
		Duration:    duration,
		S3Key:       s3Key,
		Tags:        []string{"s3", fmt.Sprintf("channel_%d", channelNum)},
		CreatedAt:   time.Now(),
		URL:         videoURL,
	}

	return video
}

// GetChannelsAndVideos retrieves all channels and their metadata from S3,
// but only downloads the first video for each channel
func (sm *S3Manager) GetChannelsAndVideos() ([]*models.Channel, map[string]*models.Video, error) {
	// List all channel folders
	folders, err := sm.ListChannelFolders()
	if err != nil {
		return nil, nil, err
	}

	channels := make([]*models.Channel, 0, len(folders))
	videos := make(map[string]*models.Video)
	videosByChannel := make(map[int][]string)

	// First pass: collect information about all videos without downloading
	for _, folder := range folders {
		// Extract channel number from folder name
		var channelNum int
		fmt.Sscanf(folder, "channel_%d", &channelNum)
		if channelNum < 1 {
			log.Printf("Invalid channel folder name: %s", folder)
			continue
		}

		// Create channel
		channel := &models.Channel{
			Number:      channelNum,
			Name:        fmt.Sprintf("Channel %d", channelNum),
			Description: fmt.Sprintf("Channel %d from S3", channelNum),
			Theme:       getThemeForChannel(channelNum),
		}
		channels = append(channels, channel)

		// List videos in folder
		videoKeys, err := sm.ListVideosInFolder(folder)
		if err != nil {
			log.Printf("Error listing videos in folder %s: %v", folder, err)
			continue
		}

		// Store video keys for this channel
		videosByChannel[channelNum] = videoKeys

		// Create video objects without downloading
		for _, videoKey := range videoKeys {
			// The local path where video would be stored if/when downloaded
			video := sm.CreateVideoObject(videoKey, filepath.Join(sm.videoDir, videoKey), channelNum)
			videos[video.ID] = video
		}

		// If no videos were found for this channel, log a warning
		if len(videoKeys) == 0 {
			log.Printf("Warning: No videos found for channel %d", channelNum)
		}
	}

	// Second pass: download only the first video for each channel
	for channelNum, videoKeys := range videosByChannel {
		if len(videoKeys) > 0 {
			// Download only the first video
			firstVideoKey := videoKeys[0]
			_, err := sm.DownloadVideo(firstVideoKey)
			if err != nil {
				log.Printf("Error downloading first video %s for channel %d: %v", firstVideoKey, channelNum, err)
				continue
			}
			log.Printf("Downloaded first video %s for channel %d", firstVideoKey, channelNum)
		}
	}

	return channels, videos, nil
}

// getThemeForChannel returns an appropriate theme for a given channel number
func getThemeForChannel(channelNum int) string {
	themes := []string{"news", "sports", "nature", "technology", "entertainment"}
	if channelNum > 0 && channelNum <= len(themes) {
		return themes[channelNum-1]
	}
	return "general"
}

func validateMP4(path string) error {
	cmd := exec.Command("ffprobe", "-v", "error", "-read_intervals", "%+#3", "-i", path)
	return cmd.Run()
}

func (sm *S3Manager) rawDownload(s3Key string) (string, error) {
	// Check cache first
	sm.mutex.Lock()
	if sm.downloads[s3Key] {
		sm.mutex.Unlock()
		return filepath.Join(sm.videoDir, s3Key), nil
	}
	sm.mutex.Unlock()

	// Get the object from S3
	resp, err := sm.s3Client.GetObject(
		context.Background(),
		&s3.GetObjectInput{
			Bucket: aws.String(sm.bucket),
			Key:    aws.String(s3Key),
		},
	)
	if err != nil {
		return "", fmt.Errorf("s3 getObject %s: %w", s3Key, err)
	}
	defer resp.Body.Close()

	// Make sure parent folders exist
	localPath := filepath.Join(sm.videoDir, s3Key)
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(localPath), err)
	}

	// Stream copy to disk
	out, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", localPath, err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(localPath)
		return "", fmt.Errorf("copy %s: %w", s3Key, err)
	}
	out.Close()

	// mark as downloaded
	sm.mutex.Lock()
	sm.downloads[s3Key] = true
	sm.mutex.Unlock()

	return localPath, nil
}