import React, { useState, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

const UnmuteButton = ({ muted, toggleMute }) => {
  const [isVisible, setIsVisible] = useState(true);
  const hideTimeoutRef = useRef(null);
  
  // Reset visibility timer when mouse moves or component mounts
  useEffect(() => {
    const handleMouseMove = () => {
      setIsVisible(true);
      startHideTimeout();
    };

    document.addEventListener('mousemove', handleMouseMove);
    startHideTimeout();

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      clearTimeout(hideTimeoutRef.current);
    };
  }, []);
  
  // Set a timeout to hide the button after inactivity
  const startHideTimeout = () => {
    clearTimeout(hideTimeoutRef.current);
    if (muted) { // Only start the timeout if muted (button is shown)
      hideTimeoutRef.current = setTimeout(() => {
        setIsVisible(false);
      }, 3000); // Hide after 3 seconds of inactivity
    }
  };
  
  // Don't render anything if not muted
  if (!muted) {
    return null;
  }

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: isVisible ? 1 : 0, y: 0 }}
        exit={{ opacity: 0, y: 20 }}
        transition={{ duration: 0.3 }}
        className="absolute bottom-6 left-6 z-10"
      >
        <motion.button
          whileTap={{ scale: 0.95 }}
          onClick={toggleMute}
          className="bg-gray-800 bg-opacity-80 hover:bg-opacity-100 text-white rounded-lg px-4 py-3 
                   shadow-lg flex items-center gap-2 transition-colors duration-300"
        >
          <svg 
            xmlns="http://www.w3.org/2000/svg" 
            width="22" 
            height="22" 
            viewBox="0 0 24 24" 
            fill="none" 
            stroke="currentColor" 
            strokeWidth="2" 
            strokeLinecap="round" 
            strokeLinejoin="round"
          >
            <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
            <line x1="23" y1="9" x2="17" y2="15"></line>
            <line x1="17" y1="9" x2="23" y2="15"></line>
          </svg>
          <span className="font-medium">Click to Unmute</span>
        </motion.button>
      </motion.div>
    </AnimatePresence>
  );
};

export default UnmuteButton;
