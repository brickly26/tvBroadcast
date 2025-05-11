package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type VideoService struct {
	s3Client *s3.Client
	bucket   string
}

func NewVideoService(bucket string) (*VideoService, error) {
	// Get region from environment or default to us-east-1
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1" // Default region if not specified
	}

	log.Printf("Initializing S3 client with region: %s for bucket: %s", region, bucket)

	// Load the AWS configuration with explicit region
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	// Create an S3 client
	client := s3.NewFromConfig(cfg)

	return &VideoService{
		s3Client: client,
		bucket:   bucket,
	}, nil
}

// GetVideoURL generates a pre-signed URL for a video in the S3 bucket
func (vs *VideoService) GetVideoURL(channelNumber int) (string, error) {
	// Construct the video key based on channel number
	videoKey := fmt.Sprintf("channel_%d/video.mp4", channelNumber)

	// Create a pre-signed URL that expires in 1 hour
	presignClient := s3.NewPresignClient(vs.s3Client)
	presignParams := &s3.GetObjectInput{
		Bucket: &vs.bucket,
		Key:    &videoKey,
	}

	presignedURL, err := presignClient.PresignGetObject(context.TODO(), presignParams, func(po *s3.PresignOptions) {
		po.Expires = time.Hour
	})
	if err != nil {
		log.Printf("Error generating pre-signed URL for channel %d: %v", channelNumber, err)
		return "", err
	}

	return presignedURL.URL, nil
}

// ValidateVideoExists checks if a video exists for a given channel
func (vs *VideoService) ValidateVideoExists(channelNumber int) bool {
	videoKey := fmt.Sprintf("channel_%d/video.mp4", channelNumber)

	_, err := vs.s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: &vs.bucket,
		Key:    &videoKey,
	})

	return err == nil
}

// DeleteVideo deletes a video and its thumbnail from the S3 bucket
func (vs *VideoService) DeleteVideo(channelID int, videoID string, videoPosition int) error {
	// Build the S3 keys for both the video and its thumbnail
	videoKey := fmt.Sprintf("videos/%s.mp4", videoID)
	thumbnailKey := fmt.Sprintf("thumbnails/channel_%d_video%d.jpg", channelID, videoPosition)

	log.Printf("Deleting video from S3: %s, thumbnail: %s", videoKey, thumbnailKey)

	// Delete the video from S3
	_, err := vs.s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: &vs.bucket,
		Key:    &videoKey,
	})
	if err != nil {
		log.Printf("Error deleting video from S3: %v", err)
		// Continue even if there's an error, try to delete the thumbnail anyway
	}

	// Delete the thumbnail from S3
	_, err = vs.s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: &vs.bucket,
		Key:    &thumbnailKey,
	})
	if err != nil {
		log.Printf("Error deleting thumbnail from S3: %v", err)
		// Continue even if there's an error, we've already tried our best
	}

	return nil // Return nil even if there were errors, since we want to continue with database cleanup
}

// GetThumbnailURL generates a pre-signed URL for a thumbnail in the S3 bucket
func (vs *VideoService) GetThumbnailURL(thumbnailKey string) (string, error) {
	// Create a pre-signed URL that expires in 10 minutes
	presignClient := s3.NewPresignClient(vs.s3Client)
	presignParams := &s3.GetObjectInput{
		Bucket: &vs.bucket,
		Key:    &thumbnailKey,
	}

	presignedURL, err := presignClient.PresignGetObject(context.TODO(), presignParams, func(po *s3.PresignOptions) {
		po.Expires = 10 * time.Minute
	})
	if err != nil {
		log.Printf("Error generating pre-signed URL for thumbnail %s: %v", thumbnailKey, err)
		return "", err
	}

	return presignedURL.URL, nil
}
