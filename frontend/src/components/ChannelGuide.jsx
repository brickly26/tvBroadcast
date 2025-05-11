import React from 'react';
import { motion } from 'framer-motion';

const ChannelGuide = ({ channelInfo, currentChannel, changeChannel, closeGuide }) => {
  // If channel info is not available yet, show loading
  if (!channelInfo || channelInfo.length === 0) {
    return (
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="fixed inset-0 bg-black bg-opacity-80 flex items-center justify-center z-50"
        onClick={closeGuide}
      >
        <div 
          className="bg-gray-900 rounded-lg p-6 max-w-2xl w-full mx-4 text-white"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-xl font-bold">Channel Guide</h2>
            <button onClick={closeGuide} className="text-gray-400 hover:text-white">
              <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" 
                stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
          <p className="text-center text-gray-400">Loading channel information...</p>
        </div>
      </motion.div>
    );
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 bg-black bg-opacity-80 flex items-center justify-center z-50"
      onClick={closeGuide}
    >
      <div 
        className="bg-gray-900 rounded-lg p-6 max-w-2xl w-full mx-4 text-white"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-bold">Channel Guide</h2>
          <button onClick={closeGuide} className="text-gray-400 hover:text-white">
            <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" 
              stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>
        
        <div className="overflow-y-auto max-h-96">
          {channelInfo.map((channel) => (
            <div 
              key={channel.number}
              className={`border-b border-gray-700 py-3 px-2 flex items-center ${
                channel.number === currentChannel ? 'bg-blue-900 bg-opacity-50' : ''
              }`}
              onClick={() => {
                changeChannel(channel.number);
                closeGuide();
              }}
            >
              <div className="flex-shrink-0 w-10 h-10 bg-gray-800 rounded-full flex items-center justify-center mr-3">
                {channel.number}
              </div>
              <div className="flex-grow">
                <h3 className="font-medium">{channel.name}</h3>
                <p className="text-sm text-gray-400">
                  Now Playing: {channel.currentVideo?.title || 'No information available'}
                </p>
              </div>
              {channel.number === currentChannel && (
                <div className="ml-2 text-blue-400 font-medium">
                  WATCHING
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </motion.div>
  );
};

export default ChannelGuide;
