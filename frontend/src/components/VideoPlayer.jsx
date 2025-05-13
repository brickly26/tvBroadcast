import React, { useEffect, useRef, useState } from "react";

const API_HOST = "http://localhost:8080"; // back‑end base URL

/**
 * Live, un‑seekable player for GET /live/{channelNumber}
 *
 * Props
 * ────────────────────────────────────────────────────────────────
 * channelNumber   int        – which live channel to watch
 * muted           boolean    – start muted (needed for autoplay)
 * volume          0‥1        – initial volume
 */
const VideoPlayer = ({ channelNumber, muted = true, volume = 1.0 }) => {
  const videoRef = useRef(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);

  /* ─── create / switch stream ──────────────────────────────────────── */
  useEffect(() => {
    if (channelNumber === undefined || channelNumber === null) return;

    const video = videoRef.current;
    if (!video) return;

    setIsLoading(true);
    setError(null);

    const mediaSource = new MediaSource();
    let reader; // fetch reader for tidy cleanup
    let firstChunk = true; // hide spinner after first append

    /* fires once when MSE is ready                                      */
    const handleSourceOpen = () => {
      /* pick a codec string the current browser actually supports ------ */
      const candidates = [
        'video/mp4; codecs="avc1.640028,mp4a.40.2"', // H.264 High L4.0 + AAC
        'video/mp4; codecs="avc1.4d401f,mp4a.40.2"', // Main L3.1
        'video/mp4; codecs="avc1.42E01E,mp4a.40.2"', // Baseline L3.0
        "video/mp4", // wildcard fallback
      ];
      const mime = candidates.find((c) => MediaSource.isTypeSupported(c));
      if (!mime) {
        setError("Browser lacks MP4 (H.264/AAC) support");
        return;
      }

      const buf = mediaSource.addSourceBuffer(mime);
      buf.addEventListener("error", (e) => {
        console.error("SourceBuffer fatal error", e);
        setError("Browser rejected the stream");
      });

      /* recursive read‑&‑append loop with updateend gating ------------- */
      const pump = () =>
        reader.read().then(({ value, done }) => {
          if (done) {
            const finish = () => mediaSource.endOfStream();
            return buf.updating
              ? buf.addEventListener("updateend", finish, { once: true })
              : finish();
          }

          const push = () => {
            if (!buf.updating) {
              try {
                buf.appendBuffer(value);
                if (firstChunk) {
                  setIsLoading(false);
                  firstChunk = false;
                }
                pump(); // next chunk
              } catch (e) {
                console.error("appendBuffer failed:", e);
                setError("Decode error");
              }
            } else {
              buf.addEventListener("updateend", push, { once: true });
            }
          };
          push();
        });

      /* start the network fetch --------------------------------------- */
      fetch(`${API_HOST}/live/${channelNumber}`)
        .then((res) => {
          reader = res.body.getReader();
          pump();
        })
        .catch((e) => {
          console.error("Stream fetch failed:", e);
          setError("Stream error");
        });
    };

    /* attach BEFORE setting src to avoid missing the event              */
    mediaSource.addEventListener("sourceopen", handleSourceOpen);
    video.src = URL.createObjectURL(mediaSource);

    /* respect autoplay policies                                         */
    video.muted = muted;
    video.volume = volume;
    video.play().catch(() => {
      /* user will need to click play; spinner stays until then */
    });

    /* cleanup on unmount / channel change ----------------------------- */
    return () => {
      mediaSource.removeEventListener("sourceopen", handleSourceOpen);
      if (reader) reader.cancel();
      video.pause();
      video.removeAttribute("src");
      video.load();
    };
  }, [channelNumber]);

  /* apply mute / volume updates on the fly ---------------------------- */
  useEffect(() => {
    const v = videoRef.current;
    if (!v) return;
    v.muted = muted;
    v.volume = volume;
  }, [muted, volume]);

  /* ─── UI markup (unchanged styling) ───────────────────────────────── */
  return (
    <div className="video-player-container relative w-full h-full">
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 text-white z-10">
          Loading channel&nbsp;{channelNumber}…
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
        playsInline
        controlsList="nodownload noremoteplayback nofullscreen noplaybackrate"
      />

      <div className="absolute bottom-4 left-4 text-white bg-black/60 px-3 py-1 rounded">
        Live&nbsp;·&nbsp;Channel&nbsp;{channelNumber}
      </div>
    </div>
  );
};

export default VideoPlayer;
