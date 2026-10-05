//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <MetalKit/MetalKit.h>
#import <CoreText/CoreText.h>
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

@interface GDFrameSlot : NSObject { @public _Atomic bool busy; }
@property(nonatomic,strong) id<MTLBuffer> instances;
@property(nonatomic) NSUInteger capacity;
@end
@implementation GDFrameSlot
- (instancetype)init {
    self=[super init];
    if(self) atomic_init(&busy,false);
    return self;
}
@end

typedef struct { NSUInteger start, count, texture; } GDBatch;

@interface GDView : MTKView <MTKViewDelegate>
@property(nonatomic,strong) id<MTLCommandQueue> queue;
@property(nonatomic,strong) id<MTLRenderPipelineState> pipeline;
@property(nonatomic,strong) id<MTLTexture> white;
@property(nonatomic,strong) NSArray<GDFrameSlot *> *slots;
@property(nonatomic) NSUInteger nextSlot;
@property(nonatomic) BOOL deferred;
@property(nonatomic,strong) dispatch_group_t outstanding;
@property(nonatomic,strong) NSMutableDictionary<NSString *,id<MTLTexture>> *glyphs;
@property(nonatomic) NSUInteger glyphBytes;
@property(nonatomic,strong) NSData *scene;
@property(nonatomic,strong) NSData *text;
@property(nonatomic) GDColor background;
@property(atomic,copy) NSString *failure;
@end

@interface GDDelegate : NSObject <NSApplicationDelegate,NSWindowDelegate>
@property(nonatomic,strong) NSWindow *window;
@end

static GDView *active_view;
static BOOL running;
static _Atomic uint64_t generation;
static _Atomic uint64_t rendered_frames;
static _Atomic uint64_t submitted_frames, draw_calls, instance_count, uploaded_bytes, cpu_nanos, gpu_nanos;
static _Atomic uint32_t used_slots, in_flight, max_in_flight;

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
- (void)mtkView:(MTKView *)view drawableSizeWillChange:(CGSize)size { [self setNeedsDisplay:YES]; }
- (void)fail:(NSString *)message {
    self.failure=message;
    gd_quit();
}
- (id<MTLTexture>)glyph:(const GDCommand *)command scale:(CGFloat)scale {
    const char *bytes=(const char *)self.text.bytes+command->text_offset;
    NSString *value=string_utf8(bytes,command->text_length);
    NSString *family=string_utf8((const char *)self.text.bytes+command->font_offset,command->font_length);
    NSString *key=[NSString stringWithFormat:@"%g/%g/%@/%@",command->font_size,scale,family,value];
    id<MTLTexture> existing=self.glyphs[key];
    if(existing) return existing;
    CTLineRef line=text_line(value,command->font_size,family);
    float width,height; CGFloat descent;
    text_size(line,&width,&height,&descent);
    NSUInteger pixelWidth=MAX(1,ceil(width*scale)),pixelHeight=MAX(1,ceil(height*scale));
    if(pixelWidth>16384 || pixelHeight>16384) { CFRelease(line); [self fail:@"Text exceeds the Metal texture size limit"]; return nil; }
    NSUInteger textureBytes=pixelWidth*pixelHeight*4;
    if(textureBytes>16*1024*1024) { CFRelease(line); [self fail:@"Text exceeds the 16 MiB raster budget"]; return nil; }
    if(self.glyphs.count>=1024 || self.glyphBytes+textureBytes>16*1024*1024) {
        [self.glyphs removeAllObjects]; self.glyphBytes=0;
    }
    CGColorSpaceRef space=CGColorSpaceCreateDeviceRGB();
    CGContextRef context=CGBitmapContextCreate(NULL,pixelWidth,pixelHeight,8,pixelWidth*4,space,kCGImageAlphaPremultipliedLast|kCGBitmapByteOrder32Big);
    CGColorSpaceRelease(space);
    if(!context) { CFRelease(line); [self fail:@"Text bitmap allocation failed"]; return nil; }
    CGContextScaleCTM(context,scale,scale);
    CGContextSetTextPosition(context,1,descent+1);
    CTLineDraw(line,context);
    CFRelease(line);
    MTLTextureDescriptor *descriptor=[MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA8Unorm width:pixelWidth height:pixelHeight mipmapped:NO];
    descriptor.usage=MTLTextureUsageShaderRead;
    id<MTLTexture> texture=[self.device newTextureWithDescriptor:descriptor];
    if(texture) {
        [texture replaceRegion:MTLRegionMake2D(0,0,pixelWidth,pixelHeight) mipmapLevel:0 withBytes:CGBitmapContextGetData(context) bytesPerRow:pixelWidth*4];
        self.glyphs[key]=texture;
        self.glyphBytes+=textureBytes;
    } else [self fail:@"Metal text texture allocation failed"];
    CGContextRelease(context);
    return texture;
}
- (void)drawInMTKView:(MTKView *)view {
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
        if(!slot) { self.deferred=YES; return; }
        self.deferred=NO;
        uint64_t started=nanos();
        gd_go_event(1,self.bounds.size.width,self.bounds.size.height,0,0);
        if(self.failure) { atomic_store(&slot->busy,false); return; }
        NSUInteger count=self.scene.length/sizeof(GDCommand);
        const GDCommand *commands=self.scene.bytes;
        if(count>16*1024*1024/sizeof(GDGPUInstance)) {
            atomic_store(&slot->busy,false); [self fail:@"Scene exceeds the 16 MiB instance budget"]; return;
        }
        NSUInteger needed=count*sizeof(GDGPUInstance);
        if(needed>slot.capacity) {
            slot.capacity=MIN(16*1024*1024,MAX(needed,slot.capacity*2+4096));
            slot.instances=[self.device newBufferWithLength:slot.capacity options:MTLResourceStorageModeShared|MTLResourceCPUCacheModeWriteCombined];
            if(!slot.instances) { atomic_store(&slot->busy,false); [self fail:@"Metal instance allocation failed"]; return; }
        }
        MTLRenderPassDescriptor *pass=self.currentRenderPassDescriptor;
        id<CAMetalDrawable> drawable=self.currentDrawable;
        if(!pass || !drawable) {
            atomic_store(&slot->busy,false); self.deferred=YES;
            __weak GDView *weak=self;
            dispatch_after(dispatch_time(DISPATCH_TIME_NOW,16*NSEC_PER_MSEC),dispatch_get_main_queue(),^{
                GDView *strong=weak;
                if(running && active_view==strong && strong.deferred && strong.window.visible && !strong.window.miniaturized) [strong setNeedsDisplay:YES];
            });
            return;
        }
        GDBatch *batches=calloc(count+1,sizeof(GDBatch));
        if(!batches) { atomic_store(&slot->busy,false); [self fail:@"Metal batch allocation failed"]; return; }
        NSMutableArray<id<MTLTexture>> *textures=[NSMutableArray arrayWithObject:self.white];
        GDGPUInstance *instances=slot.instances.contents;
        NSUInteger encoded=0,batchCount=0,currentTexture=0;
        CGFloat scale=drawable.texture.width/self.bounds.size.width;
        for(NSUInteger i=0;i<count;i++) {
            const GDCommand *command=&commands[i];
            if(command->kind==4 || command->clip.w<=0 || command->clip.h<=0 || (command->kind==2 && !command->text_length)) continue;
            if(command->kind==2) {
                id<MTLTexture> texture=[self glyph:command scale:scale];
                if(!texture) { free(batches); atomic_store(&slot->busy,false); return; }
                if(textures[currentTexture]!=texture) {
                    [textures addObject:texture]; currentTexture=textures.count-1;
                }
            }
            if(!batchCount || batches[batchCount-1].texture!=currentTexture) {
                batches[batchCount++]=(GDBatch){encoded,0,currentTexture};
            }
            instances[encoded++]=gd_gpu_instance(command);
            batches[batchCount-1].count++;
        }
        GDColor bg=self.background;
        pass.colorAttachments[0].clearColor=MTLClearColorMake(bg.r,bg.g,bg.b,bg.a);
        id<MTLCommandBuffer> buffer=[self.queue commandBuffer];
        id<MTLRenderCommandEncoder> encoder=[buffer renderCommandEncoderWithDescriptor:pass];
        if(!buffer || !encoder) { free(batches); atomic_store(&slot->busy,false); [self fail:@"Metal command encoding failed"]; return; }
        [encoder setRenderPipelineState:self.pipeline];
        float viewport[2]={self.bounds.size.width,self.bounds.size.height};
        [encoder setVertexBytes:viewport length:sizeof(viewport) atIndex:1];
        for(NSUInteger i=0;i<batchCount;i++) {
            GDBatch batch=batches[i];
            [encoder setVertexBuffer:slot.instances offset:batch.start*sizeof(GDGPUInstance) atIndex:0];
            [encoder setFragmentTexture:textures[batch.texture] atIndex:0];
            [encoder drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:6 instanceCount:batch.count];
        }
        free(batches);
        [encoder endEncoding];
        dispatch_group_enter(self.outstanding);
        uint32_t flight=atomic_fetch_add(&in_flight,1)+1;
        uint32_t peak=atomic_load(&max_in_flight);
        while(flight>peak && !atomic_compare_exchange_weak(&max_in_flight,&peak,flight)) {}
        atomic_fetch_or(&used_slots,(uint32_t)(1u<<slotIndex));
        atomic_store(&draw_calls,batchCount); atomic_store(&instance_count,encoded);
        atomic_store(&uploaded_bytes,encoded*sizeof(GDGPUInstance));
        uint64_t expectedGeneration=atomic_load(&generation);
        [buffer addCompletedHandler:^(id<MTLCommandBuffer> completed) {
            BOOL current=expectedGeneration==atomic_load(&generation);
            if(current && completed.status==MTLCommandBufferStatusError) {
                NSString *message=completed.error.localizedDescription ?: @"Metal submission failed";
                self.failure=message;
                dispatch_async(dispatch_get_main_queue(),^{ if(running && active_view==self) gd_quit(); });
            } else if(current) {
                atomic_fetch_add(&rendered_frames,1);
                double elapsed=completed.GPUEndTime-completed.GPUStartTime;
                if(elapsed>0) atomic_store(&gpu_nanos,(uint64_t)(elapsed*1000000000));
            }
            if(current) atomic_fetch_sub(&in_flight,1);
            atomic_store(&slot->busy,false);
            dispatch_group_leave(self.outstanding);
            dispatch_async(dispatch_get_main_queue(),^{
                if(running && active_view==self && self.deferred) { self.deferred=NO; [self setNeedsDisplay:YES]; }
            });
        }];
        [buffer presentDrawable:drawable];
        atomic_fetch_add(&submitted_frames,1);
        [buffer commit];
        atomic_store(&cpu_nanos,nanos()-started);
    }
}
@end

@implementation GDDelegate
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender { return NO; }
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender { [self.window close]; return NSTerminateCancel; }
- (void)windowDidResignKey:(NSNotification *)notification { gd_go_event(5,0,0,0,0); }
- (void)windowWillClose:(NSNotification *)notification {
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
    atomic_store(&used_slots,0); atomic_store(&in_flight,0); atomic_store(&max_in_flight,0);
    if(![NSThread isMainThread]) return "AppKit must run on the process main thread; call godesktop.Run from main";
    @autoreleasepool {
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
        MTLTextureDescriptor *whiteDescriptor=[MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA8Unorm width:1 height:1 mipmapped:NO];
        view.white=[device newTextureWithDescriptor:whiteDescriptor];
        if(!view.white) return "Metal fallback texture allocation failed";
        uint32_t white=0xffffffff;
        [view.white replaceRegion:MTLRegionMake2D(0,0,1,1) mipmapLevel:0 withBytes:&white bytesPerRow:4];
        view.background=background;
        view.glyphs=[NSMutableDictionary dictionary]; view.scene=[NSData data]; view.text=[NSData data];
        view.colorPixelFormat=MTLPixelFormatBGRA8Unorm;
        view.paused=YES; view.enableSetNeedsDisplay=YES; view.delegate=view;
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
        [NSApp activateIgnoringOtherApps:YES]; [view setNeedsDisplay:YES];
        [NSApp run];
        running=NO;
        // Only shutdown drains the queue. Normal frames never wait for the GPU.
        if(dispatch_group_wait(view.outstanding,dispatch_time(DISPATCH_TIME_NOW,5*NSEC_PER_SEC))) view.failure=@"Metal shutdown did not finish within five seconds";
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
    dispatch_async(dispatch_get_main_queue(),^{ if(running && expected==atomic_load(&generation)) [active_view setNeedsDisplay:YES]; });
}
void gd_quit(void) {
    uint64_t expected=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{ if(running && expected==atomic_load(&generation)) [active_view.window close]; });
}
uint64_t gd_rendered_frames(void) { return atomic_load(&rendered_frames); }
GDRenderStats gd_render_stats(void) {
    return (GDRenderStats){
        .backend=2,.frame_slots=3,.used_slots_mask=atomic_load(&used_slots),
        .in_flight=atomic_load(&in_flight),.max_in_flight=atomic_load(&max_in_flight),
        .submitted=atomic_load(&submitted_frames),.completed=atomic_load(&rendered_frames),
        .draw_calls=atomic_load(&draw_calls),.instances=atomic_load(&instance_count),
        .uploaded_bytes=atomic_load(&uploaded_bytes),.cpu_nanos=atomic_load(&cpu_nanos),.gpu_nanos=atomic_load(&gpu_nanos)
    };
}

void gd_window_action(int action) {
    uint64_t expected=atomic_load(&generation);
    dispatch_async(dispatch_get_main_queue(),^{ if(!running || expected!=atomic_load(&generation)) return; if(action==1) [active_view.window miniaturize:nil]; if(action==2) [active_view.window zoom:nil]; });
}
