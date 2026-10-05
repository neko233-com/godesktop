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
#include <tuple>
#include <algorithm>
#include "bridge.h"

namespace {
constexpr UINT wake_message = WM_APP + 1;
constexpr UINT action_message = WM_APP + 2;
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
    bool custom_titlebar=false;
    unsigned high_surrogate=0;
    bool readback=false;
    std::vector<GDCommand> commands;
    std::string text;
    std::map<std::tuple<std::string,float,std::string>,IDWriteTextLayout *> layouts;
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
    IDWriteTextLayout *layout(const char *value,size_t length,float size,const char *font,size_t font_length) {
        std::string family=font_length?std::string(font,font_length):"Segoe UI";
        auto key=std::make_tuple(std::string(value,length),size,family);
        auto existing=layouts.find(key);
        if(existing!=layouts.end()) return existing->second;
        if(layouts.size()>=1024) clear_text();
        IDWriteTextFormat *format=nullptr;
        auto font_name=wide(family.data(),family.size());
        HRESULT hr=text_factory->CreateTextFormat(font_name.c_str(),nullptr,DWRITE_FONT_WEIGHT_NORMAL,DWRITE_FONT_STYLE_NORMAL,DWRITE_FONT_STRETCH_NORMAL,size,L"",&format);
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
        if(readback) {
            properties.usage=D2D1_RENDER_TARGET_USAGE_GDI_COMPATIBLE;
            properties.pixelFormat=D2D1::PixelFormat(DXGI_FORMAT_B8G8R8A8_UNORM,D2D1_ALPHA_MODE_IGNORE);
        }
        properties.dpiX=properties.dpiY=scale;
        auto hwnd_properties=D2D1::HwndRenderTargetProperties(handle,D2D1::SizeU(client.right,client.bottom),readback?D2D1_PRESENT_OPTIONS_RETAIN_CONTENTS:D2D1_PRESENT_OPTIONS_NONE);
        HRESULT hr=factory->CreateHwndRenderTarget(properties,hwnd_properties,&target);
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
                auto shaped=layout(text.data()+cmd.text_offset,cmd.text_length,cmd.font_size,text.data()+cmd.font_offset,cmd.font_length);
                if(shaped) target->DrawTextLayout(D2D1::Point2F(cmd.bounds.x,cmd.bounds.y),shaped,brush);
            } else if(cmd.kind==3) target->DrawLine(D2D1::Point2F(cmd.bounds.x,cmd.bounds.y),D2D1::Point2F(cmd.bounds.x+cmd.bounds.w,cmd.bounds.y+cmd.bounds.h),brush,cmd.radius);
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
    case WM_NCCALCSIZE:
        if(window->custom_titlebar) return 0;
        break;
    case WM_NCHITTEST:
        if(window->custom_titlebar) {
            POINT point{GET_X_LPARAM(lparam),GET_Y_LPARAM(lparam)};
            RECT outer{}; GetWindowRect(handle,&outer);
            int border=static_cast<int>(8*dpi(handle)/96);
            if(!IsZoomed(handle)) {
                bool left=point.x<outer.left+border,right=point.x>=outer.right-border;
                bool top=point.y<outer.top+border,bottom=point.y>=outer.bottom-border;
                if(top && left) return HTTOPLEFT; if(top && right) return HTTOPRIGHT;
                if(bottom && left) return HTBOTTOMLEFT; if(bottom && right) return HTBOTTOMRIGHT;
                if(top) return HTTOP; if(bottom) return HTBOTTOM; if(left) return HTLEFT; if(right) return HTRIGHT;
            }
            ScreenToClient(handle,&point);
            float x=point.x*96.0f/dpi(handle),y=point.y*96.0f/dpi(handle);
            for(const auto &cmd:window->commands) if(cmd.kind==4 && x>=cmd.clip.x && x<cmd.clip.x+cmd.clip.w && y>=cmd.clip.y && y<cmd.clip.y+cmd.clip.h) return HTCAPTION;
            return HTCLIENT;
        }
        break;
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
        if(!(lparam & (1LL<<30))) gd_go_event(4,0,0,static_cast<int>(wparam),((GetKeyState(VK_SHIFT)&0x8000)?1:0)|((GetKeyState(VK_CONTROL)&0x8000)?2:0)|((GetKeyState(VK_MENU)&0x8000)?4:0));
        return 0;
    case WM_CHAR: {
        unsigned value=static_cast<unsigned>(wparam);
        if(value>=0xd800 && value<=0xdbff) { window->high_surrogate=value; return 0; }
        if(value>=0xdc00 && value<=0xdfff) { if(!window->high_surrogate) return 0; value=0x10000+((window->high_surrogate-0xd800)<<10)+(value-0xdc00); }
        window->high_surrogate=0;
        if(value>=32 && value!=127) gd_go_event(6,0,0,static_cast<int>(value),0);
        return 0;
    }
    case WM_MOUSEWHEEL:
        gd_go_event(7,0,GET_WHEEL_DELTA_WPARAM(wparam)/40.0f,0,0);
        return 0;
    case WM_GETMINMAXINFO:
        if(window->custom_titlebar) {
            MONITORINFO monitor{sizeof(MONITORINFO)};
            if(GetMonitorInfoW(MonitorFromWindow(handle,MONITOR_DEFAULTTONEAREST),&monitor)) {
                auto info=reinterpret_cast<MINMAXINFO *>(lparam);
                info->ptMaxPosition={monitor.rcWork.left-monitor.rcMonitor.left,monitor.rcWork.top-monitor.rcMonitor.top};
                info->ptMaxSize={monitor.rcWork.right-monitor.rcWork.left,monitor.rcWork.bottom-monitor.rcWork.top};
                info->ptMinTrackSize={static_cast<LONG>(640*dpi(handle)/96),static_cast<LONG>(420*dpi(handle)/96)};
                return 0;
            }
        }
        break;
    case wake_message: InvalidateRect(handle,nullptr,FALSE); return 0;
    case action_message:
        if(wparam==1) ShowWindow(handle,SW_MINIMIZE);
        if(wparam==2) ShowWindow(handle,IsZoomed(handle)?SW_RESTORE:SW_MAXIMIZE);
        return 0;
    case WM_CLOSE: DestroyWindow(handle); return 0;
    case WM_DESTROY:
        active_window.store(nullptr); PostQuitMessage(0); return 0;
    }
    return DefWindowProcW(handle,message,wparam,lparam);
}
}

extern "C" const char *gd_run(const char *title,float width,float height,GDColor background,int custom_titlebar) {
    static std::string last_error;
    last_error.clear();
    rendered_frames.store(0);
    HRESULT initialized=CoInitializeEx(nullptr,COINIT_APARTMENTTHREADED);
    if(FAILED(initialized)) return "CoInitializeEx failed: the UI thread must use a single-threaded COM apartment";
    {
        Window window;
        window.background=background;
        window.custom_titlebar=custom_titlebar!=0;
        wchar_t readback_option[2]{};
        window.readback=GetEnvironmentVariableW(L"GODESKTOP_READBACK",readback_option,2)>0 && readback_option[0]==L'1';
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
                // Keep the overlapped-window style for DWM composition and
                // system affordances; NCCALCSIZE removes the native caption.
                DWORD style=WS_OVERLAPPEDWINDOW;
                using AdjustForDpi=BOOL(WINAPI *)(LPRECT,DWORD,BOOL,DWORD,UINT);
                auto adjust=reinterpret_cast<AdjustForDpi>(GetProcAddress(GetModuleHandleW(L"user32.dll"),"AdjustWindowRectExForDpi"));
                if(!custom_titlebar) { if(adjust) adjust(&bounds,style,FALSE,0,initial_dpi); else AdjustWindowRectEx(&bounds,style,FALSE,0); }
                auto caption=wide(title,std::char_traits<char>::length(title));
                current=&window;
                int x=CW_USEDEFAULT,y=CW_USEDEFAULT;
                if(custom_titlebar) {
                    RECT work{}; SystemParametersInfoW(SPI_GETWORKAREA,0,&work,0);
                    x=work.left+std::max(0L,(work.right-work.left-(bounds.right-bounds.left))/2);
                    y=work.top+std::max(0L,(work.bottom-work.top-(bounds.bottom-bounds.top))/2);
                }
                HWND handle=CreateWindowExW(0,cls.lpszClassName,caption.c_str(),style,x,y,bounds.right-bounds.left,bounds.bottom-bounds.top,nullptr,nullptr,instance,&window);
                if(!handle) window.error="CreateWindowExW failed";
                else {
                    active_window.store(handle);
                    ShowWindow(handle,SW_SHOW); UpdateWindow(handle);
                    if(window.readback) SetWindowPos(handle,HWND_TOPMOST,0,0,0,0,SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE);
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
extern "C" void gd_measure(const char *text,size_t length,float size,const char *font,size_t font_length,float *width,float *height) {
    *width=0; *height=size*1.4f;
    if(!current) return;
    if(auto layout=current->layout(text,length,size,font,font_length)) {
        DWRITE_TEXT_METRICS metrics{};
        if(SUCCEEDED(layout->GetMetrics(&metrics))) { *width=metrics.widthIncludingTrailingWhitespace; *height=metrics.height; }
    }
}
extern "C" void gd_wake() { if(auto handle=active_window.load()) PostMessageW(handle,wake_message,0,0); }
extern "C" void gd_quit() { if(auto handle=active_window.load()) PostMessageW(handle,WM_CLOSE,0,0); }
extern "C" void gd_window_action(int action) { if(auto handle=active_window.load()) PostMessageW(handle,action_message,action,0); }
extern "C" uint64_t gd_rendered_frames() { return rendered_frames.load(); }
