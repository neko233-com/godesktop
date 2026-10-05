//go:build windows && cgo

#define NOMINMAX
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <windowsx.h>
#include <d2d1.h>
#include <dwrite.h>
#include <atomic>
#include <map>
#include <string>
#include <vector>
#include <cstdio>
#include "bridge.h"

namespace {
constexpr UINT wake_message = WM_APP + 1;
std::atomic<HWND> active_window{nullptr};
std::atomic<uint64_t> rendered_frames{0};

template<class T> void release(T *&value) { if (value) { value->Release(); value=nullptr; } }
std::wstring wide(const char *text, size_t length) {
    if (!length) return {};
    int count=MultiByteToWideChar(CP_UTF8,0,text,static_cast<int>(length),nullptr,0);
    std::wstring result(count,L'\0');
    MultiByteToWideChar(CP_UTF8,0,text,static_cast<int>(length),result.data(),count);
    return result;
}
float dpi(HWND window) {
    using GetDpi = UINT(WINAPI *)(HWND);
    auto fn=reinterpret_cast<GetDpi>(GetProcAddress(GetModuleHandleW(L"user32.dll"),"GetDpiForWindow"));
    return fn ? static_cast<float>(fn(window)) : 96.0f;
}
D2D1_COLOR_F color(GDColor value) { return D2D1::ColorF(value.r,value.g,value.b,value.a); }
D2D1_RECT_F rectangle(GDRect value) { return D2D1::RectF(value.x,value.y,value.x+value.w,value.y+value.h); }

struct Window {
    HWND handle=nullptr;
    ID2D1Factory *factory=nullptr;
    IDWriteFactory *text_factory=nullptr;
    ID2D1HwndRenderTarget *target=nullptr;
    ID2D1SolidColorBrush *brush=nullptr;
    GDColor background{};
    std::vector<GDCommand> commands;
    std::string text;
    std::map<std::pair<std::string,float>,IDWriteTextLayout *> layouts;
    std::string error;

    ~Window() { discard_target(); clear_text(); release(text_factory); release(factory); }
    void clear_text() { for(auto &item:layouts) release(item.second); layouts.clear(); }
    void discard_target() { release(brush); release(target); }
    void fail(const char *operation,HRESULT result) {
        char message[192];
        std::snprintf(message,sizeof(message),"%s failed (HRESULT 0x%08lx)",operation,static_cast<unsigned long>(result));
        error=message;
        if(handle) PostMessageW(handle,WM_CLOSE,0,0);
    }
    IDWriteTextLayout *layout(const char *value,size_t length,float size) {
        auto key=std::make_pair(std::string(value,length),size);
        auto existing=layouts.find(key);
        if(existing!=layouts.end()) return existing->second;
        if(layouts.size()>=1024) clear_text();
        IDWriteTextFormat *format=nullptr;
        HRESULT hr=text_factory->CreateTextFormat(L"Segoe UI",nullptr,DWRITE_FONT_WEIGHT_NORMAL,DWRITE_FONT_STYLE_NORMAL,DWRITE_FONT_STRETCH_NORMAL,size,L"",&format);
        if(FAILED(hr)) { fail("CreateTextFormat",hr); return nullptr; }
        format->SetWordWrapping(DWRITE_WORD_WRAPPING_NO_WRAP);
        auto string=wide(value,length);
        IDWriteTextLayout *result=nullptr;
        hr=text_factory->CreateTextLayout(string.data(),static_cast<UINT32>(string.size()),format,100000,100000,&result);
        release(format);
        if(FAILED(hr)) { fail("CreateTextLayout",hr); return nullptr; }
        layouts.emplace(std::move(key),result);
        return result;
    }
    bool ensure_target() {
        if(target) return true;
        RECT client{}; GetClientRect(handle,&client);
        float scale=dpi(handle);
        auto properties=D2D1::RenderTargetProperties();
        properties.dpiX=properties.dpiY=scale;
        HRESULT hr=factory->CreateHwndRenderTarget(properties,D2D1::HwndRenderTargetProperties(handle,D2D1::SizeU(client.right,client.bottom)),&target);
        if(SUCCEEDED(hr)) hr=target->CreateSolidColorBrush(D2D1::ColorF(0,0,0,1),&brush);
        if(FAILED(hr)) { discard_target(); fail("CreateHwndRenderTarget",hr); return false; }
        return true;
    }
    void draw() {
        RECT client{}; GetClientRect(handle,&client);
        if(client.right==0 || client.bottom==0) return;
        if(!ensure_target()) return;
        float scale=dpi(handle)/96.0f;
        gd_go_event(1,client.right/scale,client.bottom/scale,0,0);
        if(!error.empty()) return;
        target->BeginDraw();
        target->Clear(color(background));
        for(const auto &cmd:commands) {
            target->PushAxisAlignedClip(rectangle(cmd.clip),D2D1_ANTIALIAS_MODE_PER_PRIMITIVE);
            brush->SetColor(color(cmd.color));
            if(cmd.kind==1) {
                target->FillRoundedRectangle(D2D1::RoundedRect(rectangle(cmd.bounds),cmd.radius,cmd.radius),brush);
            } else if(cmd.kind==2 && cmd.text_length) {
                auto shaped=layout(text.data()+cmd.text_offset,cmd.text_length,cmd.font_size);
                if(shaped) target->DrawTextLayout(D2D1::Point2F(cmd.bounds.x,cmd.bounds.y),shaped,brush);
            }
            target->PopAxisAlignedClip();
        }
        HRESULT hr=target->EndDraw();
        if(hr==D2DERR_RECREATE_TARGET) { discard_target(); InvalidateRect(handle,nullptr,FALSE); }
        else if(FAILED(hr)) fail("EndDraw",hr);
        else rendered_frames.fetch_add(1);
    }
};

Window *current=nullptr; // UI-thread-only; background threads use active_window.

LRESULT CALLBACK procedure(HWND handle,UINT message,WPARAM wparam,LPARAM lparam) {
    auto window=reinterpret_cast<Window *>(GetWindowLongPtrW(handle,GWLP_USERDATA));
    if(message==WM_NCCREATE) {
        window=static_cast<Window *>(reinterpret_cast<CREATESTRUCTW *>(lparam)->lpCreateParams);
        window->handle=handle;
        SetWindowLongPtrW(handle,GWLP_USERDATA,reinterpret_cast<LONG_PTR>(window));
    }
    if(!window) return DefWindowProcW(handle,message,wparam,lparam);
    switch(message) {
    case WM_PAINT: {
        PAINTSTRUCT paint{}; BeginPaint(handle,&paint); window->draw(); EndPaint(handle,&paint); return 0;
    }
    case WM_ERASEBKGND: return 1;
    case WM_SIZE:
        if(window->target) {
            HRESULT hr=window->target->Resize(D2D1::SizeU(LOWORD(lparam),HIWORD(lparam)));
            if(FAILED(hr)) window->discard_target();
        }
        InvalidateRect(handle,nullptr,FALSE); return 0;
    case WM_DPICHANGED: {
        auto suggested=reinterpret_cast<RECT *>(lparam);
        window->discard_target();
        SetWindowPos(handle,nullptr,suggested->left,suggested->top,suggested->right-suggested->left,suggested->bottom-suggested->top,SWP_NOZORDER|SWP_NOACTIVATE);
        InvalidateRect(handle,nullptr,FALSE); return 0;
    }
    case WM_LBUTTONDOWN:
        SetFocus(handle); SetCapture(handle);
        gd_go_event(2,GET_X_LPARAM(lparam)*96.0f/dpi(handle),GET_Y_LPARAM(lparam)*96.0f/dpi(handle),0,0); return 0;
    case WM_LBUTTONUP:
        gd_go_event(3,GET_X_LPARAM(lparam)*96.0f/dpi(handle),GET_Y_LPARAM(lparam)*96.0f/dpi(handle),0,0);
        ReleaseCapture(); return 0;
    case WM_CAPTURECHANGED: case WM_KILLFOCUS:
        gd_go_event(5,0,0,0,0); return 0;
    case WM_KEYDOWN:
        if(!(lparam & (1LL<<30))) gd_go_event(4,0,0,static_cast<int>(wparam),(GetKeyState(VK_SHIFT)&0x8000)?1:0);
        return 0;
    case wake_message: InvalidateRect(handle,nullptr,FALSE); return 0;
    case WM_CLOSE: DestroyWindow(handle); return 0;
    case WM_DESTROY:
        active_window.store(nullptr); PostQuitMessage(0); return 0;
    }
    return DefWindowProcW(handle,message,wparam,lparam);
}
}

extern "C" const char *gd_run(const char *title,float width,float height,GDColor background) {
    static std::string last_error;
    last_error.clear();
    rendered_frames.store(0);
    HRESULT initialized=CoInitializeEx(nullptr,COINIT_APARTMENTTHREADED);
    if(FAILED(initialized)) return "CoInitializeEx failed: the UI thread must use a single-threaded COM apartment";
    {
        Window window;
        window.background=background;
        HRESULT hr=D2D1CreateFactory(D2D1_FACTORY_TYPE_SINGLE_THREADED,&window.factory);
        if(SUCCEEDED(hr)) hr=DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED,__uuidof(IDWriteFactory),reinterpret_cast<IUnknown **>(&window.text_factory));
        if(FAILED(hr)) { window.fail("Initialize Direct2D/DirectWrite",hr); }
        else {
            using SetAwareness=BOOL(WINAPI *)(HANDLE);
            auto set_awareness=reinterpret_cast<SetAwareness>(GetProcAddress(GetModuleHandleW(L"user32.dll"),"SetProcessDpiAwarenessContext"));
            if(set_awareness) set_awareness(reinterpret_cast<HANDLE>(-4)); else SetProcessDPIAware();
            HINSTANCE instance=GetModuleHandleW(nullptr);
            WNDCLASSW cls{};
            cls.lpfnWndProc=procedure; cls.hInstance=instance; cls.lpszClassName=L"GoDesktopNativeWindow";
            cls.hCursor=LoadCursorW(nullptr,MAKEINTRESOURCEW(32512));
            ATOM registered=RegisterClassW(&cls);
            if(!registered && GetLastError()!=ERROR_CLASS_ALREADY_EXISTS) window.error="RegisterClassW failed";
            else {
                using GetSystemDpi=UINT(WINAPI *)();
                auto get_dpi=reinterpret_cast<GetSystemDpi>(GetProcAddress(GetModuleHandleW(L"user32.dll"),"GetDpiForSystem"));
                UINT initial_dpi=get_dpi?get_dpi():96;
                RECT bounds{0,0,static_cast<LONG>(width*initial_dpi/96),static_cast<LONG>(height*initial_dpi/96)};
                using AdjustForDpi=BOOL(WINAPI *)(LPRECT,DWORD,BOOL,DWORD,UINT);
                auto adjust=reinterpret_cast<AdjustForDpi>(GetProcAddress(GetModuleHandleW(L"user32.dll"),"AdjustWindowRectExForDpi"));
                if(adjust) adjust(&bounds,WS_OVERLAPPEDWINDOW,FALSE,0,initial_dpi); else AdjustWindowRectEx(&bounds,WS_OVERLAPPEDWINDOW,FALSE,0);
                auto caption=wide(title,std::char_traits<char>::length(title));
                current=&window;
                HWND handle=CreateWindowExW(0,cls.lpszClassName,caption.c_str(),WS_OVERLAPPEDWINDOW,CW_USEDEFAULT,CW_USEDEFAULT,bounds.right-bounds.left,bounds.bottom-bounds.top,nullptr,nullptr,instance,&window);
                if(!handle) window.error="CreateWindowExW failed";
                else {
                    active_window.store(handle);
                    ShowWindow(handle,SW_SHOW); UpdateWindow(handle);
                    MSG message{};
                    BOOL result;
                    while((result=GetMessageW(&message,nullptr,0,0))>0) { TranslateMessage(&message); DispatchMessageW(&message); }
                    if(result==-1) window.error="GetMessageW failed";
                    if(IsWindow(handle)) DestroyWindow(handle);
                }
                active_window.store(nullptr);
                current=nullptr;
            }
        }
        last_error=window.error;
    }
    CoUninitialize();
    return last_error.empty()?nullptr:last_error.c_str();
}

extern "C" void gd_present(const GDCommand *commands,size_t count,const char *text,size_t length) {
    if(!current) return;
    if(count) current->commands.assign(commands,commands+count); else current->commands.clear();
    if(length) current->text.assign(text,length); else current->text.clear();
}
extern "C" void gd_measure(const char *text,size_t length,float size,float *width,float *height) {
    *width=0; *height=size*1.4f;
    if(!current) return;
    if(auto layout=current->layout(text,length,size)) {
        DWRITE_TEXT_METRICS metrics{};
        if(SUCCEEDED(layout->GetMetrics(&metrics))) { *width=metrics.widthIncludingTrailingWhitespace; *height=metrics.height; }
    }
}
extern "C" void gd_wake() { if(auto handle=active_window.load()) PostMessageW(handle,wake_message,0,0); }
extern "C" void gd_quit() { if(auto handle=active_window.load()) PostMessageW(handle,WM_CLOSE,0,0); }
extern "C" uint64_t gd_rendered_frames() { return rendered_frames.load(); }
