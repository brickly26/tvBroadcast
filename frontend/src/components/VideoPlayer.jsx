/*  VideoPlayer.jsx  ─────────────────────────────────────────────────────
 *
 *  Requirements :
 *    npm i mp4box
 *  or (if you already added it)
 *    import MP4Box from "mp4box/dist/mp4box.all.min.js";
 *
 *  This component:
 *    • fetches  /live/{channelNumber}  (fragmented‑MP4)
 *    • parses   incoming boxes with mp4box.js
 *    • pushes   ready segments to Media Source Extensions
 *    • shows    spinner / error overlays like the original
 *    • keeps    the same Tailwind classes & channel label
 * -------------------------------------------------------------------- */
import React, { useEffect, useRef, useState } from "react";
import MP4Box from "mp4box"; // ←  gpac/mp4box.js

// In development, we use the current host with Vite's proxy
// In production, we use the origin (same domain as the frontend)
const API_HOST = ""; // Empty string means "use the current host"

// Configure debug logging
const DEBUG = true;
const debugLog = (...args) => {
  if (DEBUG) console.log("[VideoPlayer]", ...args);
};

// Test direct fetch to live endpoint - can be called outside of component
const testLiveEndpoint = async (channelNumber) => {
  const url = `${API_HOST}/live/${channelNumber}`;
  debugLog(`Testing live endpoint: ${url}`);

  try {
    // Make a HEAD request first to check if endpoint exists
    const headResponse = await fetch(url, {
      method: "HEAD",
      headers: { Accept: "video/mp4" },
    });

    debugLog(
      `HEAD response: ${headResponse.status} ${headResponse.statusText}`
    );
    debugLog(`Content-Type: ${headResponse.headers.get("content-type")}`);

    if (!headResponse.ok) {
      return {
        success: false,
        status: headResponse.status,
        message: `Backend server returned ${headResponse.status} ${headResponse.statusText}`,
      };
    }

    // Make a real fetch but abort it after getting first chunk
    const controller = new AbortController();
    const response = await fetch(url, {
      signal: controller.signal,
      headers: { Accept: "video/mp4" },
    });

    if (!response.ok) {
      return {
        success: false,
        status: response.status,
        message: `Server returned ${response.status} ${response.statusText}`,
      };
    }

    if (!response.body) {
      return {
        success: false,
        status: response.status,
        message: "Response has no body stream",
      };
    }

    // Try to read the first chunk
    const reader = response.body.getReader();
    const { value, done } = await reader.read();

    // Abort the fetch after getting the first chunk
    controller.abort();

    if (done) {
      return {
        success: false,
        message: "Stream ended immediately (empty stream)",
      };
    }

    return {
      success: true,
      message: `Got first chunk: ${value.byteLength} bytes`,
      byteLength: value.byteLength,
    };
  } catch (err) {
    return {
      success: false,
      message: `Error connecting to stream: ${err.message}`,
    };
  }
};

// Map AAC audio object types to their codec strings
const getAudioCodec = (objectType) => {
  const codecMap = {
    2: "mp4a.40.2", // AAC-LC
    5: "mp4a.40.5", // HE-AAC v1
    29: "mp4a.40.29", // HE-AAC v2
    64: "mp4a.40.5", // Object type 0x40 == HE-AAC v1
  };
  return codecMap[objectType] || "mp4a.40.2"; // Default to AAC-LC if unknown
};

function VideoPlayer({ channelNumber, muted = true, volume = 1 }) {
  const videoRef = useRef(null);
  const mediaSourceRef = useRef(null);
  const sourceBuffersRef = useRef({});
  const hasFirstSegmentRef = useRef(false);
  const pendingBuffersRef = useRef([]);
  const initSegmentsProcessedRef = useRef({});
  const mediaSourceErrorRef = useRef(false);
  const playAttemptedRef = useRef(false);
  const fetchControllerRef = useRef(null);
  const isUnmountingRef = useRef(false);
  const usingDirectModeRef = useRef(false);
  const lastMoofRef = useRef(null);
  const lastMdatRef = useRef(null);

  // ADDED: New flag to completely bypass MP4Box in a desperate attempt to get any video
  const [bypassMP4Box, setBypassMP4Box] = useState(false);
  const rawBufferRef = useRef(null); // To store the raw data for direct feeding

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [canPlay, setCanPlay] = useState(false);

  // Add endpoint test state
  const [endpointStatus, setEndpointStatus] = useState(null);

  // Run the endpoint test when channel changes
  useEffect(() => {
    if (channelNumber) {
      // Test live endpoint before starting the player
      testLiveEndpoint(channelNumber)
        .then((result) => {
          setEndpointStatus(result);
          debugLog(`Live endpoint test result:`, result);

          if (!result.success) {
            setError(`Stream error: ${result.message}`);
          }
        })
        .catch((err) => {
          debugLog(`Live endpoint test failed: ${err.message}`);
          setEndpointStatus({ success: false, message: err.message });
        });
    }
  }, [channelNumber]);

  // Flush any source buffer queue if it exists
  const safeFlushQueue = (trackId) => {
    if (isUnmountingRef.current) return;

    const buf = sourceBuffersRef.current[trackId];
    if (!buf || !buf.sb || buf.sb.updating || !buf.queue.length) return;

    try {
      const segment = buf.queue.shift();
      debugLog(
        `Appending ${segment.byteLength} bytes to buffer for track ${trackId}`
      );
      buf.sb.appendBuffer(segment);
    } catch (err) {
      if (err.name === "QuotaExceededError") {
        // If buffer is full, remove some data from the start
        debugLog(`Buffer full for track ${trackId}, removing old data`);
        if (buf.sb && buf.sb.buffered.length > 0) {
          const video = videoRef.current;
          if (video) {
            const currentTime = video.currentTime;
            const bufferEnd = buf.sb.buffered.end(0);
            // Keep at least 10 seconds, but remove earlier content if needed
            if (bufferEnd - currentTime > 10 && currentTime > 0) {
              buf.sb.remove(0, currentTime - 2);
            }
          }
        }
        // Push the segment back to try again later
        buf.queue.unshift(segment);
      } else if (!isUnmountingRef.current) {
        debugLog(`Buffer append error for track ${trackId}: ${err.message}`);
        mediaSourceErrorRef.current = true;
      }
    }
  };

  // Try to play the video - ensure it's muted for autoplay
  const attemptPlayback = () => {
    if (isUnmountingRef.current || playAttemptedRef.current) return;

    const video = videoRef.current;
    if (!video) return;

    // Mark as attempted
    playAttemptedRef.current = true;

    // Ensure video is muted for autoplay compliance
    video.muted = true;

    // Attempt to play
    debugLog("Attempting to play video...");
    const playPromise = video.play();

    if (playPromise !== undefined) {
      playPromise
        .then(() => {
          debugLog("Autoplay successful!");

          // Make sure the video element has valid content to play
          setTimeout(() => {
            if (video.readyState === 0 && !isUnmountingRef.current) {
              debugLog(
                "Video element has no data to play, might need more segments"
              );
              // Reset play attempted flag to try again
              playAttemptedRef.current = false;
            }
          }, 1000);
        })
        .catch((err) => {
          debugLog("Autoplay failed:", err.message);
          if (err.name === "NotAllowedError") {
            setError("Tap screen to play video");
          } else {
            // Reset playback attempt flag to try again
            setTimeout(() => {
              if (!isUnmountingRef.current) {
                playAttemptedRef.current = false;
                attemptPlayback();
              }
            }, 1000);
          }
        });
    }
  };

  // ADDED: Function to try completely raw playback approach
  const attemptRawPlayback = () => {
    if (!rawBufferRef.current || rawBufferRef.current.length === 0) {
      debugLog("No raw data available for direct playback");
      return;
    }

    // Try creating a blob URL directly
    try {
      const blob = new Blob([rawBufferRef.current], { type: "video/mp4" });
      const url = URL.createObjectURL(blob);

      // Set the video source directly
      const video = videoRef.current;
      if (video) {
        debugLog("Attempting raw playback via blob URL");
        video.src = url;
        video.muted = true;
        video.play().catch((e) => {
          debugLog(`Raw playback failed: ${e.message}`);
        });
      }
    } catch (e) {
      debugLog(`Error creating blob URL: ${e.message}`);
    }
  };

  /* ─── Main effect: create stream, attach MSE, tear down when channelNumber changes ─────────────── */
  useEffect(() => {
    // Skip if no valid channelNumber
    if (channelNumber == null) return;

    const video = videoRef.current;
    if (!video) return;

    // Reset state
    isUnmountingRef.current = false;
    setLoading(true);
    setError(null);
    setCanPlay(false);
    hasFirstSegmentRef.current = false;
    pendingBuffersRef.current = [];
    initSegmentsProcessedRef.current = {};
    mediaSourceErrorRef.current = false;
    playAttemptedRef.current = false;
    sourceBuffersRef.current = {};
    usingDirectModeRef.current = false;
    lastMoofRef.current = null;
    lastMdatRef.current = null;

    // Set a loading timeout - force play after 5 seconds even if segments haven't arrived
    // This is a safety net in case segment events aren't firing
    const loadingTimeoutId = setTimeout(() => {
      if (loading && !isUnmountingRef.current) {
        debugLog("Loading timeout reached - forcing playback attempt");
        setLoading(false);
        setCanPlay(true);
      }
    }, 5000);

    // Ensure video is muted for autoplay
    video.muted = true;
    video.volume = muted ? 0 : volume;

    debugLog(`Setting up channel ${channelNumber}`);

    /* 1. boot MediaSource */
    const mediaSource = new MediaSource();
    mediaSourceRef.current = mediaSource;
    video.src = URL.createObjectURL(mediaSource);

    /* 2. Use mp4box for parsing and segmentation */
    const mp4boxFile = MP4Box.createFile();
    let nextFilePos = 0; // byte offset fed so far
    fetchControllerRef.current = new AbortController(); // For cancellation
    let fetchReader; // cancelled on cleanup
    let initSegmentReceived = false;
    let tracksReady = false;

    // ADDED: Debug option to use naive approach
    const useNaiveDirectMode = true; // Set to true to completely bypass mp4box.js

    /* ── Check if MediaSource and SourceBuffer are still valid ──────── */
    const isMediaSourceValid = () => {
      if (isUnmountingRef.current || mediaSourceErrorRef.current) {
        return false;
      }

      if (mediaSource.readyState === "closed") {
        mediaSourceErrorRef.current = true;
        return false;
      }

      return true;
    };

    /* ── mp4box callbacks ──────────────────────────────────────────── */
    mp4boxFile.onError = (e) => {
      if (isUnmountingRef.current) return;
      console.error("mp4box error", e);

      // ADDED: More detailed MP4Box error logging
      const errorDetail =
        typeof e === "object" ? JSON.stringify(e) : e.toString();
      debugLog(`MP4Box parse error: ${errorDetail}`);

      // Check the current state of parsing
      debugLog(
        `MP4Box state at error: filePos=${nextFilePos}, chunks processed=${chunkCount}`
      );

      // Don't set error immediately for minor parsing issues
      // This allows playback to potentially continue with other segments
      if (initSegmentReceived && hasFirstSegmentRef.current) {
        debugLog(
          "Continuing playback despite MP4Box error (init segment already processed)"
        );
      } else {
        setError("Stream parse error");
      }
    };

    /** Called when a media segment is ready */
    mp4boxFile.onSegment = (id, user, buffer, sampleNum, is_last) => {
      if (isUnmountingRef.current) return;
      debugLog(
        `Received segment for track ${id}, ${buffer.byteLength} bytes, samples: ${sampleNum}, last: ${is_last}`
      );

      // ADDED: More detailed segment inspection
      const segView = new DataView(buffer, 0, Math.min(buffer.byteLength, 16));
      let segHex = "";
      for (let i = 0; i < segView.byteLength; i++) {
        segHex += segView.getUint8(i).toString(16).padStart(2, "0") + " ";
      }
      debugLog(`Segment start bytes: ${segHex}`);

      // Added: Check if this is the user object we passed in
      if (user && user.trackId) {
        debugLog(`Segment is for track ${user.trackId}`);
      }

      const trackBuffer = sourceBuffersRef.current[id];
      if (!trackBuffer) {
        debugLog(`No buffer found for track ${id}`);
        return;
      }

      // Mark that this track has received a media segment
      trackBuffer.hasReceivedSegment = true;

      // Queue the segment for this track
      trackBuffer.queue.push(buffer);
      safeFlushQueue(id);

      // Once we've received at least one media segment, attempt to play
      if (loading && !hasFirstSegmentRef.current) {
        // Simply start playback after first segment arrives
        // This helps with streams that might only send one track's segments
        hasFirstSegmentRef.current = true;
        debugLog("First media segment received, ready to play");
        setLoading(false);
        setCanPlay(true);
      }
    };

    /** Called when moov box is parsed */
    mp4boxFile.onReady = (info) => {
      if (isUnmountingRef.current) return;
      debugLog("mp4box ready, info:", info);

      initSegmentReceived = true;

      if (mediaSource.readyState !== "open") {
        setError("MediaSource not open when init segment received");
        return;
      }

      // ADDED: Process any existing samples after moov box is parsed
      // This is critical in fragmented MP4s where samples may have already arrived
      debugLog("Processing any existing samples...");
      try {
        mp4boxFile.flush();
      } catch (e) {
        debugLog(`Flush after moov error: ${e.message}`);
      }

      // Find video and audio tracks
      let videoTrack = null;
      let audioTrack = null;

      for (const track of info.tracks) {
        if (track.video) {
          videoTrack = track;
          debugLog(
            `Video track ${track.id}: ${track.codec}, timescale: ${track.timescale}`
          );
        } else if (track.audio) {
          audioTrack = track;
          debugLog(
            `Audio track ${track.id}: ${track.codec}, objectType: ${
              track.audio ? track.audio.audio_object_type : "unknown"
            }, timescale: ${track.timescale}`
          );
        }
      }

      if (!videoTrack) {
        setError("No video track found in stream");
        return;
      }

      // Log the actual data of the moov box for debugging
      debugLog("Track details:", JSON.stringify(info.tracks, null, 2));

      // Configure segmentation for each track
      try {
        debugLog("Setting up segmentation for tracks");

        // Set up track segmentation with appropriate sample counts
        info.tracks.forEach((track) => {
          // Force smaller segments to get more frequent segment events
          // Using a smaller value can help trigger onSegment events more frequently
          const nbSamples = 10; // Very small segment size to ensure frequent segments
          debugLog(
            `Setting segment options for track ${track.id} with ${nbSamples} samples per segment`
          );

          // MODIFIED: Add user param to help track the segment in logs
          mp4boxFile.setSegmentOptions(track.id, null, {
            rapAlignement: true,
            nbSamples: nbSamples,
            user: { trackId: track.id }, // Add user object to help identify track in onSegment
          });
        });

        // Initialize segmentation
        const initSegments = mp4boxFile.initializeSegmentation();
        debugLog(`Initialized segmentation with ${initSegments.length} tracks`);

        // Create source buffers for each track
        initSegments.forEach((segment) => {
          const track = info.tracks.find((t) => t.id === segment.id);

          if (!track) {
            debugLog(`No track info found for id ${segment.id}`);
            return;
          }

          let mimeType;
          if (track.video) {
            mimeType = `video/mp4; codecs="${track.codec}"`;
          } else if (track.audio) {
            mimeType = `audio/mp4; codecs="${getAudioCodec(
              track.audio ? track.audio.audio_object_type : 2
            )}"`;
          } else {
            debugLog(`Unknown track type for id ${segment.id}`);
            return;
          }

          debugLog(
            `Creating SourceBuffer for track ${segment.id} with MIME type: ${mimeType}`
          );

          if (!MediaSource.isTypeSupported(mimeType)) {
            debugLog(`Browser cannot play ${mimeType}`);
            return;
          }

          try {
            const sb = mediaSource.addSourceBuffer(mimeType);
            // For live streams, 'segments' mode is more appropriate
            sb.mode = "segments";

            sourceBuffersRef.current[segment.id] = {
              sb,
              queue: [],
              type: track.video ? "video" : "audio",
              hasReceivedSegment: false, // Track if we've received any media segments
            };

            // Set up updateend event handler
            sb.addEventListener("updateend", () => {
              if (isUnmountingRef.current) return;

              if (!initSegmentsProcessedRef.current[segment.id]) {
                initSegmentsProcessedRef.current[segment.id] = true;
                debugLog(
                  `Initialization segment processed for track ${segment.id}`
                );

                // ADDED: Log buffer state after init segment
                if (sb.buffered.length > 0) {
                  debugLog(
                    `Initial buffer for track ${
                      segment.id
                    }: ${sb.buffered.start(0)} to ${sb.buffered.end(0)}`
                  );
                } else {
                  debugLog(
                    `No buffered data for track ${segment.id} after init segment`
                  );
                }
              }

              safeFlushQueue(segment.id);
            });

            // ADDED: Log more detailed init segment info
            const segData = new Uint8Array(segment.buffer);
            debugLog(
              `Init segment for track ${segment.id}: ${track.type}, codec: ${track.codec}, size: ${segData.byteLength} bytes`
            );

            // Look for ftyp and moov boxes
            let offset = 0;
            while (offset < segData.byteLength - 8) {
              const size =
                (segData[offset] << 24) |
                (segData[offset + 1] << 16) |
                (segData[offset + 2] << 8) |
                segData[offset + 3];
              const type = new TextDecoder().decode(
                segData.subarray(offset + 4, offset + 8)
              );
              debugLog(`Box in init segment: ${type}, size: ${size} bytes`);
              offset += size;
              if (offset > segData.byteLength) {
                debugLog(`Warning: Box size exceeds buffer length`);
                break;
              }
            }

            // Append the initialization segment
            debugLog(
              `Appending initialization segment for track ${segment.id} (${segment.buffer.byteLength} bytes)`
            );
            sb.appendBuffer(segment.buffer);
          } catch (err) {
            if (!isUnmountingRef.current) {
              debugLog(
                `Failed to create source buffer for track ${segment.id}: ${err.message}`
              );
            }
          }
        });

        // Start a timer to check if we're getting segments
        // If after 3 seconds we've processed init segments but no media segments have arrived,
        // we'll start playback anyway
        setTimeout(() => {
          if (!isUnmountingRef.current && loading) {
            let allInitSegmentsProcessed = true;
            let anyMediaSegmentsReceived = false;

            for (const trackId in sourceBuffersRef.current) {
              if (!initSegmentsProcessedRef.current[trackId]) {
                allInitSegmentsProcessed = false;
              }
              if (sourceBuffersRef.current[trackId].hasReceivedSegment) {
                anyMediaSegmentsReceived = true;
              }
            }

            if (allInitSegmentsProcessed && !anyMediaSegmentsReceived) {
              debugLog(
                "Init segments processed but no media segments received after 3s - forcing playback attempt"
              );

              // ADDED: Last-ditch attempt to process segments by forcing flush
              debugLog("Last attempt to force process segments");
              try {
                mp4boxFile.flush();
              } catch (err) {
                debugLog(`Final flush error: ${err.message}`);
              }

              // ADDED: Switch to complete bypass mode if all else fails
              if (
                !anyMediaSegmentsReceived &&
                rawBufferRef.current &&
                rawBufferRef.current.length > 500000
              ) {
                // If we've collected enough raw data, try the direct approach
                debugLog(
                  `No segments after 3s, trying complete bypass with ${rawBufferRef.current.length} bytes of raw data`
                );
                setBypassMP4Box(true);
                setTimeout(attemptRawPlayback, 100);
              }

              // ADDED: Try the naive direct feeding approach as last resort
              if (useNaiveDirectMode) {
                debugLog(
                  "Enabling naive direct mode to bypass MP4Box segmentation entirely"
                );
                usingDirectModeRef.current = true;
              }

              // ADDED: If we see moof boxes but no segments, try manual processing approach
              debugLog(
                "Trying direct processing of next moof+mdat pair if mp4box segmentation failed"
              );

              // ADDED: Switch to direct mode regardless if we have a moof (mdat might come later)
              if (lastMoofRef.current) {
                debugLog("Switching to direct moof+mdat feeding mode");
                usingDirectModeRef.current = true;

                // Try direct feeding if we already have both parts
                if (lastMdatRef.current) {
                  tryDirectFeedingMoofMdat();
                } else {
                  debugLog("Waiting for mdat to appear for direct feeding");
                }
              }

              setLoading(false);
              setCanPlay(true);
            }
          }
        }, 3000);
      } catch (err) {
        if (!isUnmountingRef.current) {
          setError(`Failed to setup media tracks: ${err.message}`);
        }
        return;
      }

      tracksReady = true;
    };

    /* ── once MediaSource opens, start the network fetch ───────────── */
    const handleSourceOpen = () => {
      if (isUnmountingRef.current) return;
      debugLog("MediaSource opened, starting fetch");

      // Build the complete URL for clarity in logs
      const streamUrl = `${API_HOST}/live/${channelNumber}`;
      debugLog(`Fetching stream from: ${streamUrl}`);

      // For live fMP4, use the fetch API with ReadableStream
      fetch(streamUrl, {
        signal: fetchControllerRef.current.signal,
        headers: {
          Accept: "video/mp4",
        },
      })
        .then((res) => {
          if (isUnmountingRef.current) return;

          debugLog(`Stream response status: ${res.status} ${res.statusText}`);
          debugLog(`Stream response type: ${res.type}`);
          debugLog(`Stream content-type: ${res.headers.get("content-type")}`);

          if (!res.ok) {
            throw new Error(`Server returned ${res.status} ${res.statusText}`);
          }

          if (!res.body) {
            throw new Error("Stream response has no body");
          }

          debugLog("Stream fetch started, getting reader...");
          fetchReader = res.body.getReader();
          debugLog("Stream reader obtained successfully");

          // Keep track of how many chunks we've processed
          let chunkCount = 0;
          let lastSegmentCheck = 0;
          let bytesReceived = 0;
          let lastLogTime = Date.now();

          // Initialize raw buffer for potential direct playback
          rawBufferRef.current = new Uint8Array(0);

          /** recursive read‑loop */
          const pump = () => {
            if (isUnmountingRef.current) return;

            return fetchReader
              .read()
              .then(({ value, done }) => {
                if (isUnmountingRef.current) return;

                if (done) {
                  debugLog("Stream complete (done=true received)");
                  return;
                }

                // Log first chunk receipt
                if (chunkCount === 0) {
                  debugLog(
                    `First chunk received! Length: ${
                      value?.byteLength || 0
                    } bytes`
                  );

                  // ADDED: Detailed inspection of first chunk
                  if (value && value.byteLength > 0) {
                    const view = new DataView(
                      value.buffer,
                      value.byteOffset,
                      Math.min(value.byteLength, 24)
                    );
                    let hexDump = "";
                    for (let i = 0; i < view.byteLength; i++) {
                      hexDump +=
                        view.getUint8(i).toString(16).padStart(2, "0") + " ";
                    }
                    debugLog(`First 24 bytes: ${hexDump}`);

                    // Check for ftyp box
                    const boxType = new TextDecoder().decode(
                      new Uint8Array(value.buffer, value.byteOffset + 4, 4)
                    );
                    debugLog(`First box type: ${boxType}`);
                  }
                }

                if (!value || value.byteLength === 0) {
                  debugLog(`Received empty chunk, continuing`);
                  return pump();
                }

                if (!isMediaSourceValid()) {
                  debugLog("MediaSource invalid, stopping stream");
                  fetchReader?.cancel();
                  return;
                }

                // Keep track of stream progress
                chunkCount++;
                bytesReceived += value.byteLength;

                // ADDED: Append to raw buffer if we're collecting for bypass mode
                if (chunkCount <= 200) {
                  // Limit to first ~10MB to avoid memory issues
                  const newBuffer = new Uint8Array(
                    rawBufferRef.current.length + value.byteLength
                  );
                  newBuffer.set(rawBufferRef.current, 0);
                  newBuffer.set(
                    new Uint8Array(
                      value.buffer,
                      value.byteOffset,
                      value.byteLength
                    ),
                    rawBufferRef.current.length
                  );
                  rawBufferRef.current = newBuffer;

                  // If we've gone into bypass mode, try raw playback
                  if (
                    bypassMP4Box &&
                    rawBufferRef.current.length > 1000000 &&
                    chunkCount % 20 === 0
                  ) {
                    debugLog(
                      `Trying raw playback with ${rawBufferRef.current.length} bytes`
                    );
                    attemptRawPlayback();
                  }
                }

                // Log progress every second
                const now = Date.now();
                if (now - lastLogTime > 1000) {
                  debugLog(
                    `Stream progress: ${chunkCount} chunks, ${bytesReceived} bytes received`
                  );
                  lastLogTime = now;
                }

                // If we're completely bypassing MP4Box, don't process this chunk
                if (bypassMP4Box) {
                  return pump();
                }

                const ab = value.buffer.slice(
                  value.byteOffset,
                  value.byteOffset + value.byteLength
                );
                ab.fileStart = nextFilePos;
                nextFilePos += ab.byteLength;

                try {
                  // Always process with mp4box - it will emit segments through onSegment callback
                  if (!isUnmountingRef.current) {
                    // Before giving to mp4box, scan the data quickly to check if it has moof or mdat boxes
                    // This can help debug if the stream contains expected media fragments
                    const dataAsUint8 = new Uint8Array(ab);
                    // First check for common box types at start of chunk
                    let checkStr = new TextDecoder().decode(
                      dataAsUint8.subarray(4, 8)
                    ); // Box type is at offset 4

                    // ADDED: Deeper inspection of chunks to find mdat boxes
                    // Sometimes mdats might not be at the start of a chunk
                    let hasMdat = false;
                    if (checkStr !== "mdat" && dataAsUint8.length > 12) {
                      // Check for mdat anywhere in the first part of the chunk
                      const chunkString = new TextDecoder().decode(
                        dataAsUint8.subarray(
                          0,
                          Math.min(100, dataAsUint8.length)
                        )
                      );
                      if (chunkString.includes("mdat")) {
                        const mdatIndex = chunkString.indexOf("mdat");
                        if (mdatIndex >= 4) {
                          // Make sure we have size bytes
                          hasMdat = true;
                          debugLog(
                            `Found mdat at offset ${
                              mdatIndex - 4
                            } in chunk ${chunkCount}`
                          );

                          // Create a view of just the mdat part
                          const mdatStart = mdatIndex - 4;
                          const mdatData = new Uint8Array(ab.slice(mdatStart));
                          lastMdatRef.current = mdatData;
                          debugLog(
                            `Extracted and cached mdat from chunk ${chunkCount}`
                          );

                          // If we're in direct mode and have a moof, try to use this mdat
                          if (
                            usingDirectModeRef.current &&
                            lastMoofRef.current
                          ) {
                            tryDirectFeedingMoofMdat();
                          }
                        }
                      }
                    }

                    if (
                      chunkCount === 1 ||
                      checkStr === "moof" ||
                      checkStr === "mdat" ||
                      checkStr === "ftyp" ||
                      checkStr === "moov" ||
                      hasMdat
                    ) {
                      debugLog(
                        `Chunk ${chunkCount} contains box type: ${
                          hasMdat ? "mdat (embedded)" : checkStr
                        }, size: ${dataAsUint8.byteLength} bytes`
                      );

                      // ADDED: Cache moof and mdat for manual processing if needed
                      if (checkStr === "moof") {
                        lastMoofRef.current = new Uint8Array(ab.slice(0));
                        debugLog(
                          `Cached moof box (${dataAsUint8.byteLength} bytes) for possible manual processing`
                        );
                      } else if (checkStr === "mdat") {
                        lastMdatRef.current = new Uint8Array(ab.slice(0));
                        debugLog(
                          `Cached mdat box (${dataAsUint8.byteLength} bytes) for possible manual processing`
                        );

                        // If we're in direct mode and we have a moof+mdat pair, feed it directly
                        if (usingDirectModeRef.current && lastMoofRef.current) {
                          tryDirectFeedingMoofMdat();
                        }
                      }

                      // ADDED: Log details of key MP4 boxes
                      if (checkStr === "moof" || checkStr === "moov") {
                        // Log box size (first 4 bytes as big-endian uint32)
                        const boxSize =
                          (dataAsUint8[0] << 24) |
                          (dataAsUint8[1] << 16) |
                          (dataAsUint8[2] << 8) |
                          dataAsUint8[3];
                        debugLog(`${checkStr} box size: ${boxSize} bytes`);
                      }
                    }

                    // Important: Use the correct fileStart parameter for mp4box
                    mp4boxFile.appendBuffer(ab, ab.fileStart);

                    // ADDED: If in direct mode and we have init segments, try directly feeding the data
                    // This simplified approach bypasses MP4Box segmentation entirely
                    if (
                      usingDirectModeRef.current &&
                      initSegmentReceived &&
                      chunkCount > 2
                    ) {
                      const videoBuffer = Object.values(
                        sourceBuffersRef.current
                      ).find((sb) => sb.type === "video")?.sb;

                      if (videoBuffer && !videoBuffer.updating) {
                        try {
                          // Try feeding raw chunks directly to MSE as a last resort
                          debugLog(
                            `Direct mode: Feeding raw chunk ${chunkCount} directly (${ab.byteLength} bytes)`
                          );
                          videoBuffer.appendBuffer(ab);
                          hasFirstSegmentRef.current = true;
                        } catch (e) {
                          // Ignore errors, we'll try with the next chunk
                          debugLog(`Error in direct chunk feed: ${e.message}`);
                        }
                      }
                    }

                    // ADDED: Explicitly try to process media segments after each moof+mdat pair
                    // This helps when the MP4 structure doesn't automatically trigger segmentation
                    if (
                      checkStr === "mdat" &&
                      initSegmentReceived &&
                      !hasFirstSegmentRef.current
                    ) {
                      debugLog(
                        "Found mdat box, flushing to process potential segments"
                      );
                      try {
                        mp4boxFile.flush();
                      } catch (e) {
                        debugLog(`Flush error: ${e.message}`);
                      }
                    }

                    if (chunkCount % 10 === 0) {
                      debugLog(
                        `Appended chunk ${chunkCount} (${ab.byteLength} bytes) at position ${ab.fileStart}`
                      );
                    }
                  }
                } catch (err) {
                  debugLog(
                    `Error processing chunk ${chunkCount}: ${err.message}`
                  );
                }

                if (!isUnmountingRef.current) {
                  return pump();
                }
              })
              .catch((e) => {
                // Provide more details on stream read errors
                if (e.name !== "AbortError" && !isUnmountingRef.current) {
                  console.error("Stream pump error:", e);
                  debugLog(`Stream read failed: ${e.name} - ${e.message}`);
                  setError(`Stream error: ${e.message}`);
                }
              });
          };

          // Start the pump
          pump().catch((err) => {
            if (!isUnmountingRef.current) {
              debugLog(`Stream pump chain error: ${err.message}`);
              setError(`Stream pump error: ${err.message}`);
            }
          });
        })
        .catch((e) => {
          // Improve fetch error reporting
          if (e.name !== "AbortError" && !isUnmountingRef.current) {
            console.error("Stream fetch error:", e);
            debugLog(`Failed to fetch stream: ${e.name} - ${e.message}`);
            setError(`Network error: ${e.message}`);
          }
        });
    };

    mediaSource.addEventListener("sourceopen", handleSourceOpen);

    /* Error handlers for MSE */
    const handleVideoError = (e) => {
      if (isUnmountingRef.current) return;

      const errorCode = video.error ? video.error.code : 0;
      const errorMsg = video.error ? video.error.message : "Unknown error";

      // ADDED: More detailed video element error reporting
      let errorName = "Unknown";
      if (video.error) {
        switch (video.error.code) {
          case 1:
            errorName = "MEDIA_ERR_ABORTED";
            break;
          case 2:
            errorName = "MEDIA_ERR_NETWORK";
            break;
          case 3:
            errorName = "MEDIA_ERR_DECODE";
            break;
          case 4:
            errorName = "MEDIA_ERR_SRC_NOT_SUPPORTED";
            break;
        }
      }

      console.error(`Video error: ${errorCode} (${errorName}) - ${errorMsg}`);
      debugLog(`Video error: ${errorCode} (${errorName}) - ${errorMsg}`);

      // Check MediaSource and SourceBuffer state
      if (mediaSourceRef.current) {
        debugLog(`MediaSource state: ${mediaSourceRef.current.readyState}`);
        const bufferInfo = Object.entries(sourceBuffersRef.current)
          .map(
            ([id, { type, queue }]) => `${type}(${id}): queue=${queue.length}`
          )
          .join(", ");
        debugLog(`SourceBuffers: ${bufferInfo}`);
      }

      setError(`Video error: ${errorMsg}`);
      mediaSourceErrorRef.current = true;
    };

    video.addEventListener("error", handleVideoError);

    /* ─── cleanup ──────────────────────────────────────────────────── */
    return () => {
      clearTimeout(loadingTimeoutId);
      debugLog("Cleaning up player resources");

      // Mark as unmounting to prevent further operations
      isUnmountingRef.current = true;

      // Remove event listeners
      mediaSource.removeEventListener("sourceopen", handleSourceOpen);
      video.removeEventListener("error", handleVideoError);

      // Cancel any ongoing fetch properly
      if (fetchControllerRef.current) {
        try {
          fetchControllerRef.current.abort();
        } catch (e) {
          console.error("Error aborting fetch:", e);
        }
      }

      // Cancel reader if it exists
      if (fetchReader) {
        try {
          fetchReader.cancel();
        } catch (e) {
          console.error("Error cancelling reader:", e);
        }
      }

      // Clean up mp4box
      if (mp4boxFile) {
        try {
          mp4boxFile.flush();
        } catch (e) {
          console.error("Error flushing mp4box", e);
        }
      }

      // Clean up video element
      if (video) {
        video.pause();
        video.removeAttribute("src");
        video.load();
      }

      // Clear refs
      mediaSourceRef.current = null;
      sourceBuffersRef.current = {};
    };
  }, [channelNumber]);

  // Attempt playback when canPlay becomes true
  useEffect(() => {
    if (canPlay && !isUnmountingRef.current) {
      attemptPlayback();
    }
  }, [canPlay]);

  /* Handle mute/volume changes separately */
  useEffect(() => {
    const video = videoRef.current;
    if (!video || isUnmountingRef.current) return;

    // Apply mute state and volume, but don't change playback state
    video.muted = muted;
    video.volume = volume;
  }, [muted, volume]);

  /* Handle user click to play when autoplay is blocked */
  const handlePlayerClick = () => {
    if (error && error.includes("Tap")) {
      setError(null);
      const video = videoRef.current;
      if (video) {
        // Ensure muted for first interaction
        video.muted = true;
        video.play().catch((e) => {
          console.error("Play after click failed", e);
          setError(`Cannot start playback: ${e.message}`);
        });
      }
    }
  };

  // MODIFIED: Direct moof+mdat feeding for cases where MP4Box segmentation isn't working
  const tryDirectFeedingMoofMdat = () => {
    if (!lastMoofRef.current || !lastMdatRef.current) {
      debugLog("No moof+mdat pair available for direct feeding");
      return;
    }

    debugLog("Attempting direct feeding of moof+mdat pair");

    // Get our video and audio SourceBuffers
    const sourceBuffers = sourceBuffersRef.current;
    const videoBuffer = Object.values(sourceBuffers).find(
      (sb) => sb.type === "video"
    )?.sb;
    const audioBuffer = Object.values(sourceBuffers).find(
      (sb) => sb.type === "audio"
    )?.sb;

    if (!videoBuffer || !audioBuffer) {
      debugLog("Missing source buffers for direct mode");
      return;
    }

    // Check if either buffer is updating
    if (videoBuffer.updating || audioBuffer.updating) {
      debugLog("Source buffers are updating, will try again later");
      setTimeout(tryDirectFeedingMoofMdat, 100);
      return;
    }

    // Combine the moof+mdat into a single fragment
    const totalLength = lastMoofRef.current.length + lastMdatRef.current.length;
    debugLog(
      `Creating combined fragment: moof (${lastMoofRef.current.length} bytes) + mdat (${lastMdatRef.current.length} bytes) = ${totalLength} bytes`
    );

    const combinedFragment = new Uint8Array(totalLength);
    combinedFragment.set(lastMoofRef.current, 0);
    combinedFragment.set(lastMdatRef.current, lastMoofRef.current.length);

    // Store references for debugging in case of failure
    const moof = lastMoofRef.current;
    const mdat = lastMdatRef.current;

    // Clear the refs immediately to avoid reusing the same fragments
    lastMoofRef.current = null;
    lastMdatRef.current = null;

    try {
      // For the first attempt, only feed to video buffer as a test
      debugLog(
        `Feeding moof+mdat directly to video buffer only (${combinedFragment.length} bytes)`
      );
      videoBuffer.appendBuffer(combinedFragment.buffer);

      // Wait for video buffer to finish
      const handleVideoUpdateEnd = () => {
        videoBuffer.removeEventListener("updateend", handleVideoUpdateEnd);

        debugLog("Video buffer updated successfully with direct fragment");

        // Mark that we've received media
        if (!hasFirstSegmentRef.current) {
          hasFirstSegmentRef.current = true;
          debugLog("First fragment fed directly, setting canPlay=true");
          setLoading(false);
          setCanPlay(true);
        }

        // Don't try to add to audio buffer for now - might cause errors
        // and we just need video to test if this approach works
      };

      videoBuffer.addEventListener("updateend", handleVideoUpdateEnd);
    } catch (e) {
      debugLog(`Error feeding buffer directly: ${e.message}`);

      // Try a different approach - use just the moof as a test
      try {
        debugLog("Failed with combined fragment, trying with just moof box");
        videoBuffer.appendBuffer(moof.buffer);
      } catch (e) {
        debugLog(`Error appending just moof: ${e.message}`);
      }
    }
  };

  /* ─── UI – exact same Tailwind look & feel ────────────────────────── */
  return (
    <div
      className="video-player-container relative w-full h-full"
      onClick={handlePlayerClick}
    >
      {loading && !error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 text-white z-10">
          <div className="text-center">
            <div>Loading channel&nbsp;{channelNumber}…</div>
            {endpointStatus && !endpointStatus.success && (
              <div className="text-red-400 text-sm mt-2">
                {endpointStatus.message}
              </div>
            )}
          </div>
        </div>
      )}

      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 text-red-500 z-10">
          {error}
        </div>
      )}

      <video
        ref={videoRef}
        className="w-full h-full object-cover bg-black"
        autoPlay
        muted
        playsInline
        controlsList="nodownload noremoteplayback nofullscreen noplaybackrate"
      />

      <div className="absolute bottom-4 left-4 bg-black/60 text-white px-3 py-1 rounded">
        Live&nbsp;·&nbsp;Channel&nbsp;{channelNumber}
      </div>

      {/* ADDED: Debug buttons to try different approaches */}
      <div className="absolute top-4 right-4 flex flex-col gap-2">
        <button
          className="bg-blue-600 text-white px-3 py-1 rounded text-xs"
          onClick={() => {
            setBypassMP4Box(true);
            attemptRawPlayback();
          }}
        >
          Try Raw
        </button>
        <button
          className="bg-green-600 text-white px-3 py-1 rounded text-xs"
          onClick={() => {
            if (lastMoofRef.current && lastMdatRef.current) {
              tryDirectFeedingMoofMdat();
            }
          }}
        >
          Feed Moof+Mdat
        </button>
      </div>
    </div>
  );
}

export default VideoPlayer;
