import React, { useEffect, useRef, useState } from 'react';

const VideoPlayer = ({ videoUrl, currentTime, muted, volume, currentChannel }) => {
  const videoRef = useRef(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);
  const lastSyncTime = useRef(0);
  const syncIntervalRef = useRef(null);

  // Handle video loading
  useEffect(() => {
    if (videoRef.current && videoUrl) {
      setIsLoading(true);
      setError(null);
      
      const video = videoRef.current;
      
      // If URL changed, update the video source
      if (video.src !== videoUrl && videoUrl) {
        console.log(`Loading new video: ${videoUrl}`);
        video.src = videoUrl;
        
        // Set up event listeners for this new source
        const handleCanPlay = () => {
          setIsLoading(false);
          // When video can play, synchronize to server time
          synchronizeWithServer();
        };

        const handleError = (e) => {
          console.error('Video error:', e);
          setIsLoading(false);
          setError('Failed to load video. Please try again later.');
        };

        video.addEventListener('canplay', handleCanPlay);
        video.addEventListener('error', handleError);

        return () => {
          video.removeEventListener('canplay', handleCanPlay);
          video.removeEventListener('error', handleError);
        };
      }
    }
  }, [videoUrl]);

  // Synchronize with server time
  const synchronizeWithServer = () => {
    if (!videoRef.current || !currentTime) return;
    
    const video = videoRef.current;
    const now = Date.now();
    
    // Only sync if we haven't synced in the last 5 seconds or if we're too far off
    const timeSinceLastSync = now - lastSyncTime.current;
    const timeDifference = Math.abs(video.currentTime - currentTime);
    
    if (timeSinceLastSync > 5000 || timeDifference > 3) {
      console.log(`Syncing video time: server=${currentTime}, current=${video.currentTime}`);
      try {
        video.currentTime = currentTime;
        lastSyncTime.current = now;
      } catch (e) {
        console.error('Error setting video time:', e);
      }
    }
  };

  // Set up regular synchronization with server
  useEffect(() => {
    // Sync immediately when current time changes significantly
    if (currentTime) {
      synchronizeWithServer();
    }
    
    // Set up periodic sync for minor corrections
    if (!syncIntervalRef.current) {
      syncIntervalRef.current = setInterval(() => {
        synchronizeWithServer();
      }, 10000); // Check sync every 10 seconds
    }
    
    return () => {
      if (syncIntervalRef.current) {
        clearInterval(syncIntervalRef.current);
        syncIntervalRef.current = null;
      }
    };
  }, [currentTime]);

  // Apply volume and mute settings
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    
    video.volume = volume;
    video.muted = muted;
  }, [volume, muted]);

  // Start playing automatically when ready and keep playing
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    
    const handlePause = () => {
      // If video somehow pauses, restart it (unless at the end)
      if (video.currentTime < video.duration - 0.5) {
        video.play().catch(e => console.error('Failed to auto-resume video:', e));
      }
    };
    
    video.addEventListener('pause', handlePause);
    
    // Attempt to play if we have a URL
    if (videoUrl && video.paused) {
      video.play().catch(e => console.error('Failed to auto-play video:', e));
    }
    
    return () => {
      video.removeEventListener('pause', handlePause);
    };
  }, [videoUrl, isLoading]);

  return (
    <div className="video-player-container absolute inset-0 z-0">
      {isLoading && (
        <div className="video-loading absolute inset-0 flex items-center justify-center bg-black bg-opacity-70 text-white text-2xl z-10">
          Loading video...
        </div>
      )}
      
      {error && (
        <div className="video-error absolute inset-0 flex items-center justify-center bg-black bg-opacity-70 text-red-500 text-2xl z-10">
          {error}
        </div>
      )}
      
      <video 
        ref={videoRef}
        className="video-player w-full h-full object-cover bg-black"
        autoPlay
        playsInline
      >
        {videoUrl && <source src={videoUrl} type="video/mp4" />}
        Your browser does not support the video tag.
      </video>
      
      <div className="video-info absolute bottom-4 left-4 text-white bg-black bg-opacity-50 px-4 py-2 rounded">
        Channel {currentChannel} - {Math.floor(currentTime / 60)}:{Math.floor(currentTime % 60).toString().padStart(2, '0')}
      </div>
    </div>
  );
};

export default VideoPlayer;
