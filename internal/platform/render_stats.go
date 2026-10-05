package platform

// RenderStats reports the most recently encoded/completed native frame. Frame
// counters, UsedSlotsMask and glyph rasterization/hit/upload/peak/epoch counters
// are cumulative within a Run; cache entries/pages/bytes describe live resources.
// Other fields describe the last frame. Metal counters can change between reads.
// DeviceRecoveries counts resource rebuilds. DroppedFrames counts old submissions
// abandoned without confirmed GPU completion; after draining, Submitted equals
// Completed plus DroppedFrames. Ordinary rendering has no dropped frames.
// CPUTimeNanos is elapsed wall time through commit/present, including drawable waits;
// SceneTimeNanos, AcquireTimeNanos and EncodeTimeNanos split that interval.
type RenderStats struct {
	Backend             string `json:"backend"`
	FrameClock          string `json:"frame_clock"`
	FrameSlots          uint32 `json:"frame_slots"`
	UsedSlotsMask       uint32 `json:"used_slots_mask"`
	InFlight            uint32 `json:"in_flight"`
	MaxInFlight         uint32 `json:"max_in_flight"`
	Submitted           uint64 `json:"submitted"`
	Completed           uint64 `json:"completed"`
	DrawCalls           uint64 `json:"draw_calls"`
	Instances           uint64 `json:"instances"`
	UploadedBytes       uint64 `json:"uploaded_bytes"`
	BufferWaits         uint64 `json:"buffer_waits"`
	CPUTimeNanos        uint64 `json:"cpu_time_nanos"`
	GPUTimeNanos        uint64 `json:"gpu_time_nanos"`
	SceneTimeNanos      uint64 `json:"scene_time_nanos"`
	AcquireTimeNanos    uint64 `json:"drawable_acquire_time_nanos"`
	EncodeTimeNanos     uint64 `json:"encode_time_nanos"`
	FrameRequests       uint64 `json:"frame_requests"`
	FrameTicks          uint64 `json:"frame_ticks"`
	CoalescedRequests   uint64 `json:"coalesced_requests"`
	IdlePauses          uint64 `json:"idle_pauses"`
	GlyphRasterizations uint64 `json:"glyph_rasterizations"`
	GlyphCacheHits      uint64 `json:"glyph_cache_hits"`
	GlyphCacheEntries   uint64 `json:"glyph_cache_entries"`
	GlyphAtlasPages     uint64 `json:"glyph_atlas_pages"`
	GlyphAtlasBytes     uint64 `json:"glyph_atlas_bytes"`
	GlyphAtlasPeakBytes uint64 `json:"glyph_atlas_peak_bytes"`
	GlyphAtlasEpochs    uint64 `json:"glyph_atlas_epochs"`
	GlyphUploadedBytes  uint64 `json:"glyph_uploaded_bytes"`
	DeviceRecoveries    uint64 `json:"device_recoveries"`
	DroppedFrames       uint64 `json:"dropped_frames"`
}
