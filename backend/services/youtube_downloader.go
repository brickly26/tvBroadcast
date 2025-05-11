package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"live-broadcast-backend/database"
	"live-broadcast-backend/models"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// YouTubeDownloader handles downloading videos from YouTube and uploading to S3
type YouTubeDownloader struct {
	videoService *VideoService
	db           *database.DB
	tempDir      string
}

// NewYouTubeDownloader creates a new YouTube downloader
func NewYouTubeDownloader(videoService *VideoService, db *database.DB, tempDir string) (*YouTubeDownloader, error) {
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}

	return &YouTubeDownloader{
		videoService: videoService,
		db:           db,
		tempDir:      tempDir,
	}, nil
}

// VideoMetadata holds extracted metadata from a YouTube video
type VideoMetadata struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Duration    float64 `json:"duration"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// ProcessVideo queues a new video for download and processing
func (yd *YouTubeDownloader) ProcessVideo(youtubeURL string, channelID int, uploadedBy string) (string, error) {
	// Generate a new ID for the video
	videoID := uuid.New().String()
	
	// Use the video ID in the S3 key instead of sequential numbering
	videoS3Key := fmt.Sprintf("channel_%d/video_%s.mp4", channelID, videoID)
	
	// Create a new video record in pending state
	video := &models.AdminVideo{
		ID:          videoID,
		YoutubeURL:  youtubeURL,
		S3Key:       videoS3Key,
		ChannelID:   channelID,
		Title:       "Processing...",
		Status:      models.StatusPending,
		UploadedBy:  uploadedBy,
	}

	// Save to database
	if err := yd.db.SaveVideo(video); err != nil {
		return "", fmt.Errorf("failed to save video to database: %v", err)
	}

	// Start async download process
	go yd.downloadAndUpload(videoID, youtubeURL, channelID)

	return videoID, nil
}

// getVideoMetadata extracts metadata from a YouTube video
func (yd *YouTubeDownloader) getVideoMetadata(youtubeURL string) (*VideoMetadata, error) {
	// Use yt-dlp to extract video metadata in JSON format with user-agent and browser cookies to bypass bot detection
	cmd := exec.Command("yt-dlp", "-j", 
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36", 
		"--cookies-from-browser", "chrome", // Use cookies from Chrome browser
		"--no-check-certificate", // Skip certificate validation if needed
		youtubeURL)
	
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error retrieving YouTube metadata: %v", err)
		return nil, fmt.Errorf("failed to get video metadata: %v", err)
	}

	// Parse the JSON output
	var rawMetadata map[string]interface{}
	if err := json.Unmarshal(output, &rawMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse video metadata: %v", err)
	}

	// Extract the relevant metadata
	metadata := &VideoMetadata{
		Title: "Untitled Video",
		Duration: 0,
	}

	// Extract title
	if title, ok := rawMetadata["title"].(string); ok {
		metadata.Title = title
	}

	// Extract description
	if description, ok := rawMetadata["description"].(string); ok {
		metadata.Description = description
	}

	// Extract duration
	if duration, ok := rawMetadata["duration"].(float64); ok {
		metadata.Duration = duration
	}

	// Extract thumbnail URL (prefer high resolution)
	if thumbnails, ok := rawMetadata["thumbnails"].([]interface{}); ok && len(thumbnails) > 0 {
		// Try to get the highest quality thumbnail
		for i := len(thumbnails) - 1; i >= 0; i-- {
			if thumbnail, ok := thumbnails[i].(map[string]interface{}); ok {
				if url, ok := thumbnail["url"].(string); ok {
					metadata.ThumbnailURL = url
					break
				}
			}
		}
	} else if thumbnail, ok := rawMetadata["thumbnail"].(string); ok {
		// Fallback to single thumbnail if available
		metadata.ThumbnailURL = thumbnail
	}

	return metadata, nil
}

// downloadThumbnail downloads a thumbnail from a URL
func (yd *YouTubeDownloader) downloadThumbnail(thumbnailURL, destPath string) error {
	// Create the HTTP request
	resp, err := http.Get(thumbnailURL)
	if err != nil {
		return fmt.Errorf("failed to download thumbnail: %v", err)
	}
	defer resp.Body.Close()
	
	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download thumbnail, status: %d", resp.StatusCode)
	}
	
	// Create the destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail file: %v", err)
	}
	defer file.Close()
	
	// Copy the content
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save thumbnail: %v", err)
	}
	
	return nil
}

// downloadAndUpload handles the async download and upload process
func (yd *YouTubeDownloader) downloadAndUpload(videoID string, youtubeURL string, channelID int) {
	// Update status to downloading
	if err := yd.db.UpdateVideoStatus(videoID, models.StatusDownloading, ""); err != nil {
		log.Printf("Error updating video status to downloading: %v", err)
		return
	}
	
	// Fetch video metadata first
	metadata, err := yd.getVideoMetadata(youtubeURL)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get video metadata: %v", err)
		log.Printf("Error: %s", errorMsg)
		yd.db.UpdateVideoStatus(videoID, models.StatusFailed, errorMsg)
		return
	}

	// Create unique temporary files
	tempVideoFile := filepath.Join(yd.tempDir, fmt.Sprintf("%s_%d.mp4", videoID, time.Now().Unix()))
	tempThumbnailFile := filepath.Join(yd.tempDir, fmt.Sprintf("%s_%d_thumb.jpg", videoID, time.Now().Unix()))
	
	// Download thumbnail if available
	hasThumbnail := false
	if metadata.ThumbnailURL != "" {
		err := yd.downloadThumbnail(metadata.ThumbnailURL, tempThumbnailFile)
		if err != nil {
			log.Printf("Warning: Failed to download thumbnail: %v", err)
			// Continue with video download even if thumbnail fails
		} else {
			hasThumbnail = true
		}
	}
	
	// Download video using yt-dlp and user-agent, outputting to tempVideoFile
	// Force mp4 format with -f "bestvideo[ext=mp4]+bestaudio[ext=m4a]/mp4" and prevent yt-dlp from adding extension
	// Include cookies from browser to bypass YouTube bot detection
	cmd := exec.Command("yt-dlp", 
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36", 
		"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/mp4", 
		"--merge-output-format", "mp4", 
		"--cookies-from-browser", "chrome", 
		"--no-check-certificate", 
		"-o", tempVideoFile, 
		youtubeURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errorMsg := fmt.Sprintf("Download failed: %v - %s", err, string(output))
		log.Printf("Error downloading video: %s", errorMsg)
		yd.db.UpdateVideoStatus(videoID, models.StatusFailed, errorMsg)
		// Clean up temp files
		os.Remove(tempVideoFile)
		if hasThumbnail {
			os.Remove(tempThumbnailFile)
		}
		return
	}

	// Update status to processing
	if err := yd.db.UpdateVideoStatus(videoID, models.StatusProcessing, ""); err != nil {
		log.Printf("Error updating video status to processing: %v", err)
		return
	}

	// Define the S3 keys for video and thumbnail using the video ID
	videoS3Key := fmt.Sprintf("channel_%d/video_%s.mp4", channelID, videoID)
	thumbnailS3Key := fmt.Sprintf("thumbnails/thumbnail_%s.jpg", videoID)

	// Upload video to S3
	if err := yd.uploadToS3(tempVideoFile, videoS3Key, "video/mp4"); err != nil {
		errorMsg := fmt.Sprintf("Video upload failed: %v", err)
		log.Printf("Error uploading to S3: %s", errorMsg)
		yd.db.UpdateVideoStatus(videoID, models.StatusFailed, errorMsg)
		// Clean up temp files
		os.Remove(tempVideoFile)
		if hasThumbnail {
			os.Remove(tempThumbnailFile)
		}
		return
	}
	
	// Upload thumbnail to S3 if available
	thumbnailURL := ""
	if hasThumbnail {
		if err := yd.uploadToS3(tempThumbnailFile, thumbnailS3Key, "image/jpeg"); err != nil {
			log.Printf("Warning: Failed to upload thumbnail: %v", err)
			// Continue even if thumbnail upload fails
		} else {
			thumbnailURL = fmt.Sprintf("/thumbnails/thumbnail_%s.jpg", videoID)
		}
	}

	// Update database with success status and all metadata
	video, err := yd.db.GetVideoByID(videoID)
	if err != nil {
		log.Printf("Error retrieving video from database: %v", err)
	} else {
		video.Title = metadata.Title
		video.Description = metadata.Description
		video.Duration = metadata.Duration
		video.S3Key = videoS3Key
		video.Status = models.StatusCompleted
		video.ThumbnailURL = thumbnailURL
		
		// Save the updated video
		if err := yd.db.SaveVideo(video); err != nil {
			log.Printf("Error updating video with metadata: %v", err)
		}
		
		// Also update the status
		if err := yd.db.UpdateVideoStatus(videoID, models.StatusCompleted, ""); err != nil {
			log.Printf("Error updating video status to completed: %v", err)
		}
	}

	// Clean up temp files
	os.Remove(tempVideoFile)
	if hasThumbnail {
		os.Remove(tempThumbnailFile)
	}
	
	log.Printf("Successfully processed video %s for channel %d (duration: %.1f seconds)", videoID, channelID, metadata.Duration)
}

// uploadToS3 uploads a file to S3
func (yd *YouTubeDownloader) uploadToS3(filePath, s3Key string, contentType string) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Get file info for content length
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	// Upload to S3
	_, err = yd.videoService.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:        aws.String(yd.videoService.bucket),
		Key:           aws.String(s3Key),
		Body:          file,
		ContentLength: aws.Int64(fileInfo.Size()),
		ContentType:   aws.String(contentType),
	})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %v", err)
	}

	return nil
}
