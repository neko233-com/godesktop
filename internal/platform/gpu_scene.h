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
    if(command->kind==7) {
        // One quad covers a truncated Gaussian mask blur. The unused texture
        // rectangle carries its caster, and padding[1] carries DIP sigma.
        // padding[0] remains the glyph texture slot; neither ABI grows.
        const float sigma=command->font_size>0?command->font_size:0;
        const float extent=4*sigma+1;
        result.uv=command->bounds;
        result.padding[1]=sigma;
        result.bounds.x-=extent; result.bounds.y-=extent;
        result.bounds.w+=2*extent; result.bounds.h+=2*extent;
    }
    for(int i=0;i<4;i++) { result.rounded_bounds[i]=command->rounded_bounds[i]; result.rounded_radii[i]=command->rounded_radii[i]; }
    return result;
}
#endif
