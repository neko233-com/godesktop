//go:build windows && cgo

#define NOMINMAX
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <cstdlib>
#include "dwrite_color.h"

extern "C" const char *gd_dx12_text_reference(const char *text,size_t length,const char *font,size_t font_length,
        float size,float scale,uint32_t width,uint32_t height,GDGPUSnapshot *result) {
    *result={};
    if(!width || !height || uint64_t(width)*height*4>64*1024*1024 || length>1024*1024 || font_length>1024 ||
       !std::isfinite(scale) || scale<=0 || !std::isfinite(size) || size<=0) return "Invalid Direct2D text reference bounds";
    auto wide=[](const char *value,size_t length) {
        int count=MultiByteToWideChar(CP_UTF8,MB_ERR_INVALID_CHARS,value,int(length),nullptr,0);
        std::wstring output(count,L'\0');
        if(count) MultiByteToWideChar(CP_UTF8,MB_ERR_INVALID_CHARS,value,int(length),output.data(),count);
        return output;
    };
    HRESULT apartment=CoInitializeEx(nullptr,COINIT_APARTMENTTHREADED);
    IDWriteFactory *factory=nullptr;
    IDWriteTextFormat *format=nullptr;
    IDWriteTextLayout *layout=nullptr;
    std::vector<unsigned char> pixels;
    HRESULT hr=DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED,__uuidof(IDWriteFactory),reinterpret_cast<IUnknown **>(&factory));
    auto family=font_length?wide(font,font_length):L"Segoe UI";
    if(SUCCEEDED(hr)) hr=factory->CreateTextFormat(family.c_str(),nullptr,DWRITE_FONT_WEIGHT_NORMAL,DWRITE_FONT_STYLE_NORMAL,DWRITE_FONT_STRETCH_NORMAL,size,L"",&format);
    if(SUCCEEDED(hr)) hr=format->SetWordWrapping(DWRITE_WORD_WRAPPING_NO_WRAP);
    auto string=wide(text,length);
    if(SUCCEEDED(hr)) hr=factory->CreateTextLayout(string.data(),UINT32(string.size()),format,100000,100000,&layout);
    if(SUCCEEDED(hr)) { gd_dx12::ColorRasterizer rasterizer; hr=rasterizer.reference(factory,layout,scale,width,height,pixels); }
    gd_dx12::drop(layout);gd_dx12::drop(format);gd_dx12::drop(factory);
    if(SUCCEEDED(apartment)) CoUninitialize();
    if(FAILED(hr)) return "Independent whole-layout Direct2D text reference failed";
    result->pixels=static_cast<unsigned char *>(std::malloc(pixels.size()));
    if(!result->pixels) return "Direct2D text reference allocation failed";
    std::memcpy(result->pixels,pixels.data(),pixels.size());
    result->width=width;result->height=height;result->stride=width*4;result->bytes=pixels.size();
    return nullptr;
}
