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
#include "image_store.h"
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
// Each Run owns its counters and snapshots. A timed-out old completion retains
// only its old state and cannot overwrite a subsequently started Run's state.
@interface GDRunState : NSObject { @public
    _Atomic uint64_t rendered_frames,submitted_frames,draw_calls,instance_count,uploaded_bytes,cpu_nanos,gpu_nanos;
    _Atomic uint64_t scene_nanos,acquire_nanos,encode_nanos;
    _Atomic uint32_t used_slots,in_flight,max_in_flight,frame_clock;
    _Atomic uint64_t frame_requests,frame_ticks,coalesced_requests,idle_pauses;
    _Atomic uint64_t glyph_rasterizations,glyph_cache_hits,glyph_cache_entries,glyph_atlas_pages;
    _Atomic uint64_t glyph_atlas_bytes,glyph_atlas_peak_bytes,glyph_atlas_epochs,glyph_uploaded_bytes;
    _Atomic uint64_t device_recoveries,dropped_frames,diagnostic_recovery_remaining;
    _Atomic uint64_t bitmap_cache_entries,bitmap_cache_bytes,bitmap_uploads,bitmap_uploaded_bytes;
    uint64_t diagnostic_recovery_interval;
}
@property(nonatomic,strong) NSLock *snapshotLock;
@property(nonatomic,strong) GDMetalSnapshot *snapshot;
@end
@implementation GDRunState
- (instancetype)init {
    self=[super init];
    if(self) {
        atomic_init(&rendered_frames,0); atomic_init(&submitted_frames,0); atomic_init(&draw_calls,0); atomic_init(&instance_count,0);
        atomic_init(&uploaded_bytes,0); atomic_init(&cpu_nanos,0); atomic_init(&gpu_nanos,0);
        atomic_init(&scene_nanos,0); atomic_init(&acquire_nanos,0); atomic_init(&encode_nanos,0);
        atomic_init(&used_slots,0); atomic_init(&in_flight,0); atomic_init(&max_in_flight,0); atomic_init(&frame_clock,0);
        atomic_init(&frame_requests,0); atomic_init(&frame_ticks,0); atomic_init(&coalesced_requests,0); atomic_init(&idle_pauses,0);
        atomic_init(&glyph_rasterizations,0); atomic_init(&glyph_cache_hits,0); atomic_init(&glyph_cache_entries,0); atomic_init(&glyph_atlas_pages,0);
        atomic_init(&glyph_atlas_bytes,0); atomic_init(&glyph_atlas_peak_bytes,0); atomic_init(&glyph_atlas_epochs,0); atomic_init(&glyph_uploaded_bytes,0);
        atomic_init(&device_recoveries,0); atomic_init(&dropped_frames,0); atomic_init(&diagnostic_recovery_remaining,0);
        atomic_init(&bitmap_cache_entries,0);atomic_init(&bitmap_cache_bytes,0);atomic_init(&bitmap_uploads,0);atomic_init(&bitmap_uploaded_bytes,0);
        self.snapshotLock=[[NSLock alloc] init];
    }
    return self;
}
@end
static GDRunState *latest_metrics;
static NSLock *metrics_lock;
static NSLock *state_lock(void) {
    static dispatch_once_t once;
    dispatch_once(&once,^{ metrics_lock=[[NSLock alloc] init]; });
    return metrics_lock;
}
static GDRunState *current_metrics(void) {
    NSLock *lock=state_lock(); [lock lock]; GDRunState *metrics=latest_metrics; [lock unlock]; return metrics;
}

@interface GDView : MTKView <MTKViewDelegate> { @public _Atomic bool recoveryRequested; }
@property(nonatomic,strong) id<MTLCommandQueue> queue;
@property(nonatomic,strong) GDRunState *metrics;
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
@property(nonatomic,strong) NSMutableDictionary<NSNumber *,GDAtlasPage *> *bitmaps;
@property(nonatomic,strong) NSMutableDictionary<NSArray *,id> *layouts;
@property(nonatomic,strong) NSData *scene;
@property(nonatomic,strong) NSData *text;
@property(nonatomic) GDColor background;
@property(nonatomic) BOOL readback;
@property(atomic,copy) NSString *failure;
@property(nonatomic,strong) id<NSObject> deviceObserver;
- (void)requestFrame;
- (void)pauseFrameClock;
- (void)startFrameClock;
- (void)stopFrameClock;
- (void)renderDrawable:(id<CAMetalDrawable>)drawable;
- (void)publishGlyphStats;
- (void)fail:(NSString *)message;
- (void)observeDevices;
- (void)stopObservingDevices;
- (void)requestRecovery:(NSString *)reason excluding:(uint64_t)registryID;
- (void)recoverExcluding:(uint64_t)registryID reason:(NSString *)reason;
- (void)deliverScrollWheel:(NSEvent *)event atPoint:(NSPoint)point;
@end

API_AVAILABLE(macos(14.0))
@interface GDMetalFrameClock : NSObject <CAMetalDisplayLinkDelegate>
@property(nonatomic,weak) GDView *view;
@property(nonatomic,strong) CAMetalDisplayLink *link;
@end

@interface GDDelegate : NSObject <NSApplicationDelegate,NSWindowDelegate>
@property(nonatomic,strong) NSWindow *window;
@property(nonatomic) BOOL focused;
@end

static GDView *active_view;
static BOOL running;
static _Atomic uint64_t generation;

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
static id<MTLDevice> select_device(uint64_t excluded) {
    id<MTLDevice> preferred=MTLCreateSystemDefaultDevice();
    if(preferred && (!excluded || preferred.registryID!=excluded)) return preferred;
    for(id<MTLDevice> candidate in MTLCopyAllDevices()) if(candidate.registryID!=excluded) return candidate;
    return nil;
}
static GDView *create_gpu_view(NSRect frame,id<MTLDevice> device,GDColor background,BOOL readback,GDRunState *metrics,NSString **failure) {
    NSError *error=nil;
    id<MTLLibrary> library=[device newLibraryWithSource:shader options:nil error:&error];
    if(!library) { *failure=error.localizedDescription ?: @"Metal shader library allocation failed"; return nil; }
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
    if(!state) { *failure=error.localizedDescription ?: @"Metal pipeline allocation failed"; return nil; }
    GDView *view=[[GDView alloc] initWithFrame:frame device:device];
    if(!view) { *failure=@"Metal view allocation failed"; return nil; }
    atomic_init(&view->recoveryRequested,false);
    view.metrics=metrics;
    view.pipeline=state; view.queue=[device newCommandQueue];
    if(!view.queue) { *failure=@"Metal command queue allocation failed"; return nil; }
    view.slots=@[[[GDFrameSlot alloc] init],[[GDFrameSlot alloc] init],[[GDFrameSlot alloc] init]];
    view.outstanding=dispatch_group_create(); view.background=background;
    view.readback=readback; view.framebufferOnly=!readback;
    view.atlas=[[GDGlyphAtlas alloc] initWithDevice:device]; view.layouts=[NSMutableDictionary dictionary];
    view.scene=[NSData data]; view.text=[NSData data];
    view.bitmaps=[NSMutableDictionary dictionary];
    view.colorPixelFormat=MTLPixelFormatBGRA8Unorm;
    const char *density=getenv("GODESKTOP_TEST_DRAWABLE_SCALE");
    if(density) {
        double scale=strtod(density,NULL);
        if(!isfinite(scale) || scale<1 || scale>4) { *failure=@"Invalid diagnostic drawable scale"; return nil; }
        // Only diagnostics override density. This still renders/copies the
        // actual window drawable; no separate render target replaces it.
        view.autoResizeDrawable=NO;
        view.drawableSize=CGSizeMake(ceil(frame.size.width*scale),ceil(frame.size.height*scale));
    }
    view.paused=YES; view.enableSetNeedsDisplay=NO; view.delegate=view;
    return view;
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
    gd_go_event(2,p.x,p.y,0,(event.modifierFlags&NSEventModifierFlagShift)?1:0);
}
- (void)mouseUp:(NSEvent *)event {
    NSPoint p=[self convertPoint:event.locationInWindow fromView:nil];
    gd_go_event(3,p.x,p.y,0,(event.modifierFlags&NSEventModifierFlagShift)?1:0);
}
- (void)mouseDragged:(NSEvent *)event {
    NSPoint p=[self convertPoint:event.locationInWindow fromView:nil];
    gd_go_event(8,p.x,p.y,0,(event.modifierFlags&NSEventModifierFlagShift)?1:0);
}
- (void)keyDown:(NSEvent *)event {
    int key=0;
    int mods=((event.modifierFlags&NSEventModifierFlagShift)?1:0)|((event.modifierFlags&NSEventModifierFlagControl)?2:0)|((event.modifierFlags&NSEventModifierFlagOption)?4:0)|((event.modifierFlags&NSEventModifierFlagCommand)?8:0);
    switch(event.keyCode) { case 48:key=9;break; case 36:case 76:key=13;break; case 49:key=32;break; case 53:key=27;break; case 51:key=8;break; case 117:key=46;break; case 116:key=33;break; case 121:key=34;break; case 123:key=37;break; case 124:key=39;break; case 125:key=40;break; case 126:key=38;break; }
    if(!key && (mods&10) && event.charactersIgnoringModifiers.length) key=toupper([event.charactersIgnoringModifiers characterAtIndex:0]);
    if(key) gd_go_event(4,0,0,key,mods|(event.isARepeat?16:0));
    if(!(mods&10)) {
        NSString *text=event.characters;
        for(NSUInteger i=0;i<text.length;i++) {
            unsigned value=[text characterAtIndex:i];
            if(value>=0xd800 && value<=0xdbff && i+1<text.length) { unsigned low=[text characterAtIndex:++i]; if(low<0xdc00 || low>0xdfff) continue; value=0x10000+((value-0xd800)<<10)+(low-0xdc00); }
            if(value>=32 && value!=127 && !(value>=0xf700 && value<=0xf8ff)) gd_go_event(6,0,0,value,mods);
        }
    }
}
- (void)scrollWheel:(NSEvent *)event {
    NSPoint p=[self convertPoint:event.locationInWindow fromView:nil];
    [self deliverScrollWheel:event atPoint:p];
}
- (void)deliverScrollWheel:(NSEvent *)event atPoint:(NSPoint)p {
    CGFloat unit=event.hasPreciseScrollingDeltas?12.0f:1.0f;
    int mods=((event.modifierFlags&NSEventModifierFlagShift)?1:0)|((event.modifierFlags&NSEventModifierFlagControl)?2:0)|((event.modifierFlags&NSEventModifierFlagOption)?4:0)|((event.modifierFlags&NSEventModifierFlagCommand)?8:0);
    gd_go_event(1,self.bounds.size.width,self.bounds.size.height,0,0);
    gd_go_scroll(-event.scrollingDeltaX/unit,event.scrollingDeltaY/unit,p.x,p.y,mods);
}
- (void)keyUp:(NSEvent *)event {
    int key=0;
    switch(event.keyCode) {case 48:key=9;break;case 36:case 76:key=13;break;case 49:key=32;break;case 53:key=27;break;case 51:key=8;break;case 117:key=46;break;case 116:key=33;break;case 121:key=34;break;case 123:key=37;break;case 124:key=39;break;case 125:key=40;break;case 126:key=38;break;}
    if(!key && event.charactersIgnoringModifiers.length && [event.charactersIgnoringModifiers characterAtIndex:0]<128) key=toupper([event.charactersIgnoringModifiers characterAtIndex:0]);
    int mods=((event.modifierFlags&NSEventModifierFlagShift)?1:0)|((event.modifierFlags&NSEventModifierFlagControl)?2:0)|((event.modifierFlags&NSEventModifierFlagOption)?4:0)|((event.modifierFlags&NSEventModifierFlagCommand)?8:0);
    if(key) gd_go_event(9,0,0,key,mods);
}
- (void)flagsChanged:(NSEvent *)event {
    int key=0;NSEventModifierFlags flag=0;
    switch(event.keyCode) {case 59:case 62:key=17;flag=NSEventModifierFlagControl;break;case 56:case 60:key=16;flag=NSEventModifierFlagShift;break;case 58:case 61:key=18;flag=NSEventModifierFlagOption;break;case 55:case 54:key=91;flag=NSEventModifierFlagCommand;break;}
    int mods=((event.modifierFlags&NSEventModifierFlagShift)?1:0)|((event.modifierFlags&NSEventModifierFlagControl)?2:0)|((event.modifierFlags&NSEventModifierFlagOption)?4:0)|((event.modifierFlags&NSEventModifierFlagCommand)?8:0);
    if(key) gd_go_event((event.modifierFlags&flag)?4:9,0,0,key,mods);
}
- (void)mtkView:(MTKView *)view drawableSizeWillChange:(CGSize)size { [self requestFrame]; }
- (void)requestFrame {
    GDRunState *metrics=self.metrics;
    atomic_fetch_add(&metrics->frame_requests,1);
    if(self.frameDirty) atomic_fetch_add(&metrics->coalesced_requests,1);
    self.frameDirty=YES;
    if(atomic_load(&recoveryRequested)) return;
    if(!self.window.visible || self.window.miniaturized) return;
    if(@available(macOS 14.0,*)) {
        if(self.frameClock) { ((GDMetalFrameClock *)self.frameClock).link.paused=NO; return; }
    }
    // The macOS 13 compatibility path uses MTKView's display-synchronised loop.
    // No timer or unpaced setNeedsDisplay chain is used on either path.
    self.paused=NO;
}
- (void)pauseFrameClock {
    GDRunState *metrics=self.metrics;
    if(@available(macOS 14.0,*)) {
        if(self.frameClock) {
            GDMetalFrameClock *clock=self.frameClock;
            if(!clock.link.paused) { clock.link.paused=YES; atomic_fetch_add(&metrics->idle_pauses,1); }
            return;
        }
    }
    if(!self.paused) { self.paused=YES; atomic_fetch_add(&metrics->idle_pauses,1); }
}
- (void)startFrameClock {
    GDRunState *metrics=self.metrics;
    const char *density=getenv("GODESKTOP_TEST_DRAWABLE_SCALE");
    if(density) {
        // MTKView attaches to the window after construction. Apply the explicit
        // diagnostic size after that attachment so backing-property setup cannot
        // replace it with the system density before the display link starts.
        double scale=strtod(density,NULL);
        self.autoResizeDrawable=NO;
        CGSize size=CGSizeMake(ceil(self.bounds.size.width*scale),ceil(self.bounds.size.height*scale));
        self.drawableSize=size;
        CAMetalLayer *layer=(CAMetalLayer *)self.layer;
        layer.contentsScale=scale; layer.drawableSize=size;
    }
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
        atomic_store(&metrics->frame_clock,2);
        [clock.link addToRunLoop:NSRunLoop.mainRunLoop forMode:NSRunLoopCommonModes];
    } else {
        self.preferredFramesPerSecond=MAX(1,self.window.screen.maximumFramesPerSecond);
        atomic_store(&metrics->frame_clock,1);
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
- (void)observeDevices {
    __weak GDView *weakView=self;
    id<NSObject> observer=nil;
    MTLCopyAllDevicesWithObserver(&observer,^(id<MTLDevice> device,MTLDeviceNotificationName notification) {
        if(![notification isEqualToString:MTLDeviceRemovalRequestedNotification] && ![notification isEqualToString:MTLDeviceWasRemovedNotification]) return;
        uint64_t registryID=device.registryID;
        dispatch_async(dispatch_get_main_queue(),^{
            GDView *view=weakView;
            if(running && view && active_view==view && view.device.registryID==registryID)
                [view requestRecovery:@"Metal GPU removal notification" excluding:registryID];
        });
    });
    self.deviceObserver=observer;
}
- (void)stopObservingDevices {
    if(self.deviceObserver) { MTLRemoveDeviceObserver(self.deviceObserver); self.deviceObserver=nil; }
}
- (void)requestRecovery:(NSString *)reason excluding:(uint64_t)registryID {
    bool expected=false;
    if(!atomic_compare_exchange_strong(&recoveryRequested,&expected,true)) return;
    uint64_t expectedGeneration=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{
        if(running && active_view==self && expectedGeneration==atomic_load(&generation)) [self recoverExcluding:registryID reason:reason];
    });
}
- (void)recoverExcluding:(uint64_t)registryID reason:(NSString *)reason {
    GDRunState *metrics=self.metrics;
    [self stopFrameClock]; [self stopObservingDevices];
    if(atomic_load(&metrics->device_recoveries)>=3) { [self fail:[@"Metal GPU recovery limit exhausted: " stringByAppendingString:reason]]; return; }
    // Recovery is exceptional. Normal rendering still never waits for a slot.
    if(dispatch_group_wait(self.outstanding,dispatch_time(DISPATCH_TIME_NOW,5*NSEC_PER_SEC))) { [self fail:@"Metal recovery could not drain submissions within five seconds"]; return; }
    [self publishGlyphStats];
    NSWindow *window=self.window;
    NSRect frame=self.frame; GDColor background=self.background; BOOL readback=self.readback;
    NSData *scene=self.scene,*text=self.text;
    NSMutableDictionary *layouts=self.layouts;
    GDGlyphAtlas *previous=self.atlas;
    // Release every old device-dependent resource before constructing the new
    // queue. Completed frames remain completed; failed submissions are counted
    // separately by their completion handlers.
    [previous clear]; self.atlas=nil; self.bitmaps=nil;self.slots=nil; self.pipeline=nil; self.queue=nil;
    self.delegate=nil; [self releaseDrawables]; self.device=nil;
    id<MTLDevice> device=select_device(registryID);
    if(!device) { [self fail:@"No usable alternate Metal device after GPU removal"]; return; }
    NSString *failure=nil;
    GDView *replacement=create_gpu_view(frame,device,background,readback,metrics,&failure);
    if(!replacement) { [self fail:[@"Metal GPU recovery failed: " stringByAppendingString:failure ?: @"unknown error"]]; return; }
    replacement.scene=scene; replacement.text=text; replacement.layouts=layouts;
    replacement.atlas.usage=previous.usage;
    replacement.atlas.rasterized=previous.rasterized; replacement.atlas.hits=previous.hits;
    replacement.atlas.epochs=previous.epochs; replacement.atlas.uploadedBytes=previous.uploadedBytes;
    replacement.atlas.bitmapUploads=previous.bitmapUploads;replacement.atlas.bitmapUploadedBytes=previous.bitmapUploadedBytes;
    // Keep the Run generation stable: already queued Go Dispatch/Quit/window
    // actions must target this same window after its view is replaced. Old view
    // completion/UI callbacks check identity and cannot affect the replacement.
    active_view=replacement; window.contentView=replacement;
    [window makeFirstResponder:replacement]; gd_go_event(5,0,0,0,0);
    atomic_fetch_add(&metrics->device_recoveries,1);
    [replacement publishGlyphStats]; [replacement observeDevices]; [replacement startFrameClock];
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
    for(NSNumber *key in self.bitmaps.allKeys) if(!gd_image_get(key.unsignedLongLongValue)) [self.bitmaps removeObjectForKey:key];
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
            } else if(command->kind==5) {
                const GDImage *image=gd_image_get(command->image_id);
                if(!image) {[self fail:@"Bitmap scene references a missing image"];return nil;}
                NSNumber *key=@(command->image_id);
                GDAtlasPage *page=self.bitmaps[key];
                if(!page) {
                    page=[[GDAtlasPage alloc] init];page.width=image->width;page.height=image->height;page.channels=4;page.bitmapID=image->id;page.version=1;
                    self.bitmaps[key]=page;
                }
                success=[scene append:gd_gpu_instance(command) page:page];
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
    GDRunState *metrics=self.metrics;
    GDGlyphAtlas *atlas=self.atlas;
    if(!atlas) { atomic_store(&metrics->glyph_cache_entries,0); atomic_store(&metrics->glyph_atlas_pages,0); atomic_store(&metrics->glyph_atlas_bytes,0); return; }
    uint64_t pages=atomic_load(&atlas.usage->pages);
    atomic_store(&metrics->glyph_rasterizations,atlas.rasterized); atomic_store(&metrics->glyph_cache_hits,atlas.hits);
    atomic_store(&metrics->glyph_cache_entries,atlas.glyphs.count); atomic_store(&metrics->glyph_atlas_pages,pages);
    atomic_store(&metrics->glyph_atlas_bytes,atomic_load(&atlas.usage->bytes));
    atomic_store(&metrics->glyph_atlas_peak_bytes,atomic_load(&atlas.usage->peakBytes));
    atomic_store(&metrics->glyph_atlas_epochs,atlas.epochs); atomic_store(&metrics->glyph_uploaded_bytes,atlas.uploadedBytes);
    atomic_store(&metrics->bitmap_cache_entries,gd_image_entries());atomic_store(&metrics->bitmap_cache_bytes,gd_image_bytes);
    atomic_store(&metrics->bitmap_uploads,atlas.bitmapUploads);atomic_store(&metrics->bitmap_uploaded_bytes,atlas.bitmapUploadedBytes);
}
- (void)drawInMTKView:(MTKView *)view {
    GDRunState *metrics=self.metrics;
    // MTKView must never obtain a second drawable while the modern driver
    // already supplied one. This delegate is only the macOS 13 path.
    if(self.modernFrameClock) return;
    if(!running || active_view!=self) return;
    atomic_fetch_add(&metrics->frame_ticks,1);
    if(!self.frameDirty || !self.window.visible || self.window.miniaturized) { [self pauseFrameClock]; return; }
    self.frameDirty=NO;
    [self renderDrawable:nil];
}
- (void)renderDrawable:(id<CAMetalDrawable>)supplied {
    @autoreleasepool {
        GDRunState *metrics=self.metrics;
        if(atomic_load(&recoveryRequested)) { [self pauseFrameClock]; return; }
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
        if(self.failure || atomic_load(&recoveryRequested)) { atomic_store(&slot->busy,false); return; }
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
        // Private frame uniform: DIP width/height, actual pixels per DIP, pad.
        // Positive Gaussian shadows use the physical fragment position.
        float viewport[4]={self.bounds.size.width,self.bounds.size.height,scale,0};
        [encoder setVertexBytes:viewport length:sizeof(viewport) atIndex:1];
        for(NSUInteger i=0;i<GDAtlasMaxPages;i++) {
            GDAtlasPage *page=i<native.glyphPages.count?native.glyphPages[i]:native.glyphPages[0];
            [encoder setFragmentTexture:page.texture atIndex:i];
        }
        [encoder setFragmentTexture:native.glyphPages[0].texture atIndex:GDAtlasMaxPages];
        for(NSUInteger i=0;i<batchCount;i++) {
            GDBatch batch=batches[i];
            [encoder setVertexBuffer:slot.instances offset:batch.start*sizeof(GDGPUInstance) atIndex:0];
            if(batch.texture) [encoder setFragmentTexture:heldPages[batch.texture].texture atIndex:GDAtlasMaxPages];
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
        uint32_t flight=atomic_fetch_add(&metrics->in_flight,1)+1;
        uint32_t peak=atomic_load(&metrics->max_in_flight);
        while(flight>peak && !atomic_compare_exchange_weak(&metrics->max_in_flight,&peak,flight)) {}
        atomic_fetch_or(&metrics->used_slots,(uint32_t)(1u<<slotIndex));
        atomic_store(&metrics->draw_calls,batchCount); atomic_store(&metrics->instance_count,encoded);
        atomic_store(&metrics->uploaded_bytes,encoded*sizeof(GDGPUInstance));
        uint64_t expectedGeneration=atomic_load(&generation);
        uint64_t serial=atomic_load(&metrics->submitted_frames)+1;
        [buffer addCompletedHandler:^(id<MTLCommandBuffer> completed) {
            BOOL current=expectedGeneration==atomic_load(&generation);
            if(current && completed.status==MTLCommandBufferStatusError) {
                NSString *message=completed.error.localizedDescription ?: @"Metal submission failed";
                atomic_fetch_add(&metrics->dropped_frames,1);
                // Device removal/access revocation must select a different GPU.
                // Other command errors rebuild the queue/resources on a usable
                // default GPU, with the same bounded three-recovery limit.
                BOOL excluded=completed.error.code==MTLCommandBufferErrorDeviceRemoved || completed.error.code==MTLCommandBufferErrorAccessRevoked;
                [self requestRecovery:message excluding:excluded?self.device.registryID:0];
            } else if(current) {
                if(readback) {
                    NSMutableData *pixels=[NSMutableData dataWithLength:(NSUInteger)readbackWidth*readbackHeight*4];
                    for(NSUInteger row=0;row<readbackHeight;row++) memcpy((unsigned char *)pixels.mutableBytes+row*readbackWidth*4,(const unsigned char *)readback.contents+row*readbackPitch,readbackWidth*4);
                    GDMetalSnapshot *snapshot=[[GDMetalSnapshot alloc] init];
                    snapshot.pixels=pixels; snapshot.width=readbackWidth; snapshot.height=readbackHeight; snapshot.stride=readbackWidth*4; snapshot.frame=serial;
                    [metrics.snapshotLock lock];
                    if(!metrics.snapshot || metrics.snapshot.frame<serial) metrics.snapshot=snapshot;
                    [metrics.snapshotLock unlock];
                }
                atomic_fetch_add(&metrics->rendered_frames,1);
                double elapsed=completed.GPUEndTime-completed.GPUStartTime;
                if(elapsed>0) atomic_store(&metrics->gpu_nanos,(uint64_t)(elapsed*1000000000));
                // Explicit diagnostic injection after an actual successful GPU
                // completion. This does not falsify command status or drop counts
                // and does not claim to physically remove hardware.
                if(metrics->diagnostic_recovery_interval && serial%metrics->diagnostic_recovery_interval==0) {
                    uint64_t remaining=atomic_load(&metrics->diagnostic_recovery_remaining);
                    while(remaining && !atomic_compare_exchange_weak(&metrics->diagnostic_recovery_remaining,&remaining,remaining-1)) {}
                    if(remaining) [self requestRecovery:@"Diagnostic recovery after real Metal GPU completion" excluding:0];
                }
            }
            if(current) atomic_fetch_sub(&metrics->in_flight,1);
            heldPages=nil; heldUploads=nil;
            atomic_store(&slot->busy,false);
            dispatch_group_leave(self.outstanding);
            dispatch_async(dispatch_get_main_queue(),^{
                if(running && active_view==self && self.deferred) { self.deferred=NO; [self requestFrame]; }
            });
        }];
        atomic_fetch_add(&metrics->submitted_frames,1);
        if(!self.modernFrameClock) [buffer presentDrawable:drawable];
        [buffer commit];
        // CAMetalDisplayLink supplied the drawable and its presentation timing.
        // Commit first, then present without an explicit time or GPU wait.
        if(self.modernFrameClock) [drawable present];
        uint64_t finished=nanos();
        atomic_store(&metrics->cpu_nanos,finished-started);
        atomic_store(&metrics->scene_nanos,(sceneEnd-started)+(nativeSceneEnd-acquireEnd));
        atomic_store(&metrics->acquire_nanos,acquireEnd-sceneEnd);
        atomic_store(&metrics->encode_nanos,finished-nativeSceneEnd);
        [self publishGlyphStats];
    }
}
@end

@implementation GDMetalFrameClock
- (void)metalDisplayLink:(CAMetalDisplayLink *)link needsUpdate:(CAMetalDisplayLinkUpdate *)update {
    GDView *view=self.view;
    if(!running || active_view!=view || !view) return;
    GDRunState *metrics=view.metrics;
    atomic_fetch_add(&metrics->frame_ticks,1);
    if(!view.frameDirty || !view.window.visible || view.window.miniaturized) { [view pauseFrameClock]; return; }
    view.frameDirty=NO;
    [view renderDrawable:update.drawable];
}
@end

@implementation GDDelegate
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender { return NO; }
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender { [self.window performClose:nil]; return NSTerminateCancel; }
- (BOOL)windowShouldClose:(NSWindow *)sender { return gd_go_should_close()!=0; }
- (void)windowDidResignKey:(NSNotification *)notification {
    if(self.focused) { self.focused=NO; gd_go_event(10,0,0,0,0); }
    gd_go_event(5,0,0,0,0);
}
- (void)windowDidDeminiaturize:(NSNotification *)notification { [active_view requestFrame]; }
- (void)windowDidBecomeKey:(NSNotification *)notification {
    if(!self.focused) { self.focused=YES; gd_go_event(10,0,0,1,0); }
    [active_view requestFrame];
}
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
    if(![NSThread isMainThread]) return "AppKit must run on the process main thread; call godesktop.Run from main";
    @autoreleasepool {
        GDRunState *metrics=[[GDRunState alloc] init];
        NSLock *lock=state_lock(); [lock lock]; latest_metrics=metrics; [lock unlock];
        const char *interval=getenv("GODESKTOP_TEST_METAL_RECOVERY");
        const char *repetitions=getenv("GODESKTOP_TEST_METAL_RECOVERIES");
        metrics->diagnostic_recovery_interval=interval?strtoull(interval,NULL,10):0;
        atomic_store(&metrics->diagnostic_recovery_remaining,metrics->diagnostic_recovery_interval?(repetitions?strtoull(repetitions,NULL,10):1):0);
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        id<MTLDevice> device=select_device(0);
        if(!device) return "No Metal device is available";
        const char *readback=getenv("GODESKTOP_READBACK");
        NSString *failure=nil;
        GDView *view=create_gpu_view(NSMakeRect(0,0,width,height),device,background,readback && strcmp(readback,"1")==0,metrics,&failure);
        if(!view) { last_error=strdup(failure.UTF8String); return last_error; }
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
        [NSApp activateIgnoringOtherApps:YES]; [view observeDevices]; [view startFrameClock];
        [NSApp run];
        view=active_view; // A GPU recovery can replace the view in this window.
        running=NO;
        [view stopFrameClock]; [view stopObservingDevices];
        // Only shutdown drains the queue. Normal frames never wait for the GPU.
        if(dispatch_group_wait(view.outstanding,dispatch_time(DISPATCH_TIME_NOW,5*NSEC_PER_SEC))) view.failure=@"Metal shutdown did not finish within five seconds";
        [view publishGlyphStats];
        active_view=nil;
        atomic_fetch_add(&generation,1);
        window.delegate=nil; [window orderOut:nil]; [window close];
        NSApp.delegate=nil; view.delegate=nil;
        if(view.failure) last_error=strdup(view.failure.UTF8String);
    }
    gd_images_clear();
    return last_error;
}

void gd_present(const GDCommand *commands,size_t count,const char *text,size_t length) {
    active_view.scene=count?[NSData dataWithBytes:commands length:count*sizeof(GDCommand)]:[NSData data];
    active_view.text=length?[NSData dataWithBytes:text length:length]:[NSData data];
}
float gd_text_advance(const char *text,size_t length,float size,const char *font,size_t font_length) {
    CTLineRef line=text_line(string_utf8(text,length),size,string_utf8(font,font_length));
    double advance=CTLineGetTypographicBounds(line,NULL,NULL,NULL);
    CFRelease(line);
    return (float)advance;
}
void gd_measure(const char *text,size_t length,float size,const char *font,size_t font_length,float *width,float *height) {
    CTLineRef line=text_line(string_utf8(text,length),size,string_utf8(font,font_length));
    CGFloat descent;
    text_size(line,width,height,&descent);
    CFRelease(line);
}
void gd_wake(void) {
    uint64_t expected=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{
        if(running && expected==atomic_load(&generation)) {
            // UI receipts are independent of a visible/available Metal drawable.
            gd_go_event(11,0,0,0,0);
            [active_view requestFrame];
        }
    });
}
void gd_quit(void) {
    uint64_t expected=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{ if(running && expected==atomic_load(&generation)) [active_view.window close]; });
}
uint64_t gd_rendered_frames(void) { @autoreleasepool { GDRunState *metrics=current_metrics(); return metrics?atomic_load(&metrics->rendered_frames):0; } }
const char *gd_metal_snapshot(GDGPUSnapshot *result) {
    @autoreleasepool {
    GDRunState *metrics=current_metrics();
    memset(result,0,sizeof(*result));
    [metrics.snapshotLock lock]; GDMetalSnapshot *snapshot=metrics.snapshot; [metrics.snapshotLock unlock];
    if(!snapshot) return "No completed Metal drawable readback; enable GODESKTOP_READBACK=1";
    result->width=snapshot.width; result->height=snapshot.height; result->stride=snapshot.stride;
    result->frame=snapshot.frame; result->bytes=snapshot.pixels.length;
    result->pixels=malloc(result->bytes);
    if(!result->pixels) return "Metal snapshot copy allocation failed";
    memcpy(result->pixels,snapshot.pixels.bytes,result->bytes); return NULL;
    }
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
    @autoreleasepool {
    GDRunState *metrics=current_metrics();
    if(!metrics) return (GDRenderStats){.backend=2,.frame_slots=3};
    return (GDRenderStats){
        .backend=2,.frame_slots=3,.used_slots_mask=atomic_load(&metrics->used_slots),
        .frame_clock=atomic_load(&metrics->frame_clock),
        .in_flight=atomic_load(&metrics->in_flight),.max_in_flight=atomic_load(&metrics->max_in_flight),
        .submitted=atomic_load(&metrics->submitted_frames),.completed=atomic_load(&metrics->rendered_frames),
        .draw_calls=atomic_load(&metrics->draw_calls),.instances=atomic_load(&metrics->instance_count),
        .uploaded_bytes=atomic_load(&metrics->uploaded_bytes),.cpu_nanos=atomic_load(&metrics->cpu_nanos),.gpu_nanos=atomic_load(&metrics->gpu_nanos),
        .scene_nanos=atomic_load(&metrics->scene_nanos),.acquire_nanos=atomic_load(&metrics->acquire_nanos),.encode_nanos=atomic_load(&metrics->encode_nanos),
        .frame_requests=atomic_load(&metrics->frame_requests),.frame_ticks=atomic_load(&metrics->frame_ticks),
        .coalesced_requests=atomic_load(&metrics->coalesced_requests),.idle_pauses=atomic_load(&metrics->idle_pauses),
        .glyph_rasterizations=atomic_load(&metrics->glyph_rasterizations),.glyph_cache_hits=atomic_load(&metrics->glyph_cache_hits),.glyph_cache_entries=atomic_load(&metrics->glyph_cache_entries),
        .glyph_atlas_pages=atomic_load(&metrics->glyph_atlas_pages),.glyph_atlas_bytes=atomic_load(&metrics->glyph_atlas_bytes),.glyph_atlas_peak_bytes=atomic_load(&metrics->glyph_atlas_peak_bytes),
        .glyph_atlas_epochs=atomic_load(&metrics->glyph_atlas_epochs),.glyph_uploaded_bytes=atomic_load(&metrics->glyph_uploaded_bytes),
        .device_recoveries=atomic_load(&metrics->device_recoveries),.dropped_frames=atomic_load(&metrics->dropped_frames),
        .bitmap_cache_entries=atomic_load(&metrics->bitmap_cache_entries),.bitmap_cache_bytes=atomic_load(&metrics->bitmap_cache_bytes),
        .bitmap_uploads=atomic_load(&metrics->bitmap_uploads),.bitmap_uploaded_bytes=atomic_load(&metrics->bitmap_uploaded_bytes)
    };
    }
}
uint64_t gd_metal_window_identity(void) { return [NSThread isMainThread]?(uint64_t)active_view.window.windowNumber:0; }

const char *gd_metal_test_wheel(int dx,int dy,float x,float y,int modifiers,int precise) {
    if(![NSThread isMainThread] || !active_view || !active_view.readback) return "Owned Metal wheel probe requires the UI thread and GODESKTOP_READBACK=1";
    CGEventRef raw=CGEventCreateScrollWheelEvent(NULL,precise?kCGScrollEventUnitPixel:kCGScrollEventUnitLine,2,dy,dx);
    if(!raw) return "Cannot construct native scroll wheel event";
    NSPoint base=[active_view convertPoint:NSMakePoint(x,y) toView:nil];
    NSPoint screen=[active_view.window convertPointToScreen:base];
    CGEventSetLocation(raw,CGPointMake(screen.x,NSMaxY(NSScreen.screens.firstObject.frame)-screen.y));
    CGEventFlags flags=0;
    if(modifiers&1) flags|=kCGEventFlagMaskShift;if(modifiers&2) flags|=kCGEventFlagMaskControl;
    if(modifiers&4) flags|=kCGEventFlagMaskAlternate;if(modifiers&8) flags|=kCGEventFlagMaskCommand;
    CGEventSetFlags(raw,flags);
    // Unposted CGEvents have no AppKit window attachment. Resolve their actual
    // Quartz screen coordinates through the owned NSWindow/NSView, then use
    // the same native delta/modifier/layout-delivery path as scrollWheel:.
    // The probe never asks AppKit to dispatch to an unrelated window.
    CGPoint quartz=CGEventGetLocation(raw);
    NSPoint nativeScreen=NSMakePoint(quartz.x,NSMaxY(NSScreen.screens.firstObject.frame)-quartz.y);
    NSPoint windowPoint=[active_view.window convertPointFromScreen:nativeScreen];
    NSPoint local=[active_view convertPoint:windowPoint fromView:nil];
    NSEvent *event=[NSEvent eventWithCGEvent:raw];CFRelease(raw);
    if(!event || event.type!=NSEventTypeScrollWheel || fabs(local.x-x)>.01 || fabs(local.y-y)>.01) return "Native wheel event coordinate conversion failed";
    [active_view deliverScrollWheel:event atPoint:local];
    return NULL;
}

const char *gd_metal_test_pointer(int pressed,float x,float y,int modifiers) {
    if(![NSThread isMainThread] || !active_view || !active_view.readback) return "Owned Metal pointer probe requires the UI thread and GODESKTOP_READBACK=1";
    NSPoint p=[active_view convertPoint:NSMakePoint(x,y) toView:nil];
    NSEventModifierFlags flags=0;
    if(modifiers&1) flags|=NSEventModifierFlagShift;if(modifiers&2) flags|=NSEventModifierFlagControl;
    if(modifiers&4) flags|=NSEventModifierFlagOption;if(modifiers&8) flags|=NSEventModifierFlagCommand;
    NSEventType type=pressed==2?NSEventTypeLeftMouseDragged:(pressed?NSEventTypeLeftMouseDown:NSEventTypeLeftMouseUp);
    NSEvent *event=[NSEvent mouseEventWithType:type location:p modifierFlags:flags timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:active_view.window.windowNumber context:nil eventNumber:1 clickCount:1 pressure:(pressed?1.0:0.0)];
    if(!event) return "Cannot construct native pointer event";
    if(pressed==2) [active_view mouseDragged:event];else if(pressed) [active_view mouseDown:event];else [active_view mouseUp:event];
    return NULL;
}

const char *gd_metal_test_key(int key,int modifiers,int pressed) {
    if(![NSThread isMainThread] || !active_view || !active_view.readback) return "Owned Metal key probe requires the UI thread and GODESKTOP_READBACK=1";
    unsigned short code=0;BOOL modifier=NO;int mask=0;
    switch(key){case 9:code=48;break;case 13:code=36;break;case 27:code=53;break;case 33:code=116;break;case 34:code=121;break;case 37:code=123;break;case 39:code=124;break;case 16:code=56;modifier=YES;mask=1;break;case 17:code=59;modifier=YES;mask=2;break;case 18:code=58;modifier=YES;mask=4;break;case 91:code=55;modifier=YES;mask=8;break;default:if(key<32||key>126)return "Unsupported diagnostic native key";}
    if(modifier) modifiers=pressed?(modifiers|mask):(modifiers&~mask);
    NSEventModifierFlags flags=0;
    if(modifiers&1)flags|=NSEventModifierFlagShift;if(modifiers&2)flags|=NSEventModifierFlagControl;if(modifiers&4)flags|=NSEventModifierFlagOption;if(modifiers&8)flags|=NSEventModifierFlagCommand;
    NSString *chars=key>=32&&key<=126?[NSString stringWithFormat:@"%c",key]:@"";
    NSEventType type=modifier?NSEventTypeFlagsChanged:(pressed?NSEventTypeKeyDown:NSEventTypeKeyUp);
    NSEvent *event=[NSEvent keyEventWithType:type location:NSZeroPoint modifierFlags:flags timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:active_view.window.windowNumber context:nil characters:chars charactersIgnoringModifiers:chars isARepeat:NO keyCode:code];
    if(!event)return "Cannot construct owned native key event";
    if(modifier)[active_view flagsChanged:event];else if(pressed)[active_view keyDown:event];else [active_view keyUp:event];
    return NULL;
}

void gd_window_action(int action) {
    uint64_t expected=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{ if(!running || expected!=atomic_load(&generation)) return; if(action==1) [active_view.window miniaturize:nil]; if(action==2) [active_view.window zoom:nil]; if(action==3) [active_view.window performClose:nil]; });
}
