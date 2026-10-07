#ifndef GODESKTOP_GPU_SCENE_H
#define GODESKTOP_GPU_SCENE_H

#include "bridge.h"

// One instance produces its six triangle vertices in the GPU. Both Metal and
// Direct3D 12 shaders use this pointer-free, tightly packed 160-byte layout.
typedef struct {
    GDRect bounds, clip;
    GDColor color;
    GDRect uv;
    float radius;
    uint32_t kind;
    float padding[2];
    GDRect rounded_bounds[4];
    float rounded_radii[4];
} GDGPUInstance;

#ifdef __cplusplus
static_assert(sizeof(GDGPUInstance)==160 && offsetof(GDGPUInstance,kind)==68 && offsetof(GDGPUInstance,rounded_bounds)==80,"GPU instance ABI changed");
#else
_Static_assert(sizeof(GDGPUInstance)==160 && offsetof(GDGPUInstance,kind)==68 && offsetof(GDGPUInstance,rounded_bounds)==80,"GPU instance ABI changed");
#endif

static inline GDGPUInstance gd_gpu_instance(const GDCommand *command) {
    GDGPUInstance result={0};
    result.bounds=command->bounds; result.clip=command->clip;
    result.color=command->color; result.radius=command->radius;
    result.kind=(uint32_t)command->kind;
    result.uv.w=1; result.uv.h=1;
    for(int i=0;i<4;i++) { result.rounded_bounds[i]=command->rounded_bounds[i]; result.rounded_radii[i]=command->rounded_radii[i]; }
    return result;
}
#endif
