#ifndef GODESKTOP_D2D_COLOR_COMPAT_H
#define GODESKTOP_D2D_COLOR_COMPAT_H

// Private API declarations for SDKs that expose DirectWrite paint trees but stop
// at ID2D1DeviceContext5. Runtime QueryInterface gates their use. Method order,
// signatures and IID are checked against Microsoft's Win32 metadata SDK header:
// https://github.com/microsoft/win32metadata/blob/75ec935a4f1ac56345daa9f7f0bc21db3e68ac68/generation/WinSDK/RecompiledIdlHeaders/um/d2d1_3.h
// These names remain private even when a newer installed SDK defines Context7.
#if defined(__IDWriteFactory8_INTERFACE_DEFINED__)
namespace gd_dx12 {
struct PaintContext6 : ID2D1DeviceContext5 {
    virtual void STDMETHODCALLTYPE BlendImage(ID2D1Image *,D2D1_BLEND_MODE,const D2D1_POINT_2F *,
        const D2D1_RECT_F *,D2D1_INTERPOLATION_MODE)=0;
};
struct PaintContext7 : PaintContext6 {
    virtual DWRITE_PAINT_FEATURE_LEVEL STDMETHODCALLTYPE GetPaintFeatureLevel()=0;
    virtual void STDMETHODCALLTYPE DrawPaintGlyphRun(D2D1_POINT_2F,const DWRITE_GLYPH_RUN *,ID2D1Brush *,UINT32,DWRITE_MEASURING_MODE)=0;
    virtual void STDMETHODCALLTYPE DrawGlyphRunWithColorSupport(D2D1_POINT_2F,const DWRITE_GLYPH_RUN *,
        const DWRITE_GLYPH_RUN_DESCRIPTION *,ID2D1Brush *,ID2D1SvgGlyphStyle *,UINT32,DWRITE_MEASURING_MODE,D2D1_COLOR_BITMAP_GLYPH_SNAP_OPTION)=0;
};
constexpr GUID paintContextIID={0xec891cf7,0x9b69,0x4851,{0x9d,0xef,0x4e,0x09,0x15,0x77,0x1e,0x62}};
}
#endif
#endif
