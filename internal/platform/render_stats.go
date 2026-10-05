package platform

// RenderStats reports the most recently encoded/completed native frame. Frame
// counters and UsedSlotsMask are cumulative within a Run; other fields describe
// the last frame. Individual counters may change between snapshot field reads.
type RenderStats struct {
	Backend       string `json:"backend"`
	FrameSlots    uint32 `json:"frame_slots"`
	UsedSlotsMask uint32 `json:"used_slots_mask"`
	InFlight      uint32 `json:"in_flight"`
	MaxInFlight   uint32 `json:"max_in_flight"`
	Submitted     uint64 `json:"submitted"`
	Completed     uint64 `json:"completed"`
	DrawCalls     uint64 `json:"draw_calls"`
	Instances     uint64 `json:"instances"`
	UploadedBytes uint64 `json:"uploaded_bytes"`
	BufferWaits   uint64 `json:"buffer_waits"`
	CPUTimeNanos  uint64 `json:"cpu_time_nanos"`
	GPUTimeNanos  uint64 `json:"gpu_time_nanos"`
}
