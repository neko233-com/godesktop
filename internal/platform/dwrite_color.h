#ifndef GODESKTOP_DWRITE_COLOR_H
#define GODESKTOP_DWRITE_COLOR_H

#include <dwrite_3.h>
#include <d2d1_3.h>
#include <d3d11.h>
#include "d2d_color_compat.h"
#include <cmath>
#include "dx12_device.h"

namespace gd_dx12 {
struct ColorGlyph {
    RECT bounds{};
    std::vector<unsigned char> pixels;
    bool foreground=false;
};

// This rasterizer owns a small, lazy Direct2D device on the window's UI thread.
// Only color glyph cache misses use it; D3D12 remains the window renderer.
class ColorRasterizer {
    IDWriteFactory4 *factory=nullptr;
#if defined(__IDWriteFactory8_INTERFACE_DEFINED__)
    IDWriteFactory8 *paintFactory=nullptr;
    PaintContext7 *paintContext=nullptr;
#endif
    ID3D11Device *device=nullptr;
    ID2D1Factory1 *d2d=nullptr;
    ID2D1Device *d2dDevice=nullptr;
    ID2D1DeviceContext4 *context=nullptr;
    ID2D1SolidColorBrush *brush=nullptr;
    bool initialized=false;
    bool init(IDWriteFactory *source) {
        if(initialized) return context!=nullptr;
        initialized=true;
        if(FAILED(source->QueryInterface(IID_PPV_ARGS(&factory)))) return false;
        trace_stage("color D3D11 WARP creation begin");
        ID3D11DeviceContext *immediate=nullptr;
        HRESULT hr=D3D11CreateDevice(nullptr,D3D_DRIVER_TYPE_WARP,nullptr,D3D11_CREATE_DEVICE_BGRA_SUPPORT,
            nullptr,0,D3D11_SDK_VERSION,&device,nullptr,&immediate);
        drop(immediate);
        trace_stage("color D3D11 WARP creation returned");
        if(FAILED(hr)) return false;
        IDXGIDevice *dxgi=nullptr;
        ID2D1DeviceContext *base=nullptr;
        hr=device->QueryInterface(IID_PPV_ARGS(&dxgi));
        if(SUCCEEDED(hr)) hr=D2D1CreateFactory(D2D1_FACTORY_TYPE_SINGLE_THREADED,__uuidof(ID2D1Factory1),nullptr,reinterpret_cast<void **>(&d2d));
        if(SUCCEEDED(hr)) hr=d2d->CreateDevice(dxgi,&d2dDevice);
        if(SUCCEEDED(hr)) hr=d2dDevice->CreateDeviceContext(D2D1_DEVICE_CONTEXT_OPTIONS_NONE,&base);
        if(SUCCEEDED(hr)) hr=base->QueryInterface(IID_PPV_ARGS(&context));
#if defined(__IDWriteFactory8_INTERFACE_DEFINED__)
        if(SUCCEEDED(hr)) {
            base->QueryInterface(paintContextIID,reinterpret_cast<void **>(&paintContext));
            source->QueryInterface(IID_PPV_ARGS(&paintFactory));
        }
#endif
        drop(base); drop(dxgi);
        if(SUCCEEDED(hr)) hr=context->CreateSolidColorBrush(D2D1::ColorF(1,1,1,1),&brush);
        if(FAILED(hr)) { drop(context); return false; }
        context->SetDpi(96,96);
        context->SetTextAntialiasMode(D2D1_TEXT_ANTIALIAS_MODE_GRAYSCALE);
        return true;
    }
    HRESULT translate(const DWRITE_GLYPH_RUN &run,DWRITE_MEASURING_MODE mode,IDWriteColorGlyphRunEnumerator1 **out) {
        HRESULT hr=S_OK;
        constexpr unsigned supported=DWRITE_GLYPH_IMAGE_FORMATS_TRUETYPE|DWRITE_GLYPH_IMAGE_FORMATS_CFF|
            DWRITE_GLYPH_IMAGE_FORMATS_COLR|DWRITE_GLYPH_IMAGE_FORMATS_SVG|DWRITE_GLYPH_IMAGE_FORMATS_PNG|
            DWRITE_GLYPH_IMAGE_FORMATS_JPEG|DWRITE_GLYPH_IMAGE_FORMATS_TIFF|DWRITE_GLYPH_IMAGE_FORMATS_PREMULTIPLIED_B8G8R8A8;
#if defined(__IDWriteFactory8_INTERFACE_DEFINED__)
        if(paintContext && paintFactory) {
            hr=paintFactory->TranslateColorGlyphRun(D2D1::Point2F(0,0),&run,nullptr,
                static_cast<DWRITE_GLYPH_IMAGE_FORMATS>(supported|DWRITE_GLYPH_IMAGE_FORMATS_COLR_PAINT_TREE),
                paintContext->GetPaintFeatureLevel(),mode,nullptr,0,out);
        } else
#endif
            hr=factory->TranslateColorGlyphRun(D2D1::Point2F(0,0),&run,nullptr,
                static_cast<DWRITE_GLYPH_IMAGE_FORMATS>(supported),mode,nullptr,0,out);
        return hr;
    }
    HRESULT drawRuns(IDWriteColorGlyphRunEnumerator1 *runs,GDColor foreground,ColorGlyph &result,unsigned &layers) {
        HRESULT hr=S_OK; BOOL next=FALSE; layers=0;
        while(SUCCEEDED(hr=runs->MoveNext(&next)) && next) {
            if(++layers>256) { hr=E_OUTOFMEMORY; break; }
            const DWRITE_COLOR_GLYPH_RUN1 *layer=nullptr;
            hr=runs->GetCurrentRun(&layer);
            if(FAILED(hr)) break;
            if(layer->glyphRun.glyphCount>4096) { hr=E_OUTOFMEMORY; break; }
            auto origin=D2D1::Point2F(layer->baselineOriginX,layer->baselineOriginY);
            unsigned format=layer->glyphImageFormat;
#if defined(__IDWriteFactory8_INTERFACE_DEFINED__)
            if(format==DWRITE_GLYPH_IMAGE_FORMATS_COLR_PAINT_TREE) {
                IDWriteFontFace7 *paintFace=nullptr;
                IDWritePaintReader *reader=nullptr;
                HRESULT attributes=layer->glyphRun.fontFace->QueryInterface(IID_PPV_ARGS(&paintFace));
                if(SUCCEEDED(attributes)) attributes=paintFace->CreatePaintReader(DWRITE_GLYPH_IMAGE_FORMATS_COLR_PAINT_TREE,paintContext->GetPaintFeatureLevel(),&reader);
                for(unsigned i=0;SUCCEEDED(attributes) && i<layer->glyphRun.glyphCount;i++) {
                    DWRITE_PAINT_ELEMENT element{}; D2D_RECT_F clip{}; DWRITE_PAINT_ATTRIBUTES flags{};
                    attributes=reader->SetCurrentGlyph(layer->glyphRun.glyphIndices[i],&element,sizeof(element),&clip,&flags);
                    if(flags&DWRITE_PAINT_ATTRIBUTES_USES_TEXT_COLOR) result.foreground=true;
                }
                // An unknown paint dependency uses a foreground-specific key.
                if(FAILED(attributes)) result.foreground=true;
                drop(reader); drop(paintFace);
                brush->SetColor(D2D1::ColorF(foreground.r,foreground.g,foreground.b,1));
                paintContext->DrawPaintGlyphRun(origin,&layer->glyphRun,brush,0,layer->measuringMode);
            } else
#endif
            if(format & (DWRITE_GLYPH_IMAGE_FORMATS_PNG|DWRITE_GLYPH_IMAGE_FORMATS_JPEG|
                         DWRITE_GLYPH_IMAGE_FORMATS_TIFF|DWRITE_GLYPH_IMAGE_FORMATS_PREMULTIPLIED_B8G8R8A8)) {
                context->DrawColorBitmapGlyphRun(layer->glyphImageFormat,origin,&layer->glyphRun,layer->measuringMode);
            } else if(format==DWRITE_GLYPH_IMAGE_FORMATS_SVG) {
                // SVG currentColor can depend on the caller's foreground.
                result.foreground=true;
                brush->SetColor(D2D1::ColorF(foreground.r,foreground.g,foreground.b,1));
                context->DrawSvgGlyphRun(origin,&layer->glyphRun,brush,nullptr,0,layer->measuringMode);
            } else {
                if(layer->paletteIndex==0xffff) {
                    result.foreground=true;
                    brush->SetColor(D2D1::ColorF(foreground.r,foreground.g,foreground.b,1));
                } else brush->SetColor(D2D1::ColorF(layer->runColor.r,layer->runColor.g,layer->runColor.b,layer->runColor.a));
                context->DrawGlyphRun(origin,&layer->glyphRun,brush,layer->measuringMode);
            }
        }
        return hr;
    }

public:
    ~ColorRasterizer() { reset(); }
    void reset() {
#if defined(__IDWriteFactory8_INTERFACE_DEFINED__)
        drop(paintContext); drop(paintFactory);
#endif
        drop(brush); drop(context); drop(d2dDevice); drop(d2d); drop(device); drop(factory);
        initialized=false;
    }
    // Independent acceptance oracle: Direct2D draws the entire text layout with
    // ENABLE_COLOR_FONT, without our glyph translation, packing or GPU shader.
    HRESULT reference(IDWriteFactory *source,IDWriteTextLayout *layout,float scale,
                      unsigned width,unsigned height,std::vector<unsigned char> &pixels) {
        if(!init(source)) return E_FAIL;
        ID2D1Bitmap1 *target=nullptr,*readback=nullptr;
        D2D1_BITMAP_PROPERTIES1 properties{};
        properties.pixelFormat={DXGI_FORMAT_B8G8R8A8_UNORM,D2D1_ALPHA_MODE_PREMULTIPLIED};
        properties.dpiX=properties.dpiY=96;
        properties.bitmapOptions=D2D1_BITMAP_OPTIONS_TARGET|D2D1_BITMAP_OPTIONS_CANNOT_DRAW;
        HRESULT hr=context->CreateBitmap(D2D1::SizeU(width,height),nullptr,0,&properties,&target);
        properties.bitmapOptions=D2D1_BITMAP_OPTIONS_CPU_READ|D2D1_BITMAP_OPTIONS_CANNOT_DRAW;
        if(SUCCEEDED(hr)) hr=context->CreateBitmap(D2D1::SizeU(width,height),nullptr,0,&properties,&readback);
        if(SUCCEEDED(hr)) {
            context->SetTarget(target); context->SetTransform(D2D1::Matrix3x2F::Scale(scale,scale));
            brush->SetColor(D2D1::ColorF(1,0,0,1));
            context->BeginDraw(); context->Clear(D2D1::ColorF(0,0,0,1));
            context->DrawTextLayout(D2D1::Point2F(0,0),layout,brush,
                static_cast<D2D1_DRAW_TEXT_OPTIONS>(D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT|D2D1_DRAW_TEXT_OPTIONS_NO_SNAP));
            hr=context->EndDraw(); context->SetTarget(nullptr);
        }
        if(SUCCEEDED(hr)) hr=readback->CopyFromBitmap(nullptr,target,nullptr);
        D2D1_MAPPED_RECT mapped{};
        if(SUCCEEDED(hr)) hr=readback->Map(D2D1_MAP_OPTIONS_READ,&mapped);
        if(SUCCEEDED(hr)) {
            pixels.resize(size_t(width)*height*4);
            for(unsigned y=0;y<height;y++) for(unsigned x=0;x<width;x++) {
                auto p=mapped.bits+y*mapped.pitch+x*4;
                auto dest=pixels.data()+(size_t(y)*width+x)*4;
                dest[0]=p[2];dest[1]=p[1];dest[2]=p[0];dest[3]=p[3];
            }
            hr=readback->Unmap();
        }
        drop(readback);drop(target);return hr;
    }
    // S_FALSE means a monochrome glyph. Failures must not silently discard color.
    HRESULT rasterize(IDWriteFactory *source,const DWRITE_GLYPH_RUN &run,float scale,
                      DWRITE_MEASURING_MODE mode,GDColor foreground,float phaseX,float phaseY,ColorGlyph &result) {
        if(!init(source)) return E_FAIL;
        HRESULT hr=S_OK;
        IDWriteColorGlyphRunEnumerator1 *runs=nullptr;
        hr=translate(run,mode,&runs);
        if(hr==DWRITE_E_NOCOLOR) return S_FALSE;
        if(FAILED(hr)) return hr;
        ID2D1CommandList *commands=nullptr;
        hr=context->CreateCommandList(&commands);
        if(FAILED(hr)) { drop(runs); return hr; }
        context->SetTarget(commands);
        context->SetTransform(D2D1::Matrix3x2F::Identity());
        context->BeginDraw();
        unsigned layers=0;
        hr=drawRuns(runs,foreground,result,layers);
        HRESULT drawn=context->EndDraw();
        context->SetTarget(nullptr); drop(runs);
        if(SUCCEEDED(hr)) hr=drawn;
        if(SUCCEEDED(hr)) hr=commands->Close();
        D2D1_RECT_F bounds{};
        if(SUCCEEDED(hr)) hr=context->GetImageLocalBounds(commands,&bounds);
        if(FAILED(hr)) { drop(commands); return hr; }
        if(!layers || bounds.right<=bounds.left || bounds.bottom<=bounds.top) { drop(commands); return S_OK; }
        double left=std::floor(double(bounds.left)*scale+phaseX)-1,top=std::floor(double(bounds.top)*scale+phaseY)-1;
        double right=std::ceil(double(bounds.right)*scale+phaseX)+1,bottom=std::ceil(double(bounds.bottom)*scale+phaseY)+1;
        if(!std::isfinite(left) || !std::isfinite(top) || !std::isfinite(right) || !std::isfinite(bottom) ||
           std::abs(left)>1048576 || std::abs(top)>1048576 || right-left>1022 || bottom-top>1022) {
            drop(commands); return E_OUTOFMEMORY;
        }
        result.bounds={LONG(left),LONG(top),LONG(right),LONG(bottom)};
        unsigned width=result.bounds.right-result.bounds.left,height=result.bounds.bottom-result.bounds.top;
        ID2D1Bitmap1 *target=nullptr,*readback=nullptr;
        D2D1_BITMAP_PROPERTIES1 properties{};
        properties.pixelFormat={DXGI_FORMAT_B8G8R8A8_UNORM,D2D1_ALPHA_MODE_PREMULTIPLIED};
        properties.dpiX=properties.dpiY=96;
        properties.bitmapOptions=D2D1_BITMAP_OPTIONS_TARGET|D2D1_BITMAP_OPTIONS_CANNOT_DRAW;
        hr=context->CreateBitmap(D2D1::SizeU(width,height),nullptr,0,&properties,&target);
        properties.bitmapOptions=D2D1_BITMAP_OPTIONS_CPU_READ|D2D1_BITMAP_OPTIONS_CANNOT_DRAW;
        if(SUCCEEDED(hr)) hr=context->CreateBitmap(D2D1::SizeU(width,height),nullptr,0,&properties,&readback);
        if(SUCCEEDED(hr)) {
            context->SetTarget(target); context->SetTransform(D2D1::Matrix3x2F(scale,0,0,scale,-float(left)+phaseX,-float(top)+phaseY));
            hr=translate(run,mode,&runs);
            if(SUCCEEDED(hr)) {
                context->BeginDraw(); context->Clear(D2D1::ColorF(0,0,0,0));
                // Draw native layers directly at the final physical phase.
                // The command list above is only the independent bounds prepass.
                unsigned renderedLayers=0;
                HRESULT rendered=drawRuns(runs,foreground,result,renderedLayers);
                HRESULT ended=context->EndDraw(); hr=FAILED(rendered)?rendered:ended;
            }
            context->SetTarget(nullptr); drop(runs);
        }
        if(SUCCEEDED(hr)) hr=readback->CopyFromBitmap(nullptr,target,nullptr);
        D2D1_MAPPED_RECT mapped{};
        if(SUCCEEDED(hr)) hr=readback->Map(D2D1_MAP_OPTIONS_READ,&mapped);
        if(SUCCEEDED(hr)) {
            result.pixels.resize(size_t(width)*height*4);
            for(unsigned y=0;y<height;y++) for(unsigned x=0;x<width;x++) {
                auto sourcePixel=mapped.bits+y*mapped.pitch+x*4;
                auto dest=result.pixels.data()+(size_t(y)*width+x)*4;
                dest[0]=sourcePixel[2]; dest[1]=sourcePixel[1]; dest[2]=sourcePixel[0]; dest[3]=sourcePixel[3];
            }
            hr=readback->Unmap();
        }
        drop(readback); drop(target); drop(commands);
        return hr;
    }
};
}
#endif
