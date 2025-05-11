import React, { useState, useEffect } from "react";
import { motion } from "framer-motion";
import { DragDropContext, Droppable, Draggable } from "react-beautiful-dnd";

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
  const [errorMessage, setErrorMessage] = useState("");
  const [activeTab, setActiveTab] = useState("upload");

  // Video management states
  const [isReordering, setIsReordering] = useState(false);

  // Fetch channel videos when selected channel changes
  useEffect(() => {
    if (activeTab === "manage") {
      fetchChannelVideos(selectedChannel);
    }
  }, [selectedChannel, activeTab]);

  // Fetch videos for a specific channel
  const fetchChannelVideos = async (channelId) => {
    setIsLoading(true);
    setErrorMessage("");

    try {
      const response = await fetch(`/api/admin/videos?channel=${channelId}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      });

      if (!response.ok) {
        throw new Error("Failed to fetch channel videos");
      }

      const data = await response.json();
      setChannelVideos(data.videos || []);
    } catch (error) {
      console.error("Error fetching videos:", error);
      setErrorMessage("Failed to load videos for this channel");
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
        body: JSON.stringify({
          youtubeUrl,
          channelNumber: parseInt(channelNumber),
        }),
      });

      const result = await response.json();

      if (!response.ok) {
        throw new Error(result.message || "Failed to upload video");
      }

      setUploadStatus({
        type: "success",
        message: `Video successfully queued for channel ${channelNumber}!`,
      });
      setYoutubeUrl("");
    } catch (error) {
      console.error("Upload error:", error);
      setUploadStatus({
        type: "error",
        message: error.message || "Failed to upload video",
      });
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
        body: JSON.stringify({ videoId }),
      });

      if (!response.ok) {
        throw new Error("Failed to delete video");
      }

      // Refresh the video list
      await fetchChannelVideos(selectedChannel);
    } catch (error) {
      console.error("Error deleting video:", error);
      setErrorMessage("Failed to delete video");
    }
  };

  // Handle saving the reordered videos
  const handleSaveVideoOrder = async () => {
    setIsReordering(true);

    try {
      // Create a map of videoId to position
      const videoOrders = {};
      channelVideos.forEach((video, index) => {
        videoOrders[video.id] = index + 1;
      });

      const response = await fetch("/api/admin/update-video-order", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          channelNumber: selectedChannel,
          videoOrders,
        }),
      });

      if (!response.ok) {
        throw new Error("Failed to update video order");
      }

      // Refresh the video list
      await fetchChannelVideos(selectedChannel);
    } catch (error) {
      console.error("Error updating video order:", error);
      setErrorMessage("Failed to update video order");
    } finally {
      setIsReordering(false);
    }
  };

  // Handle drag and drop reordering
  const onDragEnd = (result) => {
    // If dropped outside the list
    if (!result.destination) {
      return;
    }

    // Reorder the channelVideos array
    const items = Array.from(channelVideos);
    const [reorderedItem] = items.splice(result.source.index, 1);
    items.splice(result.destination.index, 0, reorderedItem);

    setChannelVideos(items);
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
        className="bg-gray-900 rounded-lg p-6 max-w-4xl w-full mx-4 text-white overflow-y-auto max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex justify-between items-center mb-4">
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
        <div className="flex border-b border-gray-700 mb-6">
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

        {/* Upload Video Tab */}
        {activeTab === "upload" && (
          <div className="mb-6 p-4 bg-gray-800 rounded-lg">
            <h3 className="text-lg font-semibold mb-3">Upload YouTube Video</h3>

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
          <div className="mb-6">
            <div className="flex items-center justify-between mb-4">
              <div>
                <label
                  htmlFor="selectedChannelManage"
                  className="block text-sm font-medium text-gray-300 mb-1"
                >
                  Select Channel to Manage
                </label>
                <select
                  id="selectedChannelManage"
                  value={selectedChannel}
                  onChange={(e) => setSelectedChannel(Number(e.target.value))}
                  className="px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  {channels.map((channel) => (
                    <option key={channel} value={channel}>
                      Channel {channel}
                    </option>
                  ))}
                </select>
              </div>

              {channelVideos.length > 0 && (
                <button
                  onClick={handleSaveVideoOrder}
                  disabled={isReordering}
                  className={`py-2 px-4 rounded-md text-white font-medium ${
                    isReordering
                      ? "bg-gray-600 cursor-not-allowed"
                      : "bg-green-600 hover:bg-green-700"
                  }`}
                >
                  {isReordering ? "Saving..." : "Save Order"}
                </button>
              )}
            </div>

            {errorMessage && (
              <div className="mb-4 p-3 bg-red-800 bg-opacity-50 rounded-md">
                {errorMessage}
              </div>
            )}

            {isLoading ? (
              <div className="flex justify-center items-center h-48">
                <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white"></div>
              </div>
            ) : channelVideos.length === 0 ? (
              <div className="bg-gray-800 rounded-lg p-4 text-center text-gray-400">
                No videos available for this channel
              </div>
            ) : (
              <DragDropContext onDragEnd={onDragEnd}>
                <Droppable droppableId="videos">
                  {(provided) => (
                    <div
                      {...provided.droppableProps}
                      ref={provided.innerRef}
                      className="space-y-4"
                    >
                      {channelVideos.map((video, index) => (
                        <Draggable
                          key={video.id}
                          draggableId={video.id}
                          index={index}
                        >
                          {(provided) => (
                            <div
                              ref={provided.innerRef}
                              {...provided.draggableProps}
                              {...provided.dragHandleProps}
                              className="bg-gray-800 rounded-lg p-4 flex flex-col md:flex-row gap-4 relative hover:bg-gray-750"
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
                                      e.target.classList.add(
                                        "placeholder-loaded"
                                      );
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
                                  {video.description ||
                                    "No description available"}
                                </p>
                                <div className="flex flex-wrap gap-2 mb-3">
                                  <span className="px-2 py-1 bg-gray-700 rounded text-xs">
                                    Status: {video.status}
                                  </span>
                                  {video.duration && (
                                    <span className="px-2 py-1 bg-gray-700 rounded text-xs">
                                      Duration:{" "}
                                      {Math.floor(video.duration / 60)}:
                                      {String(
                                        Math.floor(video.duration % 60)
                                      ).padStart(2, "0")}
                                    </span>
                                  )}
                                  <span className="px-2 py-1 bg-gray-700 rounded text-xs">
                                    Uploaded:{" "}
                                    {new Date(
                                      video.createdAt
                                    ).toLocaleDateString()}
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

                              {/* Drag Handle */}
                              <div className="absolute top-2 right-2 text-gray-400">
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
                                  <circle cx="9" cy="5" r="1"></circle>
                                  <circle cx="9" cy="12" r="1"></circle>
                                  <circle cx="9" cy="19" r="1"></circle>
                                  <circle cx="15" cy="5" r="1"></circle>
                                  <circle cx="15" cy="12" r="1"></circle>
                                  <circle cx="15" cy="19" r="1"></circle>
                                </svg>
                              </div>
                            </div>
                          )}
                        </Draggable>
                      ))}
                      {provided.placeholder}
                    </div>
                  )}
                </Droppable>
              </DragDropContext>
            )}
          </div>
        )}

        <div className="mt-6 text-xs text-gray-400">
          <h4 className="font-medium text-sm text-gray-300 mb-2">
            Usage Instructions:
          </h4>
          <ol className="list-decimal pl-5 space-y-1">
            <li>
              Enter a valid YouTube URL (e.g.,
              https://www.youtube.com/watch?v=abcd1234)
            </li>
            <li>Select the target channel number (1-5)</li>
            <li>Click "Upload to S3" to start the process</li>
            <li>
              The server will download the video from YouTube and upload it to
              your S3 bucket
            </li>
            <li>
              Once complete, the video will be available on the selected channel
            </li>
          </ol>
          <p className="mt-2">
            Note: Uploading videos may take several minutes depending on video
            length and server load.
          </p>
          <p className="mt-1">
            The server requires{" "}
            <code className="px-1 py-0.5 bg-gray-700 rounded">youtube-dl</code>{" "}
            to be installed.
          </p>
        </div>
      </div>
    </motion.div>
  );
};

export default AdminDashboard;
