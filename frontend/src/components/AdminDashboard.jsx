import React, { useState, useEffect } from "react";
import { motion } from "framer-motion";
import toast from "react-hot-toast";

const AdminDashboard = ({ closeAdmin, isOpen }) => {
  // Upload states
  const [youtubeUrl, setYoutubeUrl] = useState("");
  const [channelNumber, setChannelNumber] = useState(1);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadStatus, setUploadStatus] = useState(null);

  // Channel management states
  const [selectedChannel, setSelectedChannel] = useState(1);
  const [channelVideos, setChannelVideos] = useState([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isLoadingChannelDetails, setIsLoadingChannelDetails] = useState(false);
  const [activeTab, setActiveTab] = useState("upload");
  const [channelDetails, setChannelDetails] = useState(null);
  const [isEditingChannel, setIsEditingChannel] = useState(false);
  const [editedChannelDetails, setEditedChannelDetails] = useState(null);
  const [isSavingChannel, setIsSavingChannel] = useState(false);

  // Fetch channel videos when selected channel changes
  useEffect(() => {
    if (activeTab === "manage") {
      fetchChannelVideos(selectedChannel);
      fetchChannelDetails(selectedChannel);
    }
  }, [selectedChannel, activeTab]);

  // Fetch channel details
  const fetchChannelDetails = async (channelId) => {
    setIsLoadingChannelDetails(true);
    setChannelDetails(null); // Clear previous data while loading

    try {
      const response = await fetch(`/api/admin/channel?channel=${channelId}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          "Cache-Control": "no-cache, no-store, must-revalidate",
          Pragma: "no-cache",
          Expires: "0",
        },
        credentials: "include", // Include cookies for authentication
      });

      if (!response.ok) {
        // If server returns 401/403, we have an auth issue
        if (response.status === 401 || response.status === 403) {
          throw new Error("Authentication required. Please login again.");
        }
        throw new Error("Failed to fetch channel details");
      }

      const data = await response.json();
      setChannelDetails(data.channel);
      setEditedChannelDetails(data.channel); // Initialize edit form with current values
    } catch (error) {
      console.error("Error fetching channel details:", error);

      // Create a fallback channel detail if API fails
      const fallbackChannel = {
        number: channelId,
        name: `Channel ${channelId}`,
        description: "Channel description not available",
        theme: "entertainment",
      };

      setChannelDetails(fallbackChannel);
      setEditedChannelDetails(fallbackChannel);

      // Show error message as a toast but don't empty the channel data
      toast.error(`Channel data may not be current: ${error.message}`);
    } finally {
      setIsLoadingChannelDetails(false);
    }
  };

  // Fetch videos for a specific channel
  const fetchChannelVideos = async (channelId) => {
    setIsLoading(true);

    try {
      const response = await fetch(`/api/admin/videos?channel=${channelId}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          "Cache-Control": "no-cache, no-store, must-revalidate",
          Pragma: "no-cache",
          Expires: "0",
        },
        credentials: "include", // Include cookies for authentication
      });

      if (!response.ok) {
        // If server returns 401/403, we have an auth issue
        if (response.status === 401 || response.status === 403) {
          throw new Error("Authentication required. Please login again.");
        }
        throw new Error("Failed to fetch channel videos");
      }

      const data = await response.json();
      setChannelVideos(data.videos || []);
    } catch (error) {
      console.error("Error fetching videos:", error);
      toast.error(`Failed to load videos: ${error.message}`);
      setChannelVideos([]);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setIsUploading(true);
    setUploadStatus({ type: "info", message: "Processing your request..." });

    try {
      const response = await fetch("/api/admin/upload-video", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include", // Include cookies for authentication
        body: JSON.stringify({
          youtubeUrl,
          channelNumber: parseInt(channelNumber),
        }),
      });

      const result = await response.json();

      if (!response.ok) {
        // If server returns 401/403, we have an auth issue
        if (response.status === 401 || response.status === 403) {
          throw new Error("Authentication required. Please login again.");
        }
        throw new Error(result.message || "Failed to upload video");
      }

      setUploadStatus({
        type: "success",
        message: `Video successfully queued for channel ${channelNumber}!`,
      });
      toast.success(`Video successfully queued for channel ${channelNumber}!`);
      setYoutubeUrl("");
    } catch (error) {
      console.error("Upload error:", error);
      setUploadStatus({
        type: "error",
        message: error.message || "Failed to upload video",
      });
      toast.error(error.message || "Failed to upload video");
    } finally {
      setIsUploading(false);
    }
  };

  const channels = [1, 2, 3, 4, 5];

  // Handle deleting a video
  const handleDeleteVideo = async (videoId) => {
    if (!window.confirm("Are you sure you want to delete this video?")) {
      return;
    }

    try {
      const response = await fetch("/api/admin/delete-video", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include", // Include cookies for authentication
        body: JSON.stringify({ videoId }),
      });

      if (!response.ok) {
        // If server returns 401/403, we have an auth issue
        if (response.status === 401 || response.status === 403) {
          throw new Error("Authentication required. Please login again.");
        }
        throw new Error("Failed to delete video");
      }

      // Refresh the video list
      await fetchChannelVideos(selectedChannel);
      toast.success("Video deleted successfully");
    } catch (error) {
      console.error("Error deleting video:", error);
      toast.error(`Failed to delete video: ${error.message}`);
    }
  };

  // Handle saving channel details
  const handleSaveChannelDetails = async () => {
    setIsSavingChannel(true);
    try {
      const response = await fetch(`/api/admin/channel/${selectedChannel}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include", // Include cookies for authentication
        body: JSON.stringify(editedChannelDetails),
      });

      if (!response.ok) {
        // If server returns 401/403, we have an auth issue
        if (response.status === 401 || response.status === 403) {
          throw new Error("Authentication required. Please login again.");
        }
        throw new Error("Failed to update channel details");
      }

      const data = await response.json();
      setChannelDetails(data.channel);
      setIsEditingChannel(false);
      setIsSavingChannel(false);
      toast.success("Channel details updated successfully");
    } catch (error) {
      console.error("Error saving channel details:", error);
      toast.error(`Failed to save channel details: ${error.message}`);
      setIsSavingChannel(false);
    }
  };

  // New function to handle creating a new channel
  const handleCreateNewChannel = () => {
    // In a real app, this would redirect to a channel creation form
    // or open a modal for creating a new channel
    toast.error(
      "Create New Channel functionality will be implemented in the future"
    );
  };

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 bg-black bg-opacity-80 flex items-center justify-center z-50"
      onClick={closeAdmin}
    >
      <div
        className="bg-gray-900 rounded-lg p-6 max-w-4xl w-full mx-4 text-white flex flex-col h-[85vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex justify-between items-center mb-4 flex-shrink-0">
          <h2 className="text-xl font-bold">Admin Dashboard</h2>
          <button
            onClick={closeAdmin}
            className="text-gray-400 hover:text-white"
          >
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
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        {/* Tab Navigation */}
        <div className="flex border-b border-gray-700 mb-4 flex-shrink-0">
          <button
            className={`py-2 px-4 font-medium ${
              activeTab === "upload"
                ? "text-blue-400 border-b-2 border-blue-400"
                : "text-gray-400 hover:text-gray-200"
            }`}
            onClick={() => setActiveTab("upload")}
          >
            Upload Video
          </button>
          <button
            className={`py-2 px-4 font-medium ${
              activeTab === "manage"
                ? "text-blue-400 border-b-2 border-blue-400"
                : "text-gray-400 hover:text-gray-200"
            }`}
            onClick={() => setActiveTab("manage")}
          >
            Manage Channels
          </button>
        </div>

        {/* Main Content Area - Flexbox with flex-grow to take available space */}
        <div className="flex-grow flex flex-col overflow-hidden">
          {/* Upload Video Tab */}
          {activeTab === "upload" && (
            <div className="mb-4 p-4 bg-gray-800 rounded-lg flex-shrink-0">
              <h3 className="text-lg font-semibold mb-3">
                Upload YouTube Video
              </h3>

              <form onSubmit={handleSubmit}>
                <div className="mb-4">
                  <label
                    htmlFor="youtubeUrl"
                    className="block text-sm font-medium text-gray-300 mb-1"
                  >
                    YouTube URL
                  </label>
                  <input
                    type="text"
                    id="youtubeUrl"
                    value={youtubeUrl}
                    onChange={(e) => setYoutubeUrl(e.target.value)}
                    placeholder="https://www.youtube.com/watch?v=..."
                    className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                    required
                  />
                </div>

                <div className="mb-4">
                  <label
                    htmlFor="channelNumber"
                    className="block text-sm font-medium text-gray-300 mb-1"
                  >
                    Target Channel
                  </label>
                  <select
                    id="channelNumber"
                    value={channelNumber}
                    onChange={(e) => setChannelNumber(e.target.value)}
                    className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                  >
                    {channels.map((channel) => (
                      <option key={channel} value={channel}>
                        Channel {channel}
                      </option>
                    ))}
                  </select>
                </div>

                <button
                  type="submit"
                  disabled={isUploading || !youtubeUrl}
                  className={`w-full py-2 px-4 rounded-md text-white font-medium 
                  ${
                    isUploading || !youtubeUrl
                      ? "bg-gray-600 cursor-not-allowed"
                      : "bg-blue-600 hover:bg-blue-700"
                  }`}
                >
                  {isUploading ? "Processing..." : "Upload to S3"}
                </button>
              </form>

              {uploadStatus && (
                <div
                  className={`mt-4 p-3 rounded-md ${
                    uploadStatus.type === "success"
                      ? "bg-green-800 bg-opacity-50"
                      : uploadStatus.type === "error"
                      ? "bg-red-800 bg-opacity-50"
                      : "bg-blue-800 bg-opacity-50"
                  }`}
                >
                  {uploadStatus.message}
                </div>
              )}
            </div>
          )}

          {/* Manage Channels Tab */}
          {activeTab === "manage" && (
            <div className="flex flex-col h-full overflow-hidden">
              {/* Channel Selection Section - Fixed height */}
              <div className="flex items-center justify-between mb-3 flex-shrink-0">
                <div>
                  <label
                    htmlFor="selectedChannelManage"
                    className="block text-sm font-medium text-gray-300 mb-1"
                  >
                    Select Channel
                  </label>
                  <select
                    id="selectedChannelManage"
                    value={selectedChannel}
                    onChange={(e) => setSelectedChannel(Number(e.target.value))}
                    className="px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none"
                  >
                    {channels.map((channel) => (
                      <option key={channel} value={channel}>
                        Channel {channel}
                      </option>
                    ))}
                  </select>
                </div>

                <button
                  onClick={handleCreateNewChannel}
                  className="py-2 px-4 rounded-md text-white font-medium bg-blue-600 hover:bg-blue-700"
                >
                  Create New Channel
                </button>
              </div>

              {/* Channel Details Section - Fixed height */}
              {isLoadingChannelDetails ? (
                <div className="bg-gray-800 rounded-lg p-5 mb-3 flex-shrink-0 text-center">
                  <div className="flex flex-col items-center justify-center space-y-3">
                    <div className="animate-pulse flex space-x-4">
                      <div className="rounded-full bg-gray-700 h-12 w-12"></div>
                      <div className="flex-1 space-y-3 py-1">
                        <div className="h-2 bg-gray-700 rounded w-3/4"></div>
                        <div className="space-y-2">
                          <div className="h-2 bg-gray-700 rounded"></div>
                          <div className="h-2 bg-gray-700 rounded w-5/6"></div>
                        </div>
                      </div>
                    </div>
                    <p className="text-gray-400">Loading channel details...</p>
                  </div>
                </div>
              ) : channelDetails ? (
                <div className="bg-gray-800 rounded-lg p-3 mb-3 flex-shrink-0">
                  <div className="flex justify-between items-center mb-2">
                    <h3 className="text-lg font-semibold">Channel Details</h3>
                    {!isEditingChannel ? (
                      <button
                        onClick={() => setIsEditingChannel(true)}
                        className="py-1 px-3 rounded-md text-white font-medium bg-blue-600 hover:bg-blue-700 text-sm"
                      >
                        Edit Details
                      </button>
                    ) : (
                      <div className="flex space-x-2">
                        <button
                          onClick={() => setIsEditingChannel(false)}
                          className="py-1 px-3 rounded-md text-white font-medium bg-gray-600 hover:bg-gray-700 text-sm"
                          disabled={isSavingChannel}
                        >
                          Cancel
                        </button>
                        <button
                          onClick={handleSaveChannelDetails}
                          className="py-1 px-3 rounded-md text-white font-medium bg-green-600 hover:bg-green-700 text-sm"
                          disabled={isSavingChannel}
                        >
                          {isSavingChannel ? "Saving..." : "Save"}
                        </button>
                      </div>
                    )}
                  </div>

                  {!isEditingChannel ? (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
                      <div>
                        <p className="text-gray-400">Channel Number</p>
                        <p className="text-white font-medium">
                          {channelDetails.number}
                        </p>
                      </div>
                      <div>
                        <p className="text-gray-400">Channel Name</p>
                        <p className="text-white font-medium">
                          {channelDetails.name}
                        </p>
                      </div>
                      <div>
                        <p className="text-gray-400">Theme</p>
                        <p className="text-white font-medium capitalize">
                          {channelDetails.theme}
                        </p>
                      </div>
                      <div>
                        <p className="text-gray-400">Description</p>
                        <p className="text-white">
                          {channelDetails.description}
                        </p>
                      </div>
                    </div>
                  ) : (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
                      <div>
                        <label className="block text-gray-400 mb-1">
                          Channel Number
                        </label>
                        <input
                          type="number"
                          value={editedChannelDetails.number}
                          disabled
                          className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                        <p className="text-xs text-gray-500 mt-1">
                          Channel number cannot be changed
                        </p>
                      </div>
                      <div>
                        <label className="block text-gray-400 mb-1">
                          Channel Name
                        </label>
                        <input
                          type="text"
                          value={editedChannelDetails.name}
                          onChange={(e) =>
                            setEditedChannelDetails({
                              ...editedChannelDetails,
                              name: e.target.value,
                            })
                          }
                          className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                      </div>
                      <div>
                        <label className="block text-gray-400 mb-1">
                          Theme
                        </label>
                        <select
                          value={editedChannelDetails.theme}
                          onChange={(e) =>
                            setEditedChannelDetails({
                              ...editedChannelDetails,
                              theme: e.target.value,
                            })
                          }
                          className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        >
                          <option value="news">News</option>
                          <option value="sports">Sports</option>
                          <option value="nature">Nature</option>
                          <option value="technology">Technology</option>
                          <option value="entertainment">Entertainment</option>
                          <option value="education">Education</option>
                          <option value="music">Music</option>
                          <option value="gaming">Gaming</option>
                        </select>
                      </div>
                      <div>
                        <label className="block text-gray-400 mb-1">
                          Description
                        </label>
                        <textarea
                          value={editedChannelDetails.description}
                          onChange={(e) =>
                            setEditedChannelDetails({
                              ...editedChannelDetails,
                              description: e.target.value,
                            })
                          }
                          rows="2"
                          className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                        />
                      </div>
                    </div>
                  )}
                </div>
              ) : (
                <div className="bg-gray-800 rounded-lg p-5 mb-3 flex-shrink-0 text-center">
                  <div className="flex flex-col items-center justify-center space-y-3">
                    <p className="text-red-400">
                      Failed to load channel details
                    </p>
                    <button
                      onClick={() => fetchChannelDetails(selectedChannel)}
                      className="py-1 px-3 rounded-md text-white font-medium bg-blue-600 hover:bg-blue-700 text-sm mt-2"
                    >
                      Retry
                    </button>
                  </div>
                </div>
              )}

              <div className="flex justify-between items-center mb-3">
                <h3 className="text-xl font-semibold">
                  Videos on Channel {selectedChannel}
                </h3>
              </div>

              {/* Video List Section - THIS IS THE SCROLLABLE AREA */}
              <div className="flex-grow overflow-y-auto min-h-[300px] bg-gray-800 rounded-lg">
                {isLoading ? (
                  <div className="flex justify-center items-center h-full">
                    <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white"></div>
                  </div>
                ) : channelVideos.length === 0 ? (
                  <div className="bg-gray-800 rounded-lg p-4 text-center text-gray-400 h-full flex items-center justify-center">
                    <p>No videos available for this channel</p>
                  </div>
                ) : (
                  <div className="space-y-3 p-3">
                    {channelVideos.map((video, index) => (
                      <div
                        key={video.id}
                        className="bg-gray-800 rounded-lg p-3 flex flex-col md:flex-row gap-3 relative hover:bg-gray-750"
                      >
                        {/* Thumbnail */}
                        <div className="w-full md:w-1/3 lg:w-1/4 flex-shrink-0">
                          <div className="aspect-video bg-gray-700 rounded overflow-hidden relative">
                            <img
                              src={
                                video.thumbnailURL ||
                                `/api/thumbnails/thumbnail_${video.id}.jpg`
                              }
                              alt={video.title}
                              className="w-full h-full object-cover"
                              onError={(e) => {
                                e.target.onerror = null;
                                e.target.src =
                                  "/assets/images/video-placeholder.jpg";
                                // Apply a key to force re-render and prevent flashing
                                e.target.key = `placeholder-${video.id}`;
                                // Add a className to prevent further attempts
                                e.target.classList.add("placeholder-loaded");
                              }}
                              loading="lazy" // Add lazy loading for better performance
                            />
                          </div>
                        </div>

                        {/* Video Info */}
                        <div className="flex-grow">
                          <h4 className="text-lg font-medium mb-1">
                            {video.title}
                          </h4>
                          <p className="text-sm text-gray-400 line-clamp-2 mb-2">
                            {video.description || "No description available"}
                          </p>
                          <div className="flex flex-wrap gap-2 mb-2">
                            <span className="px-2 py-1 bg-gray-700 rounded text-xs">
                              Status: {video.status}
                            </span>
                            {video.duration && (
                              <span className="px-2 py-1 bg-gray-700 rounded text-xs">
                                Duration: {Math.floor(video.duration / 60)}:
                                {String(
                                  Math.floor(video.duration % 60)
                                ).padStart(2, "0")}
                              </span>
                            )}
                            <span className="px-2 py-1 bg-gray-700 rounded text-xs">
                              Uploaded:{" "}
                              {new Date(video.createdAt).toLocaleDateString()}
                            </span>
                          </div>

                          {/* Actions */}
                          <div className="flex gap-2 flex-wrap">
                            <button
                              onClick={() => handleDeleteVideo(video.id)}
                              className="px-3 py-1 bg-red-600 hover:bg-red-700 rounded text-sm"
                            >
                              Delete
                            </button>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </motion.div>
  );
};

export default AdminDashboard;
