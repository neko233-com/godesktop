//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <MetalKit/MetalKit.h>
#import <CoreText/CoreText.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>
#include <stdatomic.h>
#include "bridge.h"

typedef struct {
    float position[2], local[2], size[2], radius, color[4], uv[2];
    uint32_t textured;
} GDVertex;

static NSString *const shader = @
"#include <metal_stdlib>\n"
"using namespace metal;\n"
"struct Vertex { packed_float2 position, local, size; float radius; packed_float4 color; packed_float2 uv; uint textured; };\n"
"struct Out { float4 position [[position]]; float2 local, size; float radius; float4 color; float2 uv; uint textured [[flat]]; };\n"
"vertex Out vertex_main(uint id [[vertex_id]], const device Vertex *vertices [[buffer(0)]], constant float2 &viewport [[buffer(1)]]) {\n"
"  Vertex v = vertices[id]; Out o; o.position=float4(float2(v.position)/viewport*float2(2,-2)+float2(-1,1),0,1);\n"
"  o.local=v.local; o.size=v.size; o.radius=v.radius; o.color=v.color; o.uv=v.uv; o.textured=v.textured; return o;\n"
"}\n"
"fragment float4 fragment_main(Out in [[stage_in]], texture2d<float> glyph [[texture(0)]]) {\n"
"  if(in.textured!=0) { constexpr sampler s(filter::linear,address::clamp_to_edge); float alpha=glyph.sample(s,in.uv).a*in.color.a; return float4(in.color.rgb*alpha,alpha); }\n"
"  float2 q=abs(in.local-in.size*0.5)-(in.size*0.5-in.radius); float d=length(max(q,0.0))+min(max(q.x,q.y),0.0)-in.radius;\n"
"  float alpha=(1.0-smoothstep(-fwidth(d)*0.5,fwidth(d)*0.5,d))*in.color.a; return float4(in.color.rgb*alpha,alpha);\n"
"}\n";

@interface GDView : MTKView <MTKViewDelegate>
@property(nonatomic,strong) id<MTLCommandQueue> queue;
@property(nonatomic,strong) id<MTLRenderPipelineState> pipeline;
@property(nonatomic,strong) id<MTLTexture> white;
@property(nonatomic,strong) id<MTLBuffer> vertices;
@property(nonatomic) NSUInteger vertexCapacity;
@property(nonatomic,strong) NSMutableDictionary<NSString *,id<MTLTexture>> *glyphs;
@property(nonatomic,strong) NSData *scene;
@property(nonatomic,strong) NSData *text;
@property(nonatomic) GDColor background;
@property(nonatomic,copy) NSString *failure;
@end

@interface GDDelegate : NSObject <NSApplicationDelegate,NSWindowDelegate>
@property(nonatomic,strong) NSWindow *window;
@end

static GDView *active_view;
static BOOL running;
static _Atomic uint64_t generation;
static _Atomic uint64_t rendered_frames;

static NSString *string_utf8(const char *bytes,size_t length) {
    return [[NSString alloc] initWithBytes:bytes length:length encoding:NSUTF8StringEncoding] ?: @"";
}
static CTLineRef text_line(NSString *text,float size) {
    NSDictionary *attributes=@{NSFontAttributeName:[NSFont systemFontOfSize:size],NSForegroundColorAttributeName:[NSColor whiteColor]};
    NSAttributedString *attributed=[[NSAttributedString alloc] initWithString:text attributes:attributes];
    return CTLineCreateWithAttributedString((__bridge CFAttributedStringRef)attributed);
}
static void text_size(CTLineRef line,float *width,float *height,CGFloat *descent) {
    CGFloat ascent,leading;
    double w=CTLineGetTypographicBounds(line,&ascent,descent,&leading);
    *width=ceil(w)+2;
    *height=ceil(ascent+*descent+leading)+2;
}
static void quad(GDVertex *vertices,const GDCommand *command,BOOL textured) {
    static const float corners[6][2]={{0,0},{1,0},{0,1},{1,0},{1,1},{0,1}};
    for(int i=0;i<6;i++) {
        float u=corners[i][0],v=corners[i][1];
        GDVertex *vertex=&vertices[i];
        *vertex=(GDVertex){
            .position={command->bounds.x+u*command->bounds.w,command->bounds.y+v*command->bounds.h},
            .local={u*command->bounds.w,v*command->bounds.h},
            .size={command->bounds.w,command->bounds.h},.radius=command->radius,
            .color={command->color.r,command->color.g,command->color.b,command->color.a},
            .uv={u,v},.textured=textured?1:0
        };
    }
}

@implementation GDView
- (BOOL)acceptsFirstResponder { return YES; }
- (BOOL)isFlipped { return YES; }
- (void)mouseDown:(NSEvent *)event {
    [self.window makeFirstResponder:self];
    NSPoint p=[self convertPoint:event.locationInWindow fromView:nil];
    gd_go_event(2,p.x,p.y,0,0);
}
- (void)mouseUp:(NSEvent *)event {
    NSPoint p=[self convertPoint:event.locationInWindow fromView:nil];
    gd_go_event(3,p.x,p.y,0,0);
}
- (void)keyDown:(NSEvent *)event {
    if(event.isARepeat) return;
    int key=0;
    switch(event.keyCode) { case 48:key=9;break; case 36:case 76:key=13;break; case 49:key=32;break; case 53:key=27;break; }
    if(key) gd_go_event(4,0,0,key,(event.modifierFlags&NSEventModifierFlagShift)?1:0);
    else [super keyDown:event];
}
- (void)mtkView:(MTKView *)view drawableSizeWillChange:(CGSize)size { [self setNeedsDisplay:YES]; }
- (void)fail:(NSString *)message {
    self.failure=message;
    gd_quit();
}
- (id<MTLTexture>)glyph:(const GDCommand *)command scale:(CGFloat)scale {
    const char *bytes=(const char *)self.text.bytes+command->text_offset;
    NSString *value=string_utf8(bytes,command->text_length);
    NSString *key=[NSString stringWithFormat:@"%g/%g/%@",command->font_size,scale,value];
    id<MTLTexture> existing=self.glyphs[key];
    if(existing) return existing;
    if(self.glyphs.count>=1024) [self.glyphs removeAllObjects];
    CTLineRef line=text_line(value,command->font_size);
    float width,height; CGFloat descent;
    text_size(line,&width,&height,&descent);
    NSUInteger pixelWidth=MAX(1,ceil(width*scale)),pixelHeight=MAX(1,ceil(height*scale));
    if(pixelWidth>16384 || pixelHeight>16384) { CFRelease(line); [self fail:@"Text exceeds the Metal texture size limit"]; return nil; }
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
    } else [self fail:@"Metal text texture allocation failed"];
    CGContextRelease(context);
    return texture;
}
- (void)drawInMTKView:(MTKView *)view {
    @autoreleasepool {
        if(self.bounds.size.width<=0 || self.bounds.size.height<=0) return;
        gd_go_event(1,self.bounds.size.width,self.bounds.size.height,0,0);
        if(self.failure) return;
        NSUInteger count=self.scene.length/sizeof(GDCommand);
        const GDCommand *commands=self.scene.bytes;
        NSUInteger needed=count*6*sizeof(GDVertex);
        // Allocate once, grow geometrically. One upload per frame, no per-quad buffers.
        if(needed>self.vertexCapacity) {
            self.vertexCapacity=MAX(needed,self.vertexCapacity*2+4096);
            self.vertices=[self.device newBufferWithLength:self.vertexCapacity options:MTLResourceStorageModeShared];
            if(!self.vertices) { [self fail:@"Metal vertex allocation failed"]; return; }
        }
        GDVertex *vertices=self.vertices.contents;
        for(NSUInteger i=0;i<count;i++) quad(vertices+i*6,&commands[i],commands[i].kind==2);
        MTLRenderPassDescriptor *pass=self.currentRenderPassDescriptor;
        id<CAMetalDrawable> drawable=self.currentDrawable;
        if(!pass || !drawable) return;
        GDColor bg=self.background;
        pass.colorAttachments[0].clearColor=MTLClearColorMake(bg.r,bg.g,bg.b,bg.a);
        id<MTLCommandBuffer> buffer=[self.queue commandBuffer];
        id<MTLRenderCommandEncoder> encoder=[buffer renderCommandEncoderWithDescriptor:pass];
        if(!buffer || !encoder) { [self fail:@"Metal command encoding failed"]; return; }
        [encoder setRenderPipelineState:self.pipeline];
        [encoder setFragmentTexture:self.white atIndex:0];
        float viewport[2]={self.bounds.size.width,self.bounds.size.height};
        if(count) [encoder setVertexBuffer:self.vertices offset:0 atIndex:0];
        [encoder setVertexBytes:viewport length:sizeof(viewport) atIndex:1];
        CGFloat scale=drawable.texture.width/self.bounds.size.width;
        for(NSUInteger i=0;i<count;i++) {
            const GDCommand *command=&commands[i];
            NSUInteger x=MAX(0,floor(command->clip.x*scale)),y=MAX(0,floor(command->clip.y*scale));
            NSUInteger right=MIN(drawable.texture.width,ceil((command->clip.x+command->clip.w)*scale));
            NSUInteger bottom=MIN(drawable.texture.height,ceil((command->clip.y+command->clip.h)*scale));
            if(right<=x || bottom<=y) continue;
            [encoder setScissorRect:(MTLScissorRect){x,y,right-x,bottom-y}];
            if(command->kind==2) {
                if(!command->text_length) continue;
                id<MTLTexture> glyph=[self glyph:command scale:scale];
                if(!glyph) continue;
                [encoder setFragmentTexture:glyph atIndex:0];
            }
            [encoder drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:i*6 vertexCount:6];
        }
        [encoder endEncoding];
        [buffer presentDrawable:drawable];
        [buffer commit];
        // A single shared vertex buffer is safe only after the GPU finishes using it.
        // A triple-buffered submission ring is planned for animation workloads.
        [buffer waitUntilCompleted];
        if(buffer.status==MTLCommandBufferStatusError) [self fail:buffer.error.localizedDescription ?: @"Metal submission failed"];
        else atomic_fetch_add(&rendered_frames,1);
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

const char *gd_run(const char *title,float width,float height,GDColor background) {
    static char *last_error;
    free(last_error); last_error=NULL;
    atomic_store(&rendered_frames,0);
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
        NSWindow *window=[[NSWindow alloc] initWithContentRect:view.frame styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];
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
        running=NO; active_view=nil;
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
void gd_measure(const char *text,size_t length,float size,float *width,float *height) {
    CTLineRef line=text_line(string_utf8(text,length),size);
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
