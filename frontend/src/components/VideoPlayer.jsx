import React, { useEffect, useRef, useState } from "react";

/**
 * Live, un‑seekable player that consumes /live/{channelNumber}
 *
 * Props
 * ─────────────────────────────
 * channelNumber   integer   – the channel to watch
 * muted           boolean   – keep sound off?
 * volume          0‥1       – volume level
 */
const VideoPlayer = ({ channelNumber, muted, volume }) => {
  const videoRef = useRef(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);

  /* ─── attach MediaSource and start pumping ─────────────────────────────── */
  useEffect(() => {
    if (!channelNumber && channelNumber !== 0) return; // wait for prop

    const video = videoRef.current;
    if (!video) return;

    setIsLoading(true);
    setError(null);

    const mediaSource = new MediaSource();
    video.src = URL.createObjectURL(mediaSource);

    let reader; // will hold the fetch reader so we can abort on unmount

    const handleSourceOpen = () => {
      const mime = 'video/mp4; codecs="avc1.42E01E, mp4a.40.2"'; // H.264 + AAC baseline
      if (!MediaSource.isTypeSupported(mime)) {
        setError("Browser lacks MP4 (H.264/AAC) support");
        return;
      }

      const sourceBuffer = mediaSource.addSourceBuffer(mime);

      /* fetch live stream in chunks */
      fetch(`/live/${channelNumber}`)
        .then((res) => {
          reader = res.body.getReader();
          const pump = () =>
            reader.read().then(({ value, done }) => {
              if (done) {
                mediaSource.endOfStream();
                return;
              }
              sourceBuffer.appendBuffer(value);
              setIsLoading(false); // first chunk arrived → hide spinner
              pump();
            });
          pump();
        })
        .catch((e) => {
          console.error("Stream fetch failed:", e);
          setError("Unable to connect to stream");
        });
    };

    mediaSource.addEventListener("sourceopen", handleSourceOpen);

    /* cleanup on unmount or channel switch */
    return () => {
      mediaSource.removeEventListener("sourceopen", handleSourceOpen);
      if (reader) reader.cancel();
      video.removeAttribute("src"); // drop object URL
      video.load();
    };
  }, [channelNumber]);

  /* ─── apply mute / volume every time they change ──────────────────────── */
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    video.volume = volume;
    video.muted = muted;
  }, [volume, muted]);

  /* ─── keep the video playing (no seek bar, no pausing) ─────────────────── */
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const resume = () => {
      if (!video.paused) return;
      video.play().catch(() => {}); // ignore “not allowed” – user may click
    };

    video.addEventListener("pause", resume);
    resume(); // attempt once on mount

    return () => video.removeEventListener("pause", resume);
  }, [isLoading]);

  return (
    <div className="video-player-container absolute inset-0 z-0">
      {isLoading && (
        <div className="video-loading absolute inset-0 flex items-center justify-center bg-black bg-opacity-70 text-white text-2xl z-10">
          Loading channel {channelNumber}…
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
        controlsList="nodownload noremoteplayback nofullscreen noplaybackrate"
      />

      <div className="video-info absolute bottom-4 left-4 text-white bg-black bg-opacity-50 px-4 py-2 rounded">
        Live · Channel {channelNumber}
      </div>
    </div>
  );
};

export default VideoPlayer;
