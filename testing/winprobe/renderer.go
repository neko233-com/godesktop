package winprobe

import "github.com/neko233-com/godesktop/internal/platform"

// RenderStats describes the owned native renderer's actual submissions and
// resources. This diagnostic API performs no rendering or window input.
type RenderStats = platform.RenderStats

// RendererStats returns a thread-safe copy of the current/last Run's metrics.
// A minimized window may complete prior submissions without submitting new ones.
func RendererStats() RenderStats { return platform.RendererStats() }
