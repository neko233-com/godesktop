//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <MetalKit/MetalKit.h>
#import <CoreText/CoreText.h>
#import <QuartzCore/QuartzCore.h>
#include <math.h>
#include <ctype.h>
#include <stdlib.h>
#include <string.h>
#include <stdatomic.h>
#include <stdbool.h>
#include <time.h>
#include "bridge.h"
#include "gpu_scene.h"
#include "gpu_shader_metal.h"
#include "gpu_glyphs_metal.h"

@interface GDFrameSlot : NSObject { @public _Atomic bool busy; }
@property(nonatomic,strong) id<MTLBuffer> instances;
@property(nonatomic) NSUInteger capacity;
@property(nonatomic,strong) id<MTLBuffer> readback;
@property(nonatomic) NSUInteger readbackCapacity;
@end
@implementation GDFrameSlot
- (instancetype)init {
    self=[super init];
    if(self) atomic_init(&busy,false);
    return self;
}
@end

@interface GDMetalSnapshot : NSObject
@property(nonatomic,strong) NSData *pixels;
@property(nonatomic) uint32_t width,height,stride;
@property(nonatomic) uint64_t frame;
@end
@implementation GDMetalSnapshot
@end
static NSLock *snapshot_lock;
static GDMetalSnapshot *last_snapshot;

@interface GDView : MTKView <MTKViewDelegate>
@property(nonatomic,strong) id<MTLCommandQueue> queue;
@property(nonatomic,strong) id<MTLRenderPipelineState> pipeline;
@property(nonatomic,strong) NSArray<GDFrameSlot *> *slots;
@property(nonatomic) NSUInteger nextSlot;
@property(nonatomic) BOOL deferred;
@property(nonatomic) BOOL frameDirty;
@property(nonatomic) BOOL modernFrameClock;
// The availability-qualified driver owns the Metal display link on macOS 14+.
@property(nonatomic,strong) id frameClock;
@property(nonatomic,strong) dispatch_group_t outstanding;
@property(nonatomic,strong) GDGlyphAtlas *atlas;
@property(nonatomic,strong) NSMutableDictionary<NSArray *,id> *layouts;
@property(nonatomic,strong) NSData *scene;
@property(nonatomic,strong) NSData *text;
@property(nonatomic) GDColor background;
@property(nonatomic) BOOL readback;
@property(atomic,copy) NSString *failure;
- (void)requestFrame;
- (void)pauseFrameClock;
- (void)startFrameClock;
- (void)stopFrameClock;
- (void)renderDrawable:(id<CAMetalDrawable>)drawable;
- (void)publishGlyphStats;
@end

API_AVAILABLE(macos(14.0))
@interface GDMetalFrameClock : NSObject <CAMetalDisplayLinkDelegate>
@property(nonatomic,weak) GDView *view;
@property(nonatomic,strong) CAMetalDisplayLink *link;
@end

@interface GDDelegate : NSObject <NSApplicationDelegate,NSWindowDelegate>
@property(nonatomic,strong) NSWindow *window;
@end

static GDView *active_view;
static BOOL running;
static _Atomic uint64_t generation;
static _Atomic uint64_t rendered_frames;
static _Atomic uint64_t submitted_frames, draw_calls, instance_count, uploaded_bytes, cpu_nanos, gpu_nanos;
static _Atomic uint64_t scene_nanos, acquire_nanos, encode_nanos;
static _Atomic uint32_t used_slots, in_flight, max_in_flight;
static _Atomic uint32_t frame_clock;
static _Atomic uint64_t frame_requests, frame_ticks, coalesced_requests, idle_pauses;
static _Atomic uint64_t glyph_rasterizations,glyph_cache_hits,glyph_cache_entries,glyph_atlas_pages;
static _Atomic uint64_t glyph_atlas_bytes,glyph_atlas_peak_bytes,glyph_atlas_epochs,glyph_uploaded_bytes;

static uint64_t nanos(void) {
    struct timespec t; clock_gettime(CLOCK_MONOTONIC,&t);
    return (uint64_t)t.tv_sec*1000000000+(uint64_t)t.tv_nsec;
}

static NSString *string_utf8(const char *bytes,size_t length) {
    return [[NSString alloc] initWithBytes:bytes length:length encoding:NSUTF8StringEncoding] ?: @"";
}
static CTLineRef text_line(NSString *text,float size,NSString *family) {
    NSFont *font=family.length?[NSFont fontWithName:family size:size]:nil;
    NSDictionary *attributes=@{NSFontAttributeName:font ?: [NSFont systemFontOfSize:size],NSForegroundColorAttributeName:[NSColor whiteColor]};
    NSAttributedString *attributed=[[NSAttributedString alloc] initWithString:text attributes:attributes];
    return CTLineCreateWithAttributedString((__bridge CFAttributedStringRef)attributed);
}
static void text_size(CTLineRef line,float *width,float *height,CGFloat *descent) {
    CGFloat ascent,leading;
    double w=CTLineGetTypographicBounds(line,&ascent,descent,&leading);
    *width=ceil(w)+2;
    *height=ceil(ascent+*descent+leading)+2;
}
@implementation GDView
- (BOOL)acceptsFirstResponder { return YES; }
- (BOOL)isFlipped { return YES; }
- (void)mouseDown:(NSEvent *)event {
    [self.window makeFirstResponder:self];
    NSPoint p=[self convertPoint:event.locationInWindow fromView:nil];
    const GDCommand *commands=self.scene.bytes;
    for(NSUInteger i=0;i<self.scene.length/sizeof(GDCommand);i++) {
        GDRect clip=commands[i].clip;
        if(commands[i].kind==4 && p.x>=clip.x && p.x<clip.x+clip.w && p.y>=clip.y && p.y<clip.y+clip.h) { [self.window performWindowDragWithEvent:event]; return; }
    }
    gd_go_event(2,p.x,p.y,0,0);
}
- (void)mouseUp:(NSEvent *)event {
    NSPoint p=[self convertPoint:event.locationInWindow fromView:nil];
    gd_go_event(3,p.x,p.y,0,0);
}
- (void)keyDown:(NSEvent *)event {
    int key=0;
    int mods=((event.modifierFlags&NSEventModifierFlagShift)?1:0)|((event.modifierFlags&NSEventModifierFlagControl)?2:0)|((event.modifierFlags&NSEventModifierFlagOption)?4:0)|((event.modifierFlags&NSEventModifierFlagCommand)?8:0);
    switch(event.keyCode) { case 48:key=9;break; case 36:case 76:key=13;break; case 49:key=32;break; case 53:key=27;break; case 51:key=8;break; case 117:key=46;break; case 123:key=37;break; case 124:key=39;break; case 125:key=40;break; case 126:key=38;break; }
    if(!key && (mods&10) && event.charactersIgnoringModifiers.length) key=toupper([event.charactersIgnoringModifiers characterAtIndex:0]);
    if(key && !event.isARepeat) gd_go_event(4,0,0,key,mods);
    if(!(mods&10)) {
        NSString *text=event.characters;
        for(NSUInteger i=0;i<text.length;i++) {
            unsigned value=[text characterAtIndex:i];
            if(value>=0xd800 && value<=0xdbff && i+1<text.length) { unsigned low=[text characterAtIndex:++i]; if(low<0xdc00 || low>0xdfff) continue; value=0x10000+((value-0xd800)<<10)+(low-0xdc00); }
            if(value>=32 && value!=127 && !(value>=0xf700 && value<=0xf8ff)) gd_go_event(6,0,0,value,mods);
        }
    }
}
- (void)scrollWheel:(NSEvent *)event { gd_go_event(7,0,event.scrollingDeltaY/(event.hasPreciseScrollingDeltas?12.0f:1.0f),0,0); }
- (void)mtkView:(MTKView *)view drawableSizeWillChange:(CGSize)size { [self requestFrame]; }
- (void)requestFrame {
    atomic_fetch_add(&frame_requests,1);
    if(self.frameDirty) atomic_fetch_add(&coalesced_requests,1);
    self.frameDirty=YES;
    if(!self.window.visible || self.window.miniaturized) return;
    if(@available(macOS 14.0,*)) {
        if(self.frameClock) { ((GDMetalFrameClock *)self.frameClock).link.paused=NO; return; }
    }
    // The macOS 13 compatibility path uses MTKView's display-synchronised loop.
    // No timer or unpaced setNeedsDisplay chain is used on either path.
    self.paused=NO;
}
- (void)pauseFrameClock {
    if(@available(macOS 14.0,*)) {
        if(self.frameClock) {
            GDMetalFrameClock *clock=self.frameClock;
            if(!clock.link.paused) { clock.link.paused=YES; atomic_fetch_add(&idle_pauses,1); }
            return;
        }
    }
    if(!self.paused) { self.paused=YES; atomic_fetch_add(&idle_pauses,1); }
}
- (void)startFrameClock {
    if(@available(macOS 14.0,*)) {
        GDMetalFrameClock *clock=[[GDMetalFrameClock alloc] init];
        clock.view=self;
        clock.link=[[CAMetalDisplayLink alloc] initWithMetalLayer:(CAMetalLayer *)self.layer];
        if(!clock.link) { [self fail:@"Metal display link allocation failed"]; return; }
        clock.link.delegate=clock;
        clock.link.preferredFrameLatency=1.0f;
        clock.link.preferredFrameRateRange=CAFrameRateRangeDefault;
        clock.link.paused=YES;
        self.frameClock=clock; self.modernFrameClock=YES;
        self.paused=YES;
        atomic_store(&frame_clock,2);
        [clock.link addToRunLoop:NSRunLoop.mainRunLoop forMode:NSRunLoopCommonModes];
    } else {
        self.preferredFramesPerSecond=MAX(1,self.window.screen.maximumFramesPerSecond);
        atomic_store(&frame_clock,1);
    }
    [self requestFrame];
}
- (void)stopFrameClock {
    if(@available(macOS 14.0,*)) {
        GDMetalFrameClock *clock=self.frameClock;
        [clock.link invalidate]; clock.link.delegate=nil; clock.view=nil;
    }
    self.frameClock=nil; self.paused=YES;
}
- (void)fail:(NSString *)message {
    self.failure=message;
    gd_quit();
}
- (CTLineRef)layout:(const GDCommand *)command {
    NSString *value=string_utf8((const char *)self.text.bytes+command->text_offset,command->text_length);
    NSString *family=string_utf8((const char *)self.text.bytes+command->font_offset,command->font_length);
    NSArray *key=@[value,family,@(command->font_size)];
    id existing=self.layouts[key];
    if(existing) return (__bridge CTLineRef)existing;
    if(self.layouts.count>=1024) [self.layouts removeAllObjects];
    CTLineRef line=text_line(value,command->font_size,family);
    if(!line) { [self fail:@"CoreText line allocation failed"]; return NULL; }
    self.layouts[key]=CFBridgingRelease(line);
    return (__bridge CTLineRef)self.layouts[key];
}
- (GDGlyphScene *)buildGlyphScene:(CGFloat)scale {
    for(NSUInteger attempt=0;attempt<2;attempt++) {
        GDGlyphScene *scene=[[GDGlyphScene alloc] initWithWhite:[self.atlas white]];
        const GDCommand *commands=self.scene.bytes;
        for(NSUInteger i=0;i<self.scene.length/sizeof(GDCommand);i++) {
            const GDCommand *command=&commands[i];
            if(command->kind==4 || command->clip.w<=0 || command->clip.h<=0) continue;
            BOOL success;
            if(command->kind==2) {
                if(!command->text_length) continue;
                CTLineRef line=[self layout:command];
                if(!line) return nil;
                success=[scene appendLine:line command:command atlas:self.atlas scale:scale];
            } else success=[scene append:gd_gpu_instance(command) page:nil];
            if(!success) {
                if(self.atlas.full) break;
                [self fail:scene.failure ?: self.atlas.failure ?: @"CoreText glyph scene failed"];
                return nil;
            }
        }
        if(!self.atlas.full) return scene;
        scene=nil; [self.atlas clear];
    }
    [self fail:@"One frame exceeds the 16 MiB / 16384 glyph atlas budget"]; return nil;
}
- (void)publishGlyphStats {
    GDGlyphAtlas *atlas=self.atlas;
    uint64_t pages=atomic_load(&atlas.usage->pages);
    atomic_store(&glyph_rasterizations,atlas.rasterized); atomic_store(&glyph_cache_hits,atlas.hits);
    atomic_store(&glyph_cache_entries,atlas.glyphs.count); atomic_store(&glyph_atlas_pages,pages);
    atomic_store(&glyph_atlas_bytes,pages*GDAtlasEdge*GDAtlasEdge);
    atomic_store(&glyph_atlas_peak_bytes,atomic_load(&atlas.usage->peak)*GDAtlasEdge*GDAtlasEdge);
    atomic_store(&glyph_atlas_epochs,atlas.epochs); atomic_store(&glyph_uploaded_bytes,atlas.uploadedBytes);
}
- (void)drawInMTKView:(MTKView *)view {
    // MTKView must never obtain a second drawable while the modern driver
    // already supplied one. This delegate is only the macOS 13 path.
    if(self.modernFrameClock) return;
    if(!running || active_view!=self) return;
    atomic_fetch_add(&frame_ticks,1);
    if(!self.frameDirty || !self.window.visible || self.window.miniaturized) { [self pauseFrameClock]; return; }
    self.frameDirty=NO;
    [self renderDrawable:nil];
}
- (void)renderDrawable:(id<CAMetalDrawable>)supplied {
    @autoreleasepool {
        if(self.bounds.size.width<=0 || self.bounds.size.height<=0) return;
        GDFrameSlot *slot=nil;
        NSUInteger slotIndex=0;
        for(NSUInteger i=0;i<self.slots.count;i++) {
            NSUInteger index=(self.nextSlot+i)%self.slots.count;
            bool expected=false;
            if(atomic_compare_exchange_strong(&self.slots[index]->busy,&expected,true)) {
                slot=self.slots[index]; slotIndex=index; self.nextSlot=(index+1)%self.slots.count; break;
            }
        }
        // Saturation defers the repaint. The GPU completion callback requests it
        // again; the AppKit thread never waits for an in-flight upload buffer.
        if(!slot) { self.deferred=YES; self.frameDirty=YES; return; }
        self.deferred=NO;
        uint64_t started=nanos();
        gd_go_event(1,self.bounds.size.width,self.bounds.size.height,0,0);
        if(self.failure) { atomic_store(&slot->busy,false); return; }
        uint64_t sceneEnd=nanos();
        id<CAMetalDrawable> drawable=supplied;
        MTLRenderPassDescriptor *pass=nil;
        if(drawable) {
            pass=[MTLRenderPassDescriptor renderPassDescriptor];
            pass.colorAttachments[0].texture=drawable.texture;
            pass.colorAttachments[0].loadAction=MTLLoadActionClear;
            pass.colorAttachments[0].storeAction=MTLStoreActionStore;
        } else {
            pass=self.currentRenderPassDescriptor;
            drawable=self.currentDrawable;
        }
        uint64_t acquireEnd=nanos();
        if(!pass || !drawable) {
            atomic_store(&slot->busy,false); self.deferred=YES; self.frameDirty=YES;
            return;
        }
        CGFloat scale=drawable.texture.width/self.bounds.size.width;
        GDGlyphScene *native=[self buildGlyphScene:scale];
        if(!native) { atomic_store(&slot->busy,false); return; }
        NSUInteger needed=MAX(sizeof(GDGPUInstance),native.instances.length);
        if(needed>slot.capacity) {
            slot.capacity=MIN(16*1024*1024,MAX(needed,slot.capacity*2+4096));
            slot.instances=[self.device newBufferWithLength:slot.capacity options:MTLResourceStorageModeShared|MTLResourceCPUCacheModeWriteCombined];
            if(!slot.instances) { atomic_store(&slot->busy,false); [self fail:@"Metal instance allocation failed"]; return; }
        }
        if(native.instances.length) memcpy(slot.instances.contents,native.instances.bytes,native.instances.length);
        uint64_t nativeSceneEnd=nanos();
        NSUInteger encoded=native.instances.length/sizeof(GDGPUInstance),batchCount=native.batches.length/sizeof(GDBatch);
        const GDBatch *batches=native.batches.bytes;
        GDColor bg=self.background;
        pass.colorAttachments[0].clearColor=MTLClearColorMake(bg.r,bg.g,bg.b,bg.a);
        id<MTLCommandBuffer> buffer=[self.queue commandBuffer];
        if(!buffer) { atomic_store(&slot->busy,false); [self fail:@"Metal command buffer allocation failed"]; return; }
        __block NSArray<GDAtlasPage *> *heldPages=[native.pages copy];
        __block NSArray<id<MTLBuffer>> *heldUploads=[self.atlas encodePages:heldPages buffer:buffer];
        if(!heldUploads) { atomic_store(&slot->busy,false); [self fail:self.atlas.failure]; return; }
        id<MTLRenderCommandEncoder> encoder=[buffer renderCommandEncoderWithDescriptor:pass];
        if(!encoder) { atomic_store(&slot->busy,false); [self fail:@"Metal command encoding failed"]; return; }
        [encoder setRenderPipelineState:self.pipeline];
        float viewport[2]={self.bounds.size.width,self.bounds.size.height};
        [encoder setVertexBytes:viewport length:sizeof(viewport) atIndex:1];
        for(NSUInteger i=0;i<batchCount;i++) {
            GDBatch batch=batches[i];
            [encoder setVertexBuffer:slot.instances offset:batch.start*sizeof(GDGPUInstance) atIndex:0];
            [encoder setFragmentTexture:heldPages[batch.texture].texture atIndex:0];
            [encoder drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:6 instanceCount:batch.count];
        }
        [encoder endEncoding];
        id<MTLBuffer> readback=nil;
        NSUInteger readbackPitch=0;
        uint32_t readbackWidth=0,readbackHeight=0;
        if(self.readback) {
            readbackWidth=(uint32_t)drawable.texture.width; readbackHeight=(uint32_t)drawable.texture.height;
            readbackPitch=((NSUInteger)readbackWidth*4+255)&~(NSUInteger)255;
            NSUInteger bytes=readbackPitch*readbackHeight;
            if(bytes>64*1024*1024) { atomic_store(&slot->busy,false); [self fail:@"GPU diagnostic capture exceeds 64 MiB"]; return; }
            if(bytes>slot.readbackCapacity) {
                slot.readback=[self.device newBufferWithLength:bytes options:MTLResourceStorageModeShared]; slot.readbackCapacity=bytes;
            }
            readback=slot.readback;
            id<MTLBlitCommandEncoder> copy=[buffer blitCommandEncoder];
            if(!readback || !copy) { atomic_store(&slot->busy,false); [self fail:@"Metal drawable readback allocation failed"]; return; }
            [copy copyFromTexture:drawable.texture sourceSlice:0 sourceLevel:0 sourceOrigin:MTLOriginMake(0,0,0) sourceSize:MTLSizeMake(readbackWidth,readbackHeight,1) toBuffer:readback destinationOffset:0 destinationBytesPerRow:readbackPitch destinationBytesPerImage:bytes];
            [copy endEncoding];
        }
        dispatch_group_enter(self.outstanding);
        uint32_t flight=atomic_fetch_add(&in_flight,1)+1;
        uint32_t peak=atomic_load(&max_in_flight);
        while(flight>peak && !atomic_compare_exchange_weak(&max_in_flight,&peak,flight)) {}
        atomic_fetch_or(&used_slots,(uint32_t)(1u<<slotIndex));
        atomic_store(&draw_calls,batchCount); atomic_store(&instance_count,encoded);
        atomic_store(&uploaded_bytes,encoded*sizeof(GDGPUInstance));
        uint64_t expectedGeneration=atomic_load(&generation);
        uint64_t serial=atomic_load(&submitted_frames)+1;
        [buffer addCompletedHandler:^(id<MTLCommandBuffer> completed) {
            BOOL current=expectedGeneration==atomic_load(&generation);
            if(current && completed.status==MTLCommandBufferStatusError) {
                NSString *message=completed.error.localizedDescription ?: @"Metal submission failed";
                self.failure=message;
                dispatch_async(dispatch_get_main_queue(),^{ if(running && active_view==self) gd_quit(); });
            } else if(current) {
                if(readback) {
                    NSMutableData *pixels=[NSMutableData dataWithLength:(NSUInteger)readbackWidth*readbackHeight*4];
                    for(NSUInteger row=0;row<readbackHeight;row++) memcpy((unsigned char *)pixels.mutableBytes+row*readbackWidth*4,(const unsigned char *)readback.contents+row*readbackPitch,readbackWidth*4);
                    GDMetalSnapshot *snapshot=[[GDMetalSnapshot alloc] init];
                    snapshot.pixels=pixels; snapshot.width=readbackWidth; snapshot.height=readbackHeight; snapshot.stride=readbackWidth*4; snapshot.frame=serial;
                    [snapshot_lock lock];
                    if(!last_snapshot || last_snapshot.frame<serial) last_snapshot=snapshot;
                    [snapshot_lock unlock];
                }
                atomic_fetch_add(&rendered_frames,1);
                double elapsed=completed.GPUEndTime-completed.GPUStartTime;
                if(elapsed>0) atomic_store(&gpu_nanos,(uint64_t)(elapsed*1000000000));
            }
            if(current) atomic_fetch_sub(&in_flight,1);
            heldPages=nil; heldUploads=nil;
            atomic_store(&slot->busy,false);
            dispatch_group_leave(self.outstanding);
            dispatch_async(dispatch_get_main_queue(),^{
                if(running && active_view==self && self.deferred) { self.deferred=NO; [self requestFrame]; }
            });
        }];
        atomic_fetch_add(&submitted_frames,1);
        if(!self.modernFrameClock) [buffer presentDrawable:drawable];
        [buffer commit];
        // CAMetalDisplayLink supplied the drawable and its presentation timing.
        // Commit first, then present without an explicit time or GPU wait.
        if(self.modernFrameClock) [drawable present];
        uint64_t finished=nanos();
        atomic_store(&cpu_nanos,finished-started);
        atomic_store(&scene_nanos,(sceneEnd-started)+(nativeSceneEnd-acquireEnd));
        atomic_store(&acquire_nanos,acquireEnd-sceneEnd);
        atomic_store(&encode_nanos,finished-nativeSceneEnd);
        [self publishGlyphStats];
    }
}
@end

@implementation GDMetalFrameClock
- (void)metalDisplayLink:(CAMetalDisplayLink *)link needsUpdate:(CAMetalDisplayLinkUpdate *)update {
    GDView *view=self.view;
    if(!running || active_view!=view || !view) return;
    atomic_fetch_add(&frame_ticks,1);
    if(!view.frameDirty || !view.window.visible || view.window.miniaturized) { [view pauseFrameClock]; return; }
    view.frameDirty=NO;
    [view renderDrawable:update.drawable];
}
@end

@implementation GDDelegate
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender { return NO; }
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender { [self.window close]; return NSTerminateCancel; }
- (void)windowDidResignKey:(NSNotification *)notification { gd_go_event(5,0,0,0,0); }
- (void)windowDidDeminiaturize:(NSNotification *)notification { [active_view requestFrame]; }
- (void)windowDidBecomeKey:(NSNotification *)notification { [active_view requestFrame]; }
- (void)windowDidChangeBackingProperties:(NSNotification *)notification { [active_view requestFrame]; }
- (void)windowWillClose:(NSNotification *)notification {
    [active_view stopFrameClock];
    [NSApp stop:nil];
    // stop: sets a flag; a posted event also wakes nextEventMatchingMask:.
    NSEvent *wake=[NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
    [NSApp postEvent:wake atStart:YES];
}
@end

const char *gd_run(const char *title,float width,float height,GDColor background,int custom_titlebar) {
    static char *last_error;
    free(last_error); last_error=NULL;
    atomic_store(&rendered_frames,0);
    atomic_store(&submitted_frames,0); atomic_store(&draw_calls,0); atomic_store(&instance_count,0);
    atomic_store(&uploaded_bytes,0); atomic_store(&cpu_nanos,0); atomic_store(&gpu_nanos,0);
    atomic_store(&scene_nanos,0); atomic_store(&acquire_nanos,0); atomic_store(&encode_nanos,0);
    atomic_store(&used_slots,0); atomic_store(&in_flight,0); atomic_store(&max_in_flight,0);
    atomic_store(&frame_clock,0); atomic_store(&frame_requests,0); atomic_store(&frame_ticks,0);
    atomic_store(&coalesced_requests,0); atomic_store(&idle_pauses,0);
    atomic_store(&glyph_rasterizations,0); atomic_store(&glyph_cache_hits,0); atomic_store(&glyph_cache_entries,0); atomic_store(&glyph_atlas_pages,0);
    atomic_store(&glyph_atlas_bytes,0); atomic_store(&glyph_atlas_peak_bytes,0); atomic_store(&glyph_atlas_epochs,0); atomic_store(&glyph_uploaded_bytes,0);
    if(![NSThread isMainThread]) return "AppKit must run on the process main thread; call godesktop.Run from main";
    @autoreleasepool {
        if(!snapshot_lock) snapshot_lock=[[NSLock alloc] init];
        [snapshot_lock lock]; last_snapshot=nil; [snapshot_lock unlock];
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        id<MTLDevice> device=MTLCreateSystemDefaultDevice();
        if(!device) return "No Metal device is available";
        NSError *error=nil;
        id<MTLLibrary> library=[device newLibraryWithSource:shader options:nil error:&error];
        if(!library) { last_error=strdup(error.localizedDescription.UTF8String); return last_error; }
        MTLRenderPipelineDescriptor *pipeline=[[MTLRenderPipelineDescriptor alloc] init];
        pipeline.vertexFunction=[library newFunctionWithName:@"vertex_main"];
        pipeline.fragmentFunction=[library newFunctionWithName:@"fragment_main"];
        pipeline.colorAttachments[0].pixelFormat=MTLPixelFormatBGRA8Unorm;
        pipeline.colorAttachments[0].blendingEnabled=YES;
        pipeline.colorAttachments[0].sourceRGBBlendFactor=MTLBlendFactorOne;
        pipeline.colorAttachments[0].destinationRGBBlendFactor=MTLBlendFactorOneMinusSourceAlpha;
        pipeline.colorAttachments[0].sourceAlphaBlendFactor=MTLBlendFactorOne;
        pipeline.colorAttachments[0].destinationAlphaBlendFactor=MTLBlendFactorOneMinusSourceAlpha;
        id<MTLRenderPipelineState> state=[device newRenderPipelineStateWithDescriptor:pipeline error:&error];
        if(!state) { last_error=strdup(error.localizedDescription.UTF8String); return last_error; }
        GDView *view=[[GDView alloc] initWithFrame:NSMakeRect(0,0,width,height) device:device];
        view.pipeline=state; view.queue=[device newCommandQueue];
        if(!view.queue) return "Metal command queue allocation failed";
        view.slots=@[[[GDFrameSlot alloc] init],[[GDFrameSlot alloc] init],[[GDFrameSlot alloc] init]];
        view.outstanding=dispatch_group_create();
        view.background=background;
        const char *readback=getenv("GODESKTOP_READBACK");
        view.readback=readback && strcmp(readback,"1")==0;
        view.framebufferOnly=!view.readback;
        view.atlas=[[GDGlyphAtlas alloc] initWithDevice:device]; view.layouts=[NSMutableDictionary dictionary];
        view.scene=[NSData data]; view.text=[NSData data];
        view.colorPixelFormat=MTLPixelFormatBGRA8Unorm;
        view.paused=YES; view.enableSetNeedsDisplay=NO; view.delegate=view;
        GDDelegate *delegate=[[GDDelegate alloc] init];
        NSWindowStyleMask style=NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable;
        if(custom_titlebar) style|=NSWindowStyleMaskFullSizeContentView;
        NSWindow *window=[[NSWindow alloc] initWithContentRect:view.frame styleMask:style backing:NSBackingStoreBuffered defer:NO];
        if(custom_titlebar) { window.titleVisibility=NSWindowTitleHidden; window.titlebarAppearsTransparent=YES; }
        window.releasedWhenClosed=NO;
        window.title=string_utf8(title,strlen(title)); window.contentView=view; window.delegate=delegate;
        delegate.window=window; NSApp.delegate=delegate;
        NSMenu *menu=[[NSMenu alloc] init];
        NSMenuItem *appItem=[[NSMenuItem alloc] init];
        NSMenu *appMenu=[[NSMenu alloc] initWithTitle:@"godesktop"];
        [appMenu addItemWithTitle:@"Quit godesktop" action:@selector(terminate:) keyEquivalent:@"q"];
        appItem.submenu=appMenu; [menu addItem:appItem]; NSApp.mainMenu=menu;
        atomic_fetch_add(&generation,1);
        active_view=view; running=YES;
        [window center]; [window makeKeyAndOrderFront:nil]; [window makeFirstResponder:view];
        [NSApp activateIgnoringOtherApps:YES]; [view startFrameClock];
        [NSApp run];
        running=NO;
        [view stopFrameClock];
        // Only shutdown drains the queue. Normal frames never wait for the GPU.
        if(dispatch_group_wait(view.outstanding,dispatch_time(DISPATCH_TIME_NOW,5*NSEC_PER_SEC))) view.failure=@"Metal shutdown did not finish within five seconds";
        [view publishGlyphStats];
        active_view=nil;
        atomic_fetch_add(&generation,1);
        window.delegate=nil; [window orderOut:nil]; [window close];
        NSApp.delegate=nil; view.delegate=nil;
        if(view.failure) last_error=strdup(view.failure.UTF8String);
    }
    return last_error;
}

void gd_present(const GDCommand *commands,size_t count,const char *text,size_t length) {
    active_view.scene=count?[NSData dataWithBytes:commands length:count*sizeof(GDCommand)]:[NSData data];
    active_view.text=length?[NSData dataWithBytes:text length:length]:[NSData data];
}
void gd_measure(const char *text,size_t length,float size,const char *font,size_t font_length,float *width,float *height) {
    CTLineRef line=text_line(string_utf8(text,length),size,string_utf8(font,font_length));
    CGFloat descent;
    text_size(line,width,height,&descent);
    CFRelease(line);
}
void gd_wake(void) {
    uint64_t expected=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{ if(running && expected==atomic_load(&generation)) [active_view requestFrame]; });
}
void gd_quit(void) {
    uint64_t expected=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{ if(running && expected==atomic_load(&generation)) [active_view.window close]; });
}
uint64_t gd_rendered_frames(void) { return atomic_load(&rendered_frames); }
const char *gd_metal_snapshot(GDGPUSnapshot *result) {
    memset(result,0,sizeof(*result));
    [snapshot_lock lock]; GDMetalSnapshot *snapshot=last_snapshot; [snapshot_lock unlock];
    if(!snapshot) return "No completed Metal drawable readback; enable GODESKTOP_READBACK=1";
    result->width=snapshot.width; result->height=snapshot.height; result->stride=snapshot.stride;
    result->frame=snapshot.frame; result->bytes=snapshot.pixels.length;
    result->pixels=malloc(result->bytes);
    if(!result->pixels) return "Metal snapshot copy allocation failed";
    memcpy(result->pixels,snapshot.pixels.bytes,result->bytes); return NULL;
}
const char *gd_metal_text_reference(const char *text,size_t length,const char *font,size_t font_length,float size,float scale,uint32_t width,uint32_t height,GDGPUSnapshot *result) {
    memset(result,0,sizeof(*result));
    if(!width || !height || (uint64_t)width*height*4>64*1024*1024 || !isfinite(scale) || scale<=0) return "Invalid text reference bounds";
    result->width=width; result->height=height; result->stride=width*4; result->bytes=(uint64_t)width*height*4;
    result->pixels=calloc(1,result->bytes);
    if(!result->pixels) return "Text reference allocation failed";
    CGColorSpaceRef space=CGColorSpaceCreateDeviceRGB();
    CGContextRef context=CGBitmapContextCreate(result->pixels,width,height,8,width*4,space,kCGImageAlphaPremultipliedLast|kCGBitmapByteOrder32Big);
    CGColorSpaceRelease(space);
    if(!context) return "Text reference bitmap context allocation failed";
    CTLineRef line=text_line(string_utf8(text,length),size,string_utf8(font,font_length));
    if(!line) { CGContextRelease(context); return "CoreText reference line allocation failed"; }
    CGFloat ascent,descent,leading;
    CTLineGetTypographicBounds(line,&ascent,&descent,&leading);
    CGContextSetAllowsFontSmoothing(context,false); CGContextSetShouldSmoothFonts(context,false);
    CGContextScaleCTM(context,scale,scale);
    CGContextSetTextPosition(context,1,height/scale-ceil(ascent+leading)-1);
    CTLineDraw(line,context);
    CFRelease(line); CGContextRelease(context); return NULL;
}
GDRenderStats gd_render_stats(void) {
    return (GDRenderStats){
        .backend=2,.frame_slots=3,.used_slots_mask=atomic_load(&used_slots),
        .frame_clock=atomic_load(&frame_clock),
        .in_flight=atomic_load(&in_flight),.max_in_flight=atomic_load(&max_in_flight),
        .submitted=atomic_load(&submitted_frames),.completed=atomic_load(&rendered_frames),
        .draw_calls=atomic_load(&draw_calls),.instances=atomic_load(&instance_count),
        .uploaded_bytes=atomic_load(&uploaded_bytes),.cpu_nanos=atomic_load(&cpu_nanos),.gpu_nanos=atomic_load(&gpu_nanos),
        .scene_nanos=atomic_load(&scene_nanos),.acquire_nanos=atomic_load(&acquire_nanos),.encode_nanos=atomic_load(&encode_nanos),
        .frame_requests=atomic_load(&frame_requests),.frame_ticks=atomic_load(&frame_ticks),
        .coalesced_requests=atomic_load(&coalesced_requests),.idle_pauses=atomic_load(&idle_pauses),
        .glyph_rasterizations=atomic_load(&glyph_rasterizations),.glyph_cache_hits=atomic_load(&glyph_cache_hits),.glyph_cache_entries=atomic_load(&glyph_cache_entries),
        .glyph_atlas_pages=atomic_load(&glyph_atlas_pages),.glyph_atlas_bytes=atomic_load(&glyph_atlas_bytes),.glyph_atlas_peak_bytes=atomic_load(&glyph_atlas_peak_bytes),
        .glyph_atlas_epochs=atomic_load(&glyph_atlas_epochs),.glyph_uploaded_bytes=atomic_load(&glyph_uploaded_bytes)
    };
}

void gd_window_action(int action) {
    uint64_t expected=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{ if(!running || expected!=atomic_load(&generation)) return; if(action==1) [active_view.window miniaturize:nil]; if(action==2) [active_view.window zoom:nil]; });
}
