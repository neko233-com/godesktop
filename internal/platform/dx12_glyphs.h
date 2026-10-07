#ifndef GODESKTOP_DX12_GLYPHS_H
#define GODESKTOP_DX12_GLYPHS_H

#include <dwrite.h>
#include <map>
#include <memory>
#include <tuple>
#include "dx12_device.h"
#include "image_store.h"
#include "dwrite_color.h"

namespace gd_dx12 {
struct AtlasAccounting { uint64_t pages=0,peakPages=0,bytes=0,peakBytes=0; };
struct AtlasPage {
    static constexpr unsigned edge=1024;
    std::vector<unsigned char> pixels;
    unsigned width=edge,height=edge,channels=1;
    uint64_t bitmapID=0;
    unsigned x=1,y=1,rowHeight=0;
    uint64_t version=1,uploaded=0;
    ID3D12Resource *texture=nullptr;
    bool shaderState=false;
    std::shared_ptr<AtlasAccounting> accounting;
    explicit AtlasPage(std::shared_ptr<AtlasAccounting> usage,unsigned size=edge,unsigned components=1):
        pixels(size_t(size)*size*components,0),width(size),height(size),channels(components),accounting(std::move(usage)) {
        for(unsigned i=0;i<channels;i++) pixels[i]=255;
        accounting->pages++; accounting->bytes+=pixels.size();
        accounting->peakPages=std::max(accounting->peakPages,accounting->pages);
        accounting->peakBytes=std::max(accounting->peakBytes,accounting->bytes);
    }
    explicit AtlasPage(const GDImage &image):width(image.width),height(image.height),channels(4),bitmapID(image.id) {}
    ~AtlasPage() { drop(texture); if(accounting) { accounting->pages--; accounting->bytes-=pixels.size(); } }
    bool pack(unsigned glyphWidth,unsigned glyphHeight,unsigned *left,unsigned *top) {
        if(glyphWidth+2>width || glyphHeight+2>height) return false;
        if(x+glyphWidth+2>width) { x=1; y+=rowHeight; rowHeight=0; }
        if(y+glyphHeight+2>height) return false;
        *left=x+1; *top=y+1;
        x+=glyphWidth+2; rowHeight=std::max(rowHeight,glyphHeight+2);
        return true;
    }
};
struct Glyph {
    std::shared_ptr<AtlasPage> page;
    RECT bounds{};
    unsigned x=0,y=0,width=0,height=0;
};
class GlyphAtlas {
    // Font faces stay retained while their identity is part of a cache key.
    using Key=std::tuple<IDWriteFontFace *,float,float,UINT16,bool,unsigned,float,DWRITE_MEASURING_MODE,bool,float,float,float>;
    std::map<Key,Glyph> glyphs;
    std::map<IDWriteFontFace *,IDWriteFontFace *> faces;
    ColorRasterizer colors;
public:
    static constexpr unsigned maxPages=16,maxGlyphs=16384;
    std::shared_ptr<AtlasAccounting> accounting=std::make_shared<AtlasAccounting>();
    std::vector<std::shared_ptr<AtlasPage>> pages;
    uint64_t rasterized=0,hits=0,epochs=0;
    bool full=false;
    std::string error;
    ~GlyphAtlas() { clear(); }
    size_t entries() const { return glyphs.size(); }
    void resetColorDevice() { colors.reset(); }
    uint64_t activeBytes() const { uint64_t total=0; for(const auto &page:pages) total+=page->pixels.size(); return total; }
    void clear() {
        glyphs.clear(); pages.clear();
        for(auto &face:faces) drop(face.second);
        faces.clear(); full=false; epochs++;
    }
    std::shared_ptr<AtlasPage> white() {
        if(pages.empty()) pages.push_back(std::make_shared<AtlasPage>(accounting));
        return pages[0];
    }
    const Glyph *get(IDWriteFactory *factory,const DWRITE_GLYPH_RUN *run,unsigned index,float scale,DWRITE_MEASURING_MODE mode,GDColor foreground) {
        float advance=run->glyphAdvances?run->glyphAdvances[index]:0;
        auto key=Key{run->fontFace,run->fontEmSize,scale,run->glyphIndices[index],run->isSideways!=FALSE,run->bidiLevel&1u,(run->bidiLevel&1)?advance:0,mode,false,0,0,0};
        auto existing=glyphs.find(key);
        if(existing!=glyphs.end()) { hits++; return &existing->second; }
        auto coloredKey=key;
        std::get<8>(coloredKey)=true; std::get<9>(coloredKey)=foreground.r; std::get<10>(coloredKey)=foreground.g; std::get<11>(coloredKey)=foreground.b;
        existing=glyphs.find(coloredKey);
        if(existing!=glyphs.end()) { hits++; return &existing->second; }
        if(glyphs.size()>=maxGlyphs) { full=true; return nullptr; }
        DWRITE_GLYPH_RUN single=*run;
        single.glyphCount=1; single.glyphIndices=run->glyphIndices+index;
        single.glyphAdvances=run->glyphAdvances?&advance:nullptr;
        single.glyphOffsets=nullptr;
        ColorGlyph color;
        HRESULT hr=colors.rasterize(factory,single,scale,mode,foreground,color);
        if(FAILED(hr)) { error="Native color glyph rasterization failed (HRESULT="+std::to_string(uint32_t(hr))+")"; return nullptr; }
        bool rgba=hr==S_OK;
        IDWriteGlyphRunAnalysis *analysis=nullptr;
        Glyph glyph;
        if(rgba) glyph.bounds=color.bounds;
        else {
            hr=factory->CreateGlyphRunAnalysis(&single,scale,nullptr,DWRITE_RENDERING_MODE_NATURAL_SYMMETRIC,mode,0,0,&analysis);
            if(SUCCEEDED(hr)) hr=analysis->GetAlphaTextureBounds(DWRITE_TEXTURE_CLEARTYPE_3x1,&glyph.bounds);
        }
        if(FAILED(hr)) { drop(analysis); error="DirectWrite glyph analysis failed"; return nullptr; }
        glyph.width=std::max(0L,glyph.bounds.right-glyph.bounds.left);
        glyph.height=std::max(0L,glyph.bounds.bottom-glyph.bounds.top);
        if(glyph.width && glyph.height) {
            if(glyph.width+2>AtlasPage::edge || glyph.height+2>AtlasPage::edge) { drop(analysis); error="Glyph exceeds the 1024-pixel atlas page"; return nullptr; }
            white();
            unsigned channels=rgba?4:1;
            for(auto &page:pages) if(page->channels==channels && page->pack(glyph.width,glyph.height,&glyph.x,&glyph.y)) { glyph.page=page; break; }
            if(!glyph.page) {
                unsigned edge=rgba && glyph.width+2<=512 && glyph.height+2<=512?512:AtlasPage::edge;
                if(pages.size()>=maxPages || activeBytes()+uint64_t(edge)*edge*channels>16*1024*1024) { full=true; drop(analysis); return nullptr; }
                glyph.page=std::make_shared<AtlasPage>(accounting,edge,channels); pages.push_back(glyph.page);
                glyph.page->pack(glyph.width,glyph.height,&glyph.x,&glyph.y);
            }
            std::vector<BYTE> coverage;
            if(!rgba) {
                coverage.resize(glyph.width*glyph.height*3);
                hr=analysis->CreateAlphaTexture(DWRITE_TEXTURE_CLEARTYPE_3x1,&glyph.bounds,coverage.data(),coverage.size());
            }
            if(FAILED(hr)) { drop(analysis); error="DirectWrite glyph rasterization failed"; return nullptr; }
            for(unsigned y=0;y<glyph.height;y++) for(unsigned x=0;x<glyph.width;x++) {
                size_t dest=(size_t(glyph.y+y)*glyph.page->width+glyph.x+x)*channels;
                if(rgba) std::memcpy(glyph.page->pixels.data()+dest,color.pixels.data()+(size_t(y)*glyph.width+x)*4,4);
                else {
                    size_t source=(y*glyph.width+x)*3;
                    glyph.page->pixels[dest]=static_cast<unsigned char>((unsigned(coverage[source])+coverage[source+1]+coverage[source+2]+1)/3);
                }
            }
            glyph.page->version++;
        }
        drop(analysis); rasterized++;
        if(!faces.count(run->fontFace)) { run->fontFace->AddRef(); faces.emplace(run->fontFace,run->fontFace); }
        return &glyphs.emplace(color.foreground?std::move(coloredKey):std::move(key),std::move(glyph)).first->second;
    }
};
struct Batch { unsigned start=0,count=0,page=0; };
struct Scene {
    std::vector<GDGPUInstance> instances;
    std::vector<Batch> batches;
    std::vector<std::shared_ptr<AtlasPage>> pages;
    unsigned currentPage=0;
    void reset(std::shared_ptr<AtlasPage> white) {
        instances.clear(); batches.clear(); pages.clear(); pages.push_back(std::move(white)); currentPage=0;
    }
    void append(GDGPUInstance instance,std::shared_ptr<AtlasPage> page=nullptr) {
        if(page) {
            auto existing=std::find(pages.begin(),pages.end(),page);
            unsigned index;
            if(existing==pages.end()) { index=pages.size(); pages.push_back(page); }
            else index=existing-pages.begin();
            if(page->bitmapID) currentPage=index;
            else {
                unsigned slot=0;
                for(unsigned i=0;i<index;i++) if(!pages[i]->bitmapID) slot++;
                instance.padding[0]=float(slot); currentPage=0;
            }
        }
        if(batches.empty() || batches.back().page!=currentPage) batches.push_back(Batch{static_cast<unsigned>(instances.size()),0,currentPage});
        instances.push_back(instance); batches.back().count++;
    }
};
class TextRenderer final : public IDWriteTextRenderer {
    ULONG references=1;
public:
    IDWriteFactory *factory;
    GlyphAtlas &atlas;
    Scene &scene;
    const GDCommand &command;
    float scale;
    TextRenderer(IDWriteFactory *f,GlyphAtlas &a,Scene &s,const GDCommand &c,float pixelsPerDip):factory(f),atlas(a),scene(s),command(c),scale(pixelsPerDip) {}
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid,void **out) override {
        if(!out) return E_POINTER;
        *out=nullptr;
        if(iid==__uuidof(IUnknown) || iid==__uuidof(IDWritePixelSnapping) || iid==__uuidof(IDWriteTextRenderer)) { *out=static_cast<IDWriteTextRenderer *>(this); AddRef(); return S_OK; }
        return E_NOINTERFACE;
    }
    ULONG STDMETHODCALLTYPE AddRef() override { return ++references; }
    ULONG STDMETHODCALLTYPE Release() override { return --references; }
    HRESULT STDMETHODCALLTYPE IsPixelSnappingDisabled(void *,BOOL *disabled) override { *disabled=TRUE; return S_OK; }
    HRESULT STDMETHODCALLTYPE GetCurrentTransform(void *,DWRITE_MATRIX *matrix) override { *matrix={1,0,0,1,0,0}; return S_OK; }
    HRESULT STDMETHODCALLTYPE GetPixelsPerDip(void *,FLOAT *value) override { *value=scale; return S_OK; }
    HRESULT STDMETHODCALLTYPE DrawGlyphRun(void *,FLOAT originX,FLOAT originY,DWRITE_MEASURING_MODE mode,const DWRITE_GLYPH_RUN *run,const DWRITE_GLYPH_RUN_DESCRIPTION *,IUnknown *) override {
        float advance=0;
        bool rtl=(run->bidiLevel&1)!=0;
        for(unsigned i=0;i<run->glyphCount;i++) {
            auto glyph=atlas.get(factory,run,i,scale,mode,command.color);
            if(!glyph) return E_FAIL;
            if(glyph->page) {
                auto offset=run->glyphOffsets?run->glyphOffsets[i]:DWRITE_GLYPH_OFFSET{};
                float x=originX+(rtl?-advance:advance)+(rtl?-offset.advanceOffset:offset.advanceOffset);
                float y=originY-offset.ascenderOffset;
                GDGPUInstance v=gd_gpu_instance(&command);
                v.bounds={x+glyph->bounds.left/scale,y+glyph->bounds.top/scale,glyph->width/scale,glyph->height/scale};
                v.uv={glyph->x/float(glyph->page->width),glyph->y/float(glyph->page->height),glyph->width/float(glyph->page->width),glyph->height/float(glyph->page->height)};
                if(glyph->page->channels==4) v.kind=6;
                scene.append(v,glyph->page);
            }
            if(run->glyphAdvances) advance+=run->glyphAdvances[i];
        }
        return S_OK;
    }
    HRESULT decoration(float x,float y,float width,float thickness,float offset) {
        auto v=gd_gpu_instance(&command); v.kind=1; v.radius=0; v.bounds={x,y+offset,width,thickness}; scene.append(v); return S_OK;
    }
    HRESULT STDMETHODCALLTYPE DrawUnderline(void *,FLOAT x,FLOAT y,const DWRITE_UNDERLINE *line,IUnknown *) override { return decoration(x,y,line->width,line->thickness,line->offset); }
    HRESULT STDMETHODCALLTYPE DrawStrikethrough(void *,FLOAT x,FLOAT y,const DWRITE_STRIKETHROUGH *line,IUnknown *) override { return decoration(x,y,line->width,line->thickness,line->offset); }
    HRESULT STDMETHODCALLTYPE DrawInlineObject(void *,FLOAT,FLOAT,IDWriteInlineObject *,BOOL,BOOL,IUnknown *) override { return E_NOTIMPL; }
};
}
#endif
