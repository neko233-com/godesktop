#ifndef GODESKTOP_BRIDGE_H
#define GODESKTOP_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct { float r, g, b, a; } GDColor;
typedef struct { float x, y, w, h; } GDRect;
typedef struct {
    int kind;
    GDRect bounds, clip;
    GDColor color;
    float radius, font_size;
    uint32_t text_offset, text_length;
    uint32_t font_offset, font_length;
} GDCommand;

// Counters describe real native encoding/submission, independently of Go view
// invocations. Backend: 0 unavailable, 1 legacy Direct2D, 2 Metal, 3 Direct3D 12.
// Frame clock: 0 unavailable, 1 MTKView, 2 CAMetalDisplayLink, 3 DXGI.
typedef struct {
    uint32_t backend, frame_slots, used_slots_mask, in_flight, max_in_flight, frame_clock;
    uint64_t submitted, completed, draw_calls, instances, uploaded_bytes;
    uint64_t buffer_waits, cpu_nanos, gpu_nanos;
    uint64_t scene_nanos, acquire_nanos, encode_nanos;
    uint64_t frame_requests, frame_ticks, coalesced_requests, idle_pauses;
} GDRenderStats;

// Both languages must agree on the fixed, pointer-free scene ABI.
#ifdef __cplusplus
static_assert(sizeof(float) == 4 && sizeof(int) == 4, "GD ABI requires 32-bit scalars");
static_assert(sizeof(GDRect) == 16 && sizeof(GDColor) == 16, "GD ABI geometry layout changed");
static_assert(sizeof(GDCommand) == 76 && offsetof(GDCommand, text_offset) == 60 && offsetof(GDCommand, font_offset) == 68, "GD command ABI layout changed");
static_assert(sizeof(GDRenderStats)==144 && offsetof(GDRenderStats,submitted)==24,"GD statistics ABI changed");
#else
_Static_assert(sizeof(float) == 4 && sizeof(int) == 4, "GD ABI requires 32-bit scalars");
_Static_assert(sizeof(GDRect) == 16 && sizeof(GDColor) == 16, "GD ABI geometry layout changed");
_Static_assert(sizeof(GDCommand) == 76 && offsetof(GDCommand, text_offset) == 60 && offsetof(GDCommand, font_offset) == 68, "GD command ABI layout changed");
_Static_assert(sizeof(GDRenderStats)==144 && offsetof(GDRenderStats,submitted)==24,"GD statistics ABI changed");
#endif

const char *gd_run(const char *title, float width, float height, GDColor background, int custom_titlebar);
void gd_present(const GDCommand *commands, size_t count, const char *text, size_t text_length);
void gd_measure(const char *text, size_t length, float font_size, const char *font, size_t font_length, float *width, float *height);
void gd_wake(void);
void gd_quit(void);
void gd_window_action(int action);
uint64_t gd_rendered_frames(void);
GDRenderStats gd_render_stats(void);

// Windows-only offscreen acceptance probe. Every image is copied from a D3D12
// render target after its GPU fence completes; no desktop/GDI pixels are used.
// The caller frees json and pixels with free(), including on a returned error.
typedef struct {
    uint32_t width, height, frames, stride;
    char *json;
    unsigned char *pixels;
} GDGPUProbe;
const char *gd_dx12_probe(uint32_t flags, uint32_t frames, GDGPUProbe *result);
void gd_go_event(int kind, float x, float y, int key, int modifiers);

#ifdef __cplusplus
}
#endif
#endif
