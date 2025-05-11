import React, { useState, useEffect, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";

const FloatingMenu = ({
  currentChannel,
  changeChannel,
  volume,
  changeVolume,
  muted,
  toggleMute,
  toggleGuide,
  toggleAdmin,
  isAdmin = false,
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const [isVisible, setIsVisible] = useState(true);
  const hideTimeoutRef = useRef(null);

  // Toggle menu open/closed state
  const toggleMenu = () => {
    setIsOpen(!isOpen);
    // Keep the menu visible when open
    if (!isOpen) {
      clearTimeout(hideTimeoutRef.current);
    } else {
      startHideTimeout();
    }
  };

  // Reset visibility timer when mouse moves
  useEffect(() => {
    const handleMouseMove = () => {
      setIsVisible(true);
      startHideTimeout();
    };

    document.addEventListener("mousemove", handleMouseMove);
    startHideTimeout();

    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      clearTimeout(hideTimeoutRef.current);
    };
  }, []);

  // Set a timeout to hide the UI after inactivity
  const startHideTimeout = () => {
    clearTimeout(hideTimeoutRef.current);
    hideTimeoutRef.current = setTimeout(() => {
      if (!isOpen) {
        setIsVisible(false);
      }
    }, 1000); // Hide after 1 second of inactivity
  };

  // Handle channel change with bounds check
  const handleChannelChange = (direction) => {
    const newChannel =
      direction === "up"
        ? Math.min(currentChannel + 1, 5)
        : Math.max(currentChannel - 1, 1);

    changeChannel(newChannel);
  };

  // Handle volume change with step value
  const handleVolumeChange = (direction) => {
    const step = 10; // 10% increment/decrement
    const newVolume = direction === "up" ? volume + step : volume - step;

    changeVolume(newVolume);
  };

  // Power button functionality (reload the page for now)
  const handlePower = () => {
    window.location.reload();
  };

  return (
    <div className="absolute bottom-4 right-4 z-10">
      {/* Main toggle button (Remote Control) */}
      <motion.button
        whileTap={{ scale: 0.95 }}
        onClick={toggleMenu}
        className={`bg-gray-800 bg-opacity-70 text-white rounded-full p-3 shadow-lg 
                   transition-opacity duration-300 ${
                     isVisible ? "opacity-100" : "opacity-0"
                   }`}
      >
        {/* Remote Control Icon */}
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          {isOpen ? (
            // X icon for close
            <>
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </>
          ) : (
            // Remote control icon
            <>
              <rect x="5" y="2" width="14" height="20" rx="2" ry="2"></rect>
              <circle cx="12" cy="8" r="2"></circle>
              <line x1="12" y1="12" x2="12" y2="16"></line>
              <line x1="8" y1="16" x2="16" y2="16"></line>
            </>
          )}
        </svg>
      </motion.button>

      {/* Expanded remote control menu */}
      <AnimatePresence>
        {isOpen && (
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
            className="absolute right-0 bottom-16 bg-gray-800 bg-opacity-80 rounded-lg shadow-lg p-4 w-64"
          >
            {/* Remote Control Header */}
            <div className="text-center text-white font-semibold mb-3 pb-2 border-b border-gray-600">
              Remote Control
            </div>

            {/* 2-Column Grid Layout */}
            <div className="grid grid-cols-2 gap-3">
              {/* Power Button */}
              <motion.button
                whileTap={{ scale: 0.95 }}
                onClick={handlePower}
                className="flex flex-col items-center justify-center bg-red-700 bg-opacity-90 text-white 
                         rounded-md py-3 px-2 shadow hover:bg-red-600 transition"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M18.36 6.64a9 9 0 1 1-12.73 0"></path>
                  <line x1="12" y1="2" x2="12" y2="12"></line>
                </svg>
                <span className="text-xs mt-1">Power</span>
              </motion.button>

              {/* Mute Toggle */}
              <motion.button
                whileTap={{ scale: 0.95 }}
                onClick={toggleMute}
                className="flex flex-col items-center justify-center bg-gray-700 bg-opacity-90 text-white 
                         rounded-md py-3 px-2 shadow hover:bg-gray-600 transition"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  {muted ? (
                    // Volume X (muted)
                    <>
                      <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
                      <line x1="23" y1="9" x2="17" y2="15"></line>
                      <line x1="17" y1="9" x2="23" y2="15"></line>
                    </>
                  ) : (
                    // Volume icon based on level
                    <>
                      <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
                      {volume > 0 && (
                        <path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path>
                      )}
                      {volume > 50 && (
                        <path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path>
                      )}
                    </>
                  )}
                </svg>
                <span className="text-xs mt-1">
                  {muted ? "Unmute" : "Mute"}
                </span>
              </motion.button>

              {/* Channel Up */}
              <motion.button
                whileTap={{ scale: 0.95 }}
                onClick={() => handleChannelChange("up")}
                className="flex flex-col items-center justify-center bg-blue-700 bg-opacity-90 text-white 
                         rounded-md py-3 px-2 shadow hover:bg-blue-600 transition"
                disabled={currentChannel === 5}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polyline points="18 15 12 9 6 15"></polyline>
                </svg>
                <span className="text-xs mt-1">CH+</span>
              </motion.button>

              {/* Volume Up */}
              <motion.button
                whileTap={{ scale: 0.95 }}
                onClick={() => handleVolumeChange("up")}
                className="flex flex-col items-center justify-center bg-green-700 bg-opacity-90 text-white 
                         rounded-md py-3 px-2 shadow hover:bg-green-600 transition"
                disabled={volume >= 100}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
                  <line x1="15" y1="12" x2="21" y2="12"></line>
                  <line x1="18" y1="9" x2="18" y2="15"></line>
                </svg>
                <span className="text-xs mt-1">VOL+</span>
              </motion.button>

              {/* Channel Down */}
              <motion.button
                whileTap={{ scale: 0.95 }}
                onClick={() => handleChannelChange("down")}
                className="flex flex-col items-center justify-center bg-blue-700 bg-opacity-90 text-white 
                         rounded-md py-3 px-2 shadow hover:bg-blue-600 transition"
                disabled={currentChannel === 1}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polyline points="6 9 12 15 18 9"></polyline>
                </svg>
                <span className="text-xs mt-1">CH-</span>
              </motion.button>

              {/* Volume Down */}
              <motion.button
                whileTap={{ scale: 0.95 }}
                onClick={() => handleVolumeChange("down")}
                className="flex flex-col items-center justify-center bg-green-700 bg-opacity-90 text-white 
                         rounded-md py-3 px-2 shadow hover:bg-green-600 transition"
                disabled={volume <= 0}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
                  <line x1="15" y1="12" x2="21" y2="12"></line>
                </svg>
                <span className="text-xs mt-1">VOL-</span>
              </motion.button>

              {/* Channel Guide Button */}
              <motion.button
                whileTap={{ scale: 0.95 }}
                onClick={toggleGuide}
                className="flex flex-col items-center justify-center bg-purple-700 bg-opacity-90 text-white 
                         rounded-md py-3 px-2 shadow hover:bg-purple-600 transition"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
                  <line x1="3" y1="9" x2="21" y2="9"></line>
                  <line x1="9" y1="21" x2="9" y2="9"></line>
                </svg>
                <span className="text-xs mt-1">Guide</span>
              </motion.button>

              {/* Admin Dashboard Button */}
              {isAdmin ? (
                <motion.button
                  whileTap={{ scale: 0.95 }}
                  onClick={toggleAdmin}
                  className="flex flex-col items-center justify-center bg-yellow-700 bg-opacity-90 text-white 
                            rounded-md py-3 px-2 shadow hover:bg-yellow-600 transition"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
                    <circle cx="9" cy="7" r="4"></circle>
                    <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
                    <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
                  </svg>
                  <span className="text-xs mt-1">Admin</span>
                </motion.button>
              ) : (
                <motion.button
                  whileTap={{ scale: 0.95 }}
                  onClick={toggleAdmin}
                  className="flex flex-col items-center justify-center bg-blue-700 bg-opacity-90 text-white 
                            rounded-md py-3 px-2 shadow hover:bg-blue-600 transition"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
                    <circle cx="9" cy="7" r="4"></circle>
                    <line x1="16" y1="11" x2="22" y2="11"></line>
                    <line x1="19" y1="8" x2="19" y2="14"></line>
                  </svg>
                  <span className="text-xs mt-1">Login</span>
                </motion.button>
              )}

              {/* Channel Info */}
              <div className="col-span-2 mt-2 border-t border-gray-600 pt-2 text-center text-white text-sm">
                <span className="font-bold">Channel {currentChannel}</span>
                <span className="mx-2">•</span>
                <span>Volume: {volume}%</span>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
};

export default FloatingMenu;
