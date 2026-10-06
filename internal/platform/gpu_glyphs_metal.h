#ifndef GODESKTOP_GPU_GLYPHS_METAL_H
#define GODESKTOP_GPU_GLYPHS_METAL_H

// CoreText shapes the line; this cache stores individual glyph coverage only.
// GPU-private pages are updated through per-submission staging buffers.
enum { GDAtlasEdge=1024, GDAtlasMaxPages=16, GDAtlasMaxGlyphs=16384 };
typedef struct { NSUInteger start,count,texture; } GDBatch;

@interface GDAtlasUsage : NSObject { @public _Atomic uint64_t pages,peak; }
@end
@implementation GDAtlasUsage
- (instancetype)init {
    self=[super init];
    if(self) { atomic_init(&pages,0); atomic_init(&peak,0); }
    return self;
}
@end

@interface GDAtlasPage : NSObject
@property(nonatomic,strong) GDAtlasUsage *usage;
@property(nonatomic,strong) NSMutableData *pixels;
@property(nonatomic,strong) id<MTLTexture> texture;
@property(nonatomic) NSUInteger x,y,rowHeight;
@property(nonatomic) uint64_t version,uploaded;
@property(nonatomic) NSUInteger width,height,channels;
@property(nonatomic) uint64_t bitmapID;
- (instancetype)initWithUsage:(GDAtlasUsage *)usage;
- (BOOL)packWidth:(NSUInteger)width height:(NSUInteger)height left:(NSUInteger *)left top:(NSUInteger *)top;
@end
@implementation GDAtlasPage
- (instancetype)initWithUsage:(GDAtlasUsage *)usage {
    self=[super init];
    if(self) {
        self.usage=usage; self.pixels=[NSMutableData dataWithLength:GDAtlasEdge*GDAtlasEdge];
        ((unsigned char *)self.pixels.mutableBytes)[0]=255;
        self.x=1; self.y=1; self.version=1;
        self.width=GDAtlasEdge;self.height=GDAtlasEdge;self.channels=1;
        uint64_t count=atomic_fetch_add(&usage->pages,1)+1,peak=atomic_load(&usage->peak);
        while(count>peak && !atomic_compare_exchange_weak(&usage->peak,&peak,count)) {}
    }
    return self;
}
- (void)dealloc { if(_usage) atomic_fetch_sub(&_usage->pages,1); }
- (BOOL)packWidth:(NSUInteger)width height:(NSUInteger)height left:(NSUInteger *)left top:(NSUInteger *)top {
    if(width+2>GDAtlasEdge || height+2>GDAtlasEdge) return NO;
    if(self.x+width+2>GDAtlasEdge) { self.x=1; self.y+=self.rowHeight; self.rowHeight=0; }
    if(self.y+height+2>GDAtlasEdge) return NO;
    *left=self.x+1; *top=self.y+1;
    self.x+=width+2; self.rowHeight=MAX(self.rowHeight,height+2);
    return YES;
}
@end

@interface GDAtlasGlyph : NSObject
@property(nonatomic,strong) GDAtlasPage *page;
@property(nonatomic) CGFloat left,top;
@property(nonatomic) NSUInteger x,y,width,height;
@end
@implementation GDAtlasGlyph
@end

@interface GDGlyphAtlas : NSObject
@property(nonatomic,strong) id<MTLDevice> device;
@property(nonatomic,strong) GDAtlasUsage *usage;
@property(nonatomic,strong) NSMutableArray<GDAtlasPage *> *pages;
@property(nonatomic,strong) NSMutableDictionary<NSArray *,GDAtlasGlyph *> *glyphs;
@property(nonatomic) uint64_t rasterized,hits,epochs,uploadedBytes;
@property(nonatomic) uint64_t bitmapUploads,bitmapUploadedBytes;
@property(nonatomic) BOOL full;
@property(nonatomic,copy) NSString *failure;
- (instancetype)initWithDevice:(id<MTLDevice>)device;
- (GDAtlasPage *)white;
- (void)clear;
- (GDAtlasGlyph *)glyph:(CGGlyph)index font:(CTFontRef)font scale:(CGFloat)scale;
- (NSArray<id<MTLBuffer>> *)encodePages:(NSArray<GDAtlasPage *> *)pages buffer:(id<MTLCommandBuffer>)buffer;
@end
@implementation GDGlyphAtlas
- (instancetype)initWithDevice:(id<MTLDevice>)device {
    self=[super init];
    if(self) {
        self.device=device; self.usage=[[GDAtlasUsage alloc] init];
        self.pages=[NSMutableArray array]; self.glyphs=[NSMutableDictionary dictionary];
    }
    return self;
}
- (GDAtlasPage *)white {
    if(!self.pages.count) [self.pages addObject:[[GDAtlasPage alloc] initWithUsage:self.usage]];
    return self.pages[0];
}
- (void)clear {
    [self.glyphs removeAllObjects]; [self.pages removeAllObjects];
    self.full=NO; self.failure=nil; self.epochs++;
}
- (GDAtlasGlyph *)glyph:(CGGlyph)index font:(CTFontRef)font scale:(CGFloat)scale {
    // The immutable key retains the actual fallback/variation font and size.
    NSArray *key=@[(__bridge id)font,@(CTFontGetSize(font)),@(scale),@(index)];
    GDAtlasGlyph *cached=self.glyphs[key];
    if(cached) { self.hits++; return cached; }
    if(self.glyphs.count>=GDAtlasMaxGlyphs) { self.full=YES; return nil; }
    GDAtlasGlyph *glyph=[[GDAtlasGlyph alloc] init];
    CGRect bounds;
    CTFontGetBoundingRectsForGlyphs(font,kCTFontOrientationDefault,&index,&bounds,1);
    if(!CGRectIsEmpty(bounds)) {
        CGFloat left=floor(CGRectGetMinX(bounds)*scale)-1;
        CGFloat bottom=floor(CGRectGetMinY(bounds)*scale)-1;
        CGFloat right=ceil(CGRectGetMaxX(bounds)*scale)+1;
        CGFloat top=ceil(CGRectGetMaxY(bounds)*scale)+1;
        if(!isfinite(left) || !isfinite(bottom) || !isfinite(right) || !isfinite(top) || right-left>GDAtlasEdge-2 || top-bottom>GDAtlasEdge-2) {
            self.failure=@"Glyph exceeds the 1024-pixel atlas page"; return nil;
        }
        glyph.left=left; glyph.top=-top;
        glyph.width=(NSUInteger)(right-left); glyph.height=(NSUInteger)(top-bottom);
        [self white];
        NSUInteger x=0,y=0;
        for(GDAtlasPage *page in self.pages) if([page packWidth:glyph.width height:glyph.height left:&x top:&y]) { glyph.page=page; break; }
        if(!glyph.page) {
            if(self.pages.count>=GDAtlasMaxPages) { self.full=YES; return nil; }
            glyph.page=[[GDAtlasPage alloc] initWithUsage:self.usage]; [self.pages addObject:glyph.page];
            [glyph.page packWidth:glyph.width height:glyph.height left:&x top:&y];
        }
        glyph.x=x; glyph.y=y;
        // RGBA is temporary so bitmap/color-font glyphs also yield coverage.
        // The persistent CPU/GPU atlas is R8, with monochrome foreground tint.
        NSMutableData *rgba=[NSMutableData dataWithLength:glyph.width*glyph.height*4];
        CGColorSpaceRef space=CGColorSpaceCreateDeviceRGB();
        CGContextRef context=CGBitmapContextCreate(rgba.mutableBytes,glyph.width,glyph.height,8,glyph.width*4,space,kCGImageAlphaPremultipliedLast|kCGBitmapByteOrder32Big);
        CGColorSpaceRelease(space);
        if(!context) { self.failure=@"CoreText glyph bitmap allocation failed"; return nil; }
        CGContextSetAllowsFontSmoothing(context,false); CGContextSetShouldSmoothFonts(context,false);
        CGContextSetShouldAntialias(context,true); CGContextSetRGBFillColor(context,1,1,1,1);
        CGContextScaleCTM(context,scale,scale);
        CGPoint position=CGPointMake(-left/scale,-bottom/scale);
        CTFontDrawGlyphs(font,&index,&position,1,context);
        CGContextRelease(context);
        const unsigned char *source=rgba.bytes; unsigned char *dest=glyph.page.pixels.mutableBytes;
        for(NSUInteger row=0;row<glyph.height;row++) for(NSUInteger col=0;col<glyph.width;col++)
            dest[(y+row)*GDAtlasEdge+x+col]=source[(row*glyph.width+col)*4+3];
        glyph.page.version++;
    }
    self.rasterized++; self.glyphs[key]=glyph;
    return glyph;
}
- (NSArray<id<MTLBuffer>> *)encodePages:(NSArray<GDAtlasPage *> *)pages buffer:(id<MTLCommandBuffer>)buffer {
    NSMutableArray<id<MTLBuffer>> *uploads=[NSMutableArray array];
    id<MTLBlitCommandEncoder> blit=nil;
    for(GDAtlasPage *page in pages) {
        if(page.uploaded==page.version) continue;
        if(!page.texture) {
            MTLTextureDescriptor *desc=[MTLTextureDescriptor texture2DDescriptorWithPixelFormat:(page.bitmapID?MTLPixelFormatRGBA8Unorm:MTLPixelFormatR8Unorm) width:page.width height:page.height mipmapped:NO];
            desc.storageMode=MTLStorageModePrivate; desc.usage=MTLTextureUsageShaderRead;
            page.texture=[self.device newTextureWithDescriptor:desc];
        }
        const GDImage *image=page.bitmapID?gd_image_get(page.bitmapID):NULL;
        if(page.bitmapID && !image) {self.failure=@"Bitmap was evicted before texture upload";[blit endEncoding];return nil;}
        NSUInteger rowBytes=page.width*page.channels,pitch=(rowBytes+255)&~(NSUInteger)255;
        id<MTLBuffer> upload=[self.device newBufferWithLength:pitch*page.height options:MTLResourceStorageModeShared|MTLResourceCPUCacheModeWriteCombined];
        if(!page.texture || !upload) { self.failure=@"Metal glyph texture/staging allocation failed"; [blit endEncoding]; return nil; }
        const unsigned char *pixels=image?image->pixels:page.pixels.bytes;
        for(NSUInteger y=0;y<page.height;y++) memcpy((unsigned char *)upload.contents+y*pitch,pixels+y*rowBytes,rowBytes);
        if(!blit) blit=[buffer blitCommandEncoder];
        if(!blit) { self.failure=@"Metal glyph blit allocation failed"; return nil; }
        [uploads addObject:upload];
        [blit copyFromBuffer:upload sourceOffset:0 sourceBytesPerRow:pitch sourceBytesPerImage:pitch*page.height sourceSize:MTLSizeMake(page.width,page.height,1) toTexture:page.texture destinationSlice:0 destinationLevel:0 destinationOrigin:MTLOriginMake(0,0,0)];
        page.uploaded=page.version;
        if(page.bitmapID) {self.bitmapUploads++;self.bitmapUploadedBytes+=page.width*page.height*4;}
        else self.uploadedBytes+=GDAtlasEdge*GDAtlasEdge;
    }
    [blit endEncoding]; return uploads;
}
@end

@interface GDGlyphScene : NSObject
@property(nonatomic,strong) NSMutableData *instances,*batches;
@property(nonatomic,strong) NSMutableArray<GDAtlasPage *> *pages;
@property(nonatomic) NSUInteger currentPage;
@property(nonatomic,copy) NSString *failure;
- (instancetype)initWithWhite:(GDAtlasPage *)white;
- (BOOL)append:(GDGPUInstance)instance page:(GDAtlasPage *)page;
- (BOOL)appendLine:(CTLineRef)line command:(const GDCommand *)command atlas:(GDGlyphAtlas *)atlas scale:(CGFloat)scale;
@end
@implementation GDGlyphScene
- (instancetype)initWithWhite:(GDAtlasPage *)white {
    self=[super init];
    if(self) { self.instances=[NSMutableData data]; self.batches=[NSMutableData data]; self.pages=[NSMutableArray arrayWithObject:white]; }
    return self;
}
- (BOOL)append:(GDGPUInstance)instance page:(GDAtlasPage *)page {
    if(self.instances.length+sizeof(instance)>16*1024*1024) { self.failure=@"Scene exceeds the 16 MiB instance budget"; return NO; }
    if(page && self.pages[self.currentPage]!=page) {
        NSUInteger found=[self.pages indexOfObjectIdenticalTo:page];
        if(found==NSNotFound) { [self.pages addObject:page]; found=self.pages.count-1; }
        self.currentPage=found;
    }
    NSUInteger count=self.batches.length/sizeof(GDBatch);
    GDBatch *last=count?(GDBatch *)self.batches.mutableBytes+count-1:NULL;
    if(last && last->texture==self.currentPage) last->count++;
    else { GDBatch batch={self.instances.length/sizeof(instance),1,self.currentPage}; [self.batches appendBytes:&batch length:sizeof(batch)]; }
    [self.instances appendBytes:&instance length:sizeof(instance)]; return YES;
}
- (BOOL)appendLine:(CTLineRef)line command:(const GDCommand *)command atlas:(GDGlyphAtlas *)atlas scale:(CGFloat)scale {
    CGFloat ascent,descent,leading;
    CTLineGetTypographicBounds(line,&ascent,&descent,&leading);
    CGFloat baseline=command->bounds.y+ceil(ascent+leading)+1;
    CFArrayRef runs=CTLineGetGlyphRuns(line);
    for(CFIndex r=0;r<CFArrayGetCount(runs);r++) {
        CTRunRef run=CFArrayGetValueAtIndex(runs,r);
        CFIndex count=CTRunGetGlyphCount(run);
        if(count<0 || (NSUInteger)count>16*1024*1024/sizeof(GDGPUInstance)) { self.failure=@"Glyph run exceeds the scene budget"; return NO; }
        CTFontRef font=CFDictionaryGetValue(CTRunGetAttributes(run),kCTFontAttributeName);
        if(!font) { self.failure=@"CoreText run has no font"; return NO; }
        const CGGlyph *indices=CTRunGetGlyphsPtr(run);
        const CGPoint *positions=CTRunGetPositionsPtr(run);
        NSMutableData *indexCopy=nil,*positionCopy=nil;
        if(!indices) { indexCopy=[NSMutableData dataWithLength:count*sizeof(CGGlyph)]; CTRunGetGlyphs(run,CFRangeMake(0,count),indexCopy.mutableBytes); indices=indexCopy.bytes; }
        if(!positions) { positionCopy=[NSMutableData dataWithLength:count*sizeof(CGPoint)]; CTRunGetPositions(run,CFRangeMake(0,count),positionCopy.mutableBytes); positions=positionCopy.bytes; }
        for(CFIndex i=0;i<count;i++) {
            GDAtlasGlyph *glyph=[atlas glyph:indices[i] font:font scale:scale];
            if(!glyph) return NO;
            if(!glyph.page) continue;
            GDGPUInstance instance=gd_gpu_instance(command);
            instance.bounds=(GDRect){command->bounds.x+1+positions[i].x+glyph.left/scale,baseline-positions[i].y+glyph.top/scale,glyph.width/scale,glyph.height/scale};
            instance.uv=(GDRect){glyph.x/(float)GDAtlasEdge,glyph.y/(float)GDAtlasEdge,glyph.width/(float)GDAtlasEdge,glyph.height/(float)GDAtlasEdge};
            if(![self append:instance page:glyph.page]) return NO;
        }
    }
    return YES;
}
@end
#endif
