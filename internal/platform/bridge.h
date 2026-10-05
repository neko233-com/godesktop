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
} GDCommand;

// Both languages must agree on the fixed, pointer-free scene ABI.
#ifdef __cplusplus
static_assert(sizeof(float) == 4 && sizeof(int) == 4, "GD ABI requires 32-bit scalars");
static_assert(sizeof(GDRect) == 16 && sizeof(GDColor) == 16, "GD ABI geometry layout changed");
static_assert(sizeof(GDCommand) == 68 && offsetof(GDCommand, text_offset) == 60 && offsetof(GDCommand, text_length) == 64, "GD command ABI layout changed");
#else
_Static_assert(sizeof(float) == 4 && sizeof(int) == 4, "GD ABI requires 32-bit scalars");
_Static_assert(sizeof(GDRect) == 16 && sizeof(GDColor) == 16, "GD ABI geometry layout changed");
_Static_assert(sizeof(GDCommand) == 68 && offsetof(GDCommand, text_offset) == 60 && offsetof(GDCommand, text_length) == 64, "GD command ABI layout changed");
#endif

const char *gd_run(const char *title, float width, float height, GDColor background);
void gd_present(const GDCommand *commands, size_t count, const char *text, size_t text_length);
void gd_measure(const char *text, size_t length, float font_size, float *width, float *height);
void gd_wake(void);
void gd_quit(void);
uint64_t gd_rendered_frames(void);
void gd_go_event(int kind, float x, float y, int key, int modifiers);

#ifdef __cplusplus
}
#endif
#endif
