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

const char *gd_run(const char *title, float width, float height, GDColor background);
void gd_present(const GDCommand *commands, size_t count, const char *text, size_t text_length);
void gd_measure(const char *text, size_t length, float font_size, float *width, float *height);
void gd_wake(void);
void gd_quit(void);
void gd_go_event(int kind, float x, float y, int key, int modifiers);

#ifdef __cplusplus
}
#endif
#endif
