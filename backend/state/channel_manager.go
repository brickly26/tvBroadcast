package state

import (
	"fmt"
	"live-broadcast-backend/models"
	"live-broadcast-backend/services"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

/* ---------- interfaces already in your code ---------- */

type VideoProvider interface {
	DownloadVideo(s3Key string) (string, error)
	IsVideoDownloaded(s3Key string) bool
	DeleteVideo(s3Key string) error
}

type DBProvider interface {
	GetAllChannels() ([]*models.Channel, error)
	GetChannel(int) (*models.Channel, error)
	GetChannelVideos(int) ([]*models.Video, error)
}

/* ---------- ChannelManager ---------- */

type ChannelManager struct {
	mu                 sync.RWMutex
	channelStates      map[int]*models.ChannelState
	videos             map[string]*models.Video
	validS3Keys        []string
	videoProvider      VideoProvider
	dbProvider         DBProvider
	channelVideoMap    map[int][]*models.Video
	prefetchThreshold  float64
	nextVideoByChannel map[int]*models.Video
	broadcasters       map[int]*services.Broadcaster // NEW
	initialized        bool
}

/* ---------- constructor ---------- */

func NewChannelManager() *ChannelManager {
	cm := &ChannelManager{
		channelStates:      make(map[int]*models.ChannelState),
		videos:             make(map[string]*models.Video),
		validS3Keys:        []string{},
		channelVideoMap:    map[int][]*models.Video{},
		prefetchThreshold:  0.80,
		nextVideoByChannel: map[int]*models.Video{},
		broadcasters:       map[int]*services.Broadcaster{},
	}
	go cm.videoScheduler()
	return cm
}

/* ---------- public setters ---------- */

func (cm *ChannelManager) SetVideoProvider(p VideoProvider) { cm.mu.Lock(); cm.videoProvider = p; cm.mu.Unlock() }
func (cm *ChannelManager) SetDBProvider(p DBProvider)      { cm.mu.Lock(); cm.dbProvider = p; cm.mu.Unlock() }

/* ---------- helper: ensure a local copy and return its path ---------- */
func (cm *ChannelManager) getLocalPath(s3Key string) (string, error) {
	if cm.videoProvider.IsVideoDownloaded(s3Key) {
		// s3Key doubles as relative path inside ./videos
		return filepath.Join("./videos", s3Key), nil
	}
	return cm.videoProvider.DownloadVideo(s3Key) // returns local path
}

/* ---------- INITIALIZATION from S3 or DB (unchanged except broadcaster bootstrap) ---------- */

func (cm *ChannelManager) InitializeWithS3Content(channels []*models.Channel, videos map[string]*models.Video) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.videos = videos
	cm.validS3Keys = make([]string, 0, len(videos))
	for _, v := range videos {
		cm.validS3Keys = append(cm.validS3Keys, v.S3Key)
	}

	// Map videos → channel
	cm.channelVideoMap = map[int][]*models.Video{}
	for _, v := range videos {
		for _, tag := range v.Tags {
			if strings.HasPrefix(tag, "channel_") {
				var ch int
				if _, err := fmt.Sscanf(tag, "channel_%d", &ch); err == nil {
					cm.channelVideoMap[ch] = append(cm.channelVideoMap[ch], v)
				}
			}
		}
	}

	for _, ch := range channels {
		videoList := cm.channelVideoMap[ch.Number]
		var first *models.Video
		if len(videoList) > 0 {
			first = videoList[0]
		} else {
			for _, v := range videos { first = v; break } // any video
		}
		if first == nil { continue }

		cm.channelStates[ch.Number] = &models.ChannelState{
			Channel:        ch,
			CurrentVideo:   first,
			VideoStartTime: time.Now(),
		}

		// kick‑start broadcaster
		loc, err := cm.getLocalPath(first.S3Key)
		if err != nil {
			log.Printf("channel %d: cannot fetch first video: %v", ch.Number, err)
			continue
		}
		bc, err := services.NewBroadcaster(ch.Number, loc, 64*1024, 2_000_000/8)
		if err != nil {
			log.Printf("channel %d: broadcaster err: %v", ch.Number, err)
		} else {
			cm.broadcasters[ch.Number] = bc
		}

		// preload next if available
		if len(videoList) > 1 {
			cm.nextVideoByChannel[ch.Number] = videoList[1]
		}
	}
}

/* InitializeFromDatabase identical to your current version – omit here for brevity */

/* ---------- MAIN SCHEDULER (trimmed to core diff) ---------- */

func (cm *ChannelManager) videoScheduler() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cm.mu.Lock()
		if cm.videoProvider == nil { cm.mu.Unlock(); continue }

		now := time.Now()

		for chNum, st := range cm.channelStates {
			if st.CurrentVideo == nil { continue }

			elapsed := now.Sub(st.VideoStartTime).Seconds()
			if elapsed < st.CurrentVideo.Duration { continue } // not finished yet

			oldKey := st.CurrentVideo.S3Key
			next := cm.pickNextVideo(chNum, st.CurrentVideo)
			if next == nil { continue }

			// ensure local
			localPath, err := cm.getLocalPath(next.S3Key)
			if err != nil {
				log.Printf("channel %d: cannot fetch next video: %v", chNum, err)
				continue
			}

			// update state
			st.CurrentVideo = next
			st.VideoStartTime = now

			// broadcast switch
			if bc := cm.broadcasters[chNum]; bc != nil {
				_ = bc.SwitchSource(localPath)
			}

			// background delete previous file
			go cm.videoProvider.DeleteVideo(oldKey)
		}
		cm.mu.Unlock()
	}
}

/* pickNextVideo replicates your earlier logic but is factored out for clarity */
func (cm *ChannelManager) pickNextVideo(ch int, current *models.Video) *models.Video {
	list := cm.channelVideoMap[ch]
	if len(list) == 0 { // fallback random
		for _, v := range cm.videos { return v }
		return nil
	}
	for i, v := range list {
		if v.ID == current.ID {
			if i+1 < len(list) { return list[i+1] }
			return list[0]
		}
	}
	return list[0]
}

/* ---------- accessors used by handlers ---------- */

func (cm *ChannelManager) GetChannelState(num int) (*models.ChannelState, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	st, ok := cm.channelStates[num]
	if !ok { return nil, fmt.Errorf("channel %d not found", num) }
	cp := *st
	return &cp, nil
}

func (cm *ChannelManager) GetAllValidS3Keys() []string {
	cm.mu.RLock(); defer cm.mu.RUnlock()
	out := make([]string, len(cm.validS3Keys))
	copy(out, cm.validS3Keys)
	return out
}

// GetBroadcaster safely returns the broadcaster for a given channel number
func (cm *ChannelManager) GetBroadcaster(num int) *services.Broadcaster {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.broadcasters[num]
}

// GetAllChannelGuideInfo returns the guide information for all channels
func (cm *ChannelManager) GetAllChannelGuideInfo() interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	guideInfo := make(map[int]map[string]interface{})
	
	for chNum, state := range cm.channelStates {
		if state == nil || state.CurrentVideo == nil {
			continue
		}
		
		channelInfo := map[string]interface{}{
			"currentVideo": state.CurrentVideo,
			"startTime":    state.VideoStartTime,
		}
		
		// Add next video if available
		if next := cm.nextVideoByChannel[chNum]; next != nil {
			channelInfo["nextVideo"] = next
		}
		
		guideInfo[chNum] = channelInfo
	}
	
	return guideInfo
}

/* other existing methods (guide info etc.) stay unchanged */
