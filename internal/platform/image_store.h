#ifndef GODESKTOP_IMAGE_STORE_H
#define GODESKTOP_IMAGE_STORE_H

#include "bridge.h"
#include <stdlib.h>
#include <string.h>

// UI-thread CPU residency is shared by scene rebuilds/device recovery. Native
// textures retain their own GPU lifetime. No stored pointer refers to Go memory.
#define GDImageLimit 128
#define GDImageByteLimit (64u*1024u*1024u)
typedef struct { uint64_t id,used; uint32_t width,height; size_t bytes; unsigned char *pixels; } GDImage;
static GDImage gd_images[GDImageLimit];
static uint64_t gd_image_clock,gd_required_images[GDImageLimit];
static size_t gd_image_bytes,gd_required_count;

static const GDImage *gd_image_get(uint64_t id) {
    for(size_t i=0;i<GDImageLimit;i++) if(gd_images[i].id==id && gd_images[i].pixels) return &gd_images[i];
    return NULL;
}
static size_t gd_image_entries(void) {size_t count=0;for(size_t i=0;i<GDImageLimit;i++) if(gd_images[i].pixels) count++;return count;}
static int gd_image_required(uint64_t id) {
    for(size_t i=0;i<gd_required_count;i++) if(gd_required_images[i]==id) return 1;
    return 0;
}
static void gd_image_remove(size_t slot) {
    gd_image_bytes-=gd_images[slot].bytes; free(gd_images[slot].pixels);
    memset(&gd_images[slot],0,sizeof(GDImage));
}
static void gd_images_clear(void) {
    for(size_t i=0;i<GDImageLimit;i++) gd_image_remove(i);
    gd_required_count=0;gd_image_clock=0;
}
const char *gd_images_begin(const uint64_t *ids,size_t count) {
    if(count>GDImageLimit || (count && !ids)) return "Image frame exceeds the 128-bitmap budget";
    gd_required_count=count;
    if(count) memcpy(gd_required_images,ids,count*sizeof(uint64_t));
    return NULL;
}
int gd_image_has(uint64_t id) {
    for(size_t i=0;i<GDImageLimit;i++) if(gd_images[i].id==id && gd_images[i].pixels) {gd_images[i].used=++gd_image_clock;return 1;}
    return 0;
}
const char *gd_image_put(uint64_t id,uint32_t width,uint32_t height,const unsigned char *pixels,size_t bytes) {
    if(!id || !width || !height || width>4096 || height>4096 || bytes!=(size_t)width*height*4 || bytes>GDImageByteLimit || !pixels) return "Invalid premultiplied RGBA image";
    const GDImage *previous=gd_image_get(id);
    if(previous) return previous->width==width && previous->height==height ? NULL : "Image identity has different dimensions";
    size_t empty=GDImageLimit;
    for(;;) {
        empty=GDImageLimit;
        for(size_t i=0;i<GDImageLimit;i++) if(!gd_images[i].pixels) {empty=i;break;}
        if(empty<GDImageLimit && gd_image_bytes+bytes<=GDImageByteLimit) break;
        size_t victim=GDImageLimit;
        for(size_t i=0;i<GDImageLimit;i++) if(gd_images[i].pixels && !gd_image_required(gd_images[i].id) && (victim==GDImageLimit || gd_images[i].used<gd_images[victim].used)) victim=i;
        if(victim==GDImageLimit) return "Visible images exceed the 64 MiB / 128-bitmap residency budget";
        gd_image_remove(victim);
    }
    unsigned char *owned=(unsigned char *)malloc(bytes);
    if(!owned) return "Native image copy allocation failed";
    memcpy(owned,pixels,bytes);
    gd_images[empty]=(GDImage){id,++gd_image_clock,width,height,bytes,owned};
    gd_image_bytes+=bytes;
    return NULL;
}
#endif
