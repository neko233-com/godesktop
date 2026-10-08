//go:build windows && cgo

#define NOMINMAX
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <windowsx.h>
#include <dwrite.h>
#include <atomic>
#include <map>
#include <string>
#include <vector>
#include <cstdio>
#include <tuple>
#include <algorithm>
#include <mutex>
#include "bridge.h"
#include "image_store.h"
#include "dx12_surface.h"

namespace {
constexpr UINT wake_message = WM_APP + 1;
constexpr UINT action_message = WM_APP + 2;
std::atomic<HWND> active_window{nullptr};
std::atomic<uint64_t> rendered_frames{0};
std::mutex stats_mutex;
GDRenderStats latest_stats{};

template<class T> void release(T *&value) { if (value) { value->Release(); value=nullptr; } }
std::wstring wide(const char *text, size_t length) {
    if (!length) return {};
    int count=MultiByteToWideChar(CP_UTF8,0,text,static_cast<int>(length),nullptr,0);
    std::wstring result(count,L'\0');
    MultiByteToWideChar(CP_UTF8,0,text,static_cast<int>(length),result.data(),count);
    return result;
}
thread_local float diagnostic_dpi=0;
float dpi(HWND window) {
    if(diagnostic_dpi) return diagnostic_dpi;
    using GetDpi = UINT(WINAPI *)(HWND);
    auto fn=reinterpret_cast<GetDpi>(GetProcAddress(GetModuleHandleW(L"user32.dll"),"GetDpiForWindow"));
    return fn ? static_cast<float>(fn(window)) : 96.0f;
}
thread_local int replay_modifiers=-1;
int input_modifiers() {
    if(replay_modifiers>=0) return replay_modifiers;
    return ((GetKeyState(VK_SHIFT)&0x8000)?1:0)|((GetKeyState(VK_CONTROL)&0x8000)?2:0)|((GetKeyState(VK_MENU)&0x8000)?4:0);
}
int metric_for_dpi(int index,HWND window) {
    using GetMetric = int(WINAPI *)(int,UINT);
    auto fn=reinterpret_cast<GetMetric>(GetProcAddress(GetModuleHandleW(L"user32.dll"),"GetSystemMetricsForDpi"));
    return fn?fn(index,static_cast<UINT>(dpi(window))):MulDiv(GetSystemMetrics(index),static_cast<int>(dpi(window)),96);
}
struct Window {
	bool test_input_isolation=false;
	unsigned replay_depth=0;
    bool focused=false;
    HWND handle=nullptr;
    IDWriteFactory *text_factory=nullptr;
    gd_dx12::Capture capture;
    std::unique_ptr<gd_dx12::Surface> surface=std::make_unique<gd_dx12::Surface>();
    gd_dx12::GlyphAtlas atlas;
    gd_dx12::Scene scene;
    std::map<uint64_t,std::shared_ptr<gd_dx12::AtlasPage>> bitmaps;
    GDColor background{};
    bool custom_titlebar=false;
	bool normal_capture_release=false;
    unsigned high_surrogate=0;
    bool readback=false;
    bool hardwareOnly=false,warpOnly=false,debug=false;
    uint64_t removeAfter=0;
    bool removalInjected=false;
    bool completionRace=false;
    bool shutdownTimeout=false,shutdownHeld=false;
    bool dirty=true,idle=false,closing=false;
    LONG layoutWidth=0,layoutHeight=0;
    bool layoutValid=false;
    std::vector<GDCommand> commands;
    std::string text;
    std::map<std::tuple<std::string,float,std::string>,IDWriteTextLayout *> layouts;
    std::string error;

    ~Window() {
        close_surface();
        // Command storage is retired while the window's atlas/bitmap caches
        // still retain their resources; member destruction otherwise reverses
        // that order. Cache cleanup below never submits more work.
        surface.reset();
        clear_text(); release(text_factory);
    }
    void clear_text() { for(auto &item:layouts) release(item.second); layouts.clear(); }
    void publish_stats() {
        surface->stats.glyph_rasterizations=atlas.rasterized;
        surface->stats.glyph_cache_hits=atlas.hits;
        surface->stats.glyph_cache_entries=atlas.entries();
        surface->stats.glyph_atlas_pages=atlas.accounting->pages;
        surface->stats.glyph_atlas_bytes=atlas.accounting->bytes;
        surface->stats.glyph_atlas_peak_bytes=atlas.accounting->peakBytes;
        surface->stats.glyph_atlas_epochs=atlas.epochs;
        surface->stats.bitmap_cache_entries=gd_image_entries();surface->stats.bitmap_cache_bytes=gd_image_bytes;
        std::lock_guard<std::mutex> lock(stats_mutex);
        latest_stats=surface->stats;
        rendered_frames.store(surface->stats.completed);
    }
    void request_frame() {
        if(closing || !surface) return;
        surface->stats.frame_requests++;
        if(dirty) surface->stats.coalesced_requests++;
        dirty=true; idle=false;
    }
    bool close_surface() {
        closing=true;
        if(!surface) return error.empty();
        bool closed=surface->close();
        if(!closed && error.empty()) error=surface->error.empty()?"GPU window shutdown failed":surface->error;
        // Final failure/cancellation and counters are visible before releasing
        // the HWND/current window. Surface retirement is idempotent.
        publish_stats();
        return closed && error.empty();
    }
    bool open_surface(UINT width,UINT height) {
        if(!surface->open(handle,width,height,readback?&capture:nullptr,hardwareOnly,warpOnly,debug)) return false;
        // The first Go View must observe the clock of this actual presentation
        // path, including a selected software adapter before any submission.
        publish_stats(); return true;
    }
    bool recover_surface() {
        if(!surface->deviceLost()) { error=surface->error; return false; }
        GDRenderStats saved=surface->stats;
        if(saved.device_recoveries>=3) { error="D3D12 device was lost repeatedly; three recovery attempts exhausted: "+surface->error; return false; }
        // UINT64_MAX is the device-removed fence sentinel, never a successful
        // submission. Account for abandoned work separately from GPU completion.
        saved.dropped_frames=saved.submitted-saved.completed;
        saved.in_flight=0; saved.device_recoveries++;
        surface.reset(); // Retires command storage before clearing its caches.
        scene=gd_dx12::Scene{}; atlas.clear(); atlas.resetColorDevice(); bitmaps.clear();
        surface=std::make_unique<gd_dx12::Surface>();
        surface->stats=saved;
        RECT client{}; GetClientRect(handle,&client);
        if(!open_surface(std::max(1L,client.right),std::max(1L,client.bottom))) { error="D3D12 device recovery failed: "+surface->error; return false; }
        dirty=true; idle=false; error.clear(); publish_stats();
        return true;
    }
    void fail(const char *operation,HRESULT result) {
        char message[192];
        std::snprintf(message,sizeof(message),"%s failed (HRESULT 0x%08lx)",operation,static_cast<unsigned long>(result));
        error=message;
        if(handle) PostMessageW(handle,action_message,4,0);
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
    bool build_scene(float scale) {
        for(auto i=bitmaps.begin();i!=bitmaps.end();) {if(!gd_image_get(i->first)) i=bitmaps.erase(i);else ++i;}
        for(unsigned attempt=0;attempt<2;attempt++) {
            scene.reset(atlas.white());
            for(const auto &cmd:commands) {
                if(cmd.kind==4 || cmd.clip.w<=0 || cmd.clip.h<=0) continue;
                if(cmd.kind==2) {
                    if(!cmd.text_length) continue;
                    auto shaped=layout(text.data()+cmd.text_offset,cmd.text_length,cmd.font_size,text.data()+cmd.font_offset,cmd.font_length);
                    if(!shaped) return false;
                    gd_dx12::TextRenderer renderer(text_factory,atlas,scene,cmd,scale);
                    HRESULT hr=shaped->Draw(nullptr,&renderer,cmd.bounds.x,cmd.bounds.y);
                    if(FAILED(hr)) {
                        if(atlas.full) break;
                        error=atlas.error.empty()?"DirectWrite atlas drawing failed":atlas.error;
                        return false;
                    }
                } else if(cmd.kind==5) {
                    const GDImage *image=gd_image_get(cmd.image_id);
                    if(!image) {error="Bitmap scene references a missing image";return false;}
                    auto &page=bitmaps[cmd.image_id];
                    if(!page) page=std::make_shared<gd_dx12::AtlasPage>(*image);
                    scene.append(gd_gpu_instance(&cmd),page);
                } else scene.append(gd_gpu_instance(&cmd));
            }
            if(!atlas.full) return true;
            atlas.clear();
        }
        error="One frame exceeds the 16 MiB / 16384 glyph atlas budget";
        return false;
    }
    void layout_for_client(LONG width,LONG height,float scale) {
        gd_go_event(1,width/scale,height/scale,0,0);
        layoutWidth=width; layoutHeight=height;
        layoutValid=true;
    }
    void ensure_input_layout() {
        RECT client{}; GetClientRect(handle,&client);
        if(client.right>0 && client.bottom>0 && (!layoutValid || client.right!=layoutWidth || client.bottom!=layoutHeight)) {
            // Pointer messages may arrive immediately after restoring/resizing,
            // before the next DXGI-paced draw. Hit targets must use the current
            // viewport even when a same-size old GPU snapshot is still readable.
            layout_for_client(client.right,client.bottom,dpi(handle)/96.0f);
            request_frame();
        }
    }
    bool draw() {
        RECT client{}; GetClientRect(handle,&client);
        if(client.right==0 || client.bottom==0) return true;
        uint64_t started=gd_dx12::monotonic_nanos();
        float scale=dpi(handle)/96.0f;
        if(!surface->stats.submitted) gd_dx12::trace_stage("window scene begin");
        layout_for_client(client.right,client.bottom,scale);
        if(!error.empty() || !build_scene(scale)) return false;
        if(!surface->stats.submitted) gd_dx12::trace_stage("window scene built");
        if(!surface->submit(scene,client.right/scale,client.bottom/scale,scale,background,started,gd_dx12::monotonic_nanos())) { error=surface->error; return false; }
        publish_stats(); return true;
    }
    bool event_loop() {
        for(;;) {
            MSG message{};
            // Reentrant UI work can keep posting wakes. A bounded message turn
            // still polls completions and lets visible GPU frames make progress.
            for(unsigned messages=0;messages<64 && PeekMessageW(&message,nullptr,0,0,PM_REMOVE);messages++) {
                if(message.message==WM_QUIT) {
                    return close_surface();
                }
                TranslateMessage(&message); DispatchMessageW(&message);
            }
            if(removeAfter && !removalInjected && surface->stats.submitted>=removeAfter) {
                removalInjected=true;
                if(!surface->removeDevice()) { error=surface->error; return false; }
                if(recover_surface()) continue;
                return false;
            }
            if(!surface->poll()) { if(recover_surface()) continue; return false; }
            publish_stats();
            RECT client{}; GetClientRect(handle,&client);
            bool visible=IsWindowVisible(handle) && !IsIconic(handle) && client.right>0 && client.bottom>0;
            if(dirty && visible) {
                if(!surface->resize(client.right,client.bottom)) { if(recover_surface()) continue; return false; }
                // The owned shutdown fixture first collects four real frames,
                // then gates the fifth actual submission. It waits on the
                // existing completion event rather than polling or draining.
                bool waitForShutdownBaseline=shutdownTimeout && !shutdownHeld && surface->stats.submitted>=4 && surface->stats.in_flight!=0;
                if(surface->ready() && !waitForShutdownBaseline) {
                    if(shutdownTimeout && !shutdownHeld && surface->stats.submitted>=4) {
                        if(!surface->diagnosticHoldQueue()) { error=surface->error; return false; }
                        shutdownHeld=true;
                    }
                    dirty=false;
                    if(!draw()) { if(surface->deviceLost() && recover_surface()) continue; return false; }
                    if(shutdownHeld && !SetPropW(handle,L"godesktop.shutdown-pending",reinterpret_cast<HANDLE>(uintptr_t(1)))) { error="Publish owned pending-shutdown submission failed"; return false; }
                    if(completionRace && !surface->diagnosticCompleteBeforeWatch()) { error=surface->error; return false; }
                    continue;
                }
            }
            if(!dirty && !idle) { idle=true; surface->stats.idle_pauses++; publish_stats(); }
            HANDLE handles[3]{}; unsigned count=0,clockIndex=UINT_MAX;
            if(auto removal=surface->removalEvent()) handles[count++]=removal;
            if(dirty && visible) if(auto clock=surface->frameClock()) { clockIndex=count; handles[count++]=clock; }
            if(auto completion=surface->pendingCompletion()) handles[count++]=completion;
            if(!surface->error.empty()) { if(recover_surface()) continue; return false; }
            DWORD result=MsgWaitForMultipleObjectsEx(count,handles,INFINITE,QS_ALLINPUT,MWMO_INPUTAVAILABLE);
            if(result==WAIT_FAILED) { error="GPU/message wait failed"; return false; }
            if(clockIndex!=UINT_MAX && result==WAIT_OBJECT_0+clockIndex) surface->clockSignalled();
        }
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
    if(window->closing && message!=WM_DESTROY) {
        // Closing can produce focus/capture/paint messages. They must not
        // reenter Go callbacks or schedule work against retired GPU storage.
        if(message==WM_PAINT) { PAINTSTRUCT paint{}; BeginPaint(handle,&paint); EndPaint(handle,&paint); return 0; }
        return DefWindowProcW(handle,message,wparam,lparam);
    }
    // Opt-in native acceptance transport. USER32 marshals WM_COPYDATA across
    // processes; replay uses the original Win32 handler/GPU path. Physical
    // desktop input cannot alter a fixture while the user works elsewhere.
    if(message==WM_COPYDATA && window->test_input_isolation) {
        struct Replay { uint32_t message,modifiers; uintptr_t wparam,lparam; };
        auto copy=reinterpret_cast<const COPYDATASTRUCT *>(lparam);
        if(!copy || copy->dwData!=0x47445052 || copy->cbData!=sizeof(Replay) || !copy->lpData) return 0;
        const Replay replay=*reinterpret_cast<const Replay *>(copy->lpData);
        switch(replay.message) {
        case WM_LBUTTONDOWN:case WM_LBUTTONUP:case WM_MOUSEMOVE:case WM_MOUSEWHEEL:case WM_MOUSEHWHEEL:
        case WM_KEYDOWN:case WM_KEYUP:case WM_SYSKEYDOWN:case WM_SYSKEYUP:case WM_CHAR:
        case WM_CANCELMODE:case WM_KILLFOCUS:case WM_CAPTURECHANGED:case WM_ACTIVATE:break;
        default:return 0;
        }
        int previous=replay_modifiers; replay_modifiers=static_cast<int>(replay.modifiers&15);
        window->replay_depth++;
        SendMessageW(handle,replay.message,replay.wparam,replay.lparam);
        window->replay_depth--; replay_modifiers=previous;
        return 1;
    }
    if(window->test_input_isolation && !window->replay_depth) {
        switch(message) {
        case WM_LBUTTONDOWN:case WM_LBUTTONUP:case WM_MOUSEMOVE:case WM_MOUSEWHEEL:case WM_MOUSEHWHEEL:
        case WM_KEYDOWN:case WM_KEYUP:case WM_SYSKEYDOWN:case WM_SYSKEYUP:case WM_CHAR:
        case WM_CANCELMODE:case WM_KILLFOCUS:case WM_CAPTURECHANGED:case WM_ACTIVATE:return 0;
        case WM_MOUSEACTIVATE:return MA_NOACTIVATEANDEAT;
        }
    }
    switch(message) {
    case WM_NCCALCSIZE:
        if(window->custom_titlebar) {
            // A maximized thick frame lies outside the visible work area.
            // Keep those invisible borders outside our client rectangle.
            if(IsZoomed(handle)) {
                auto rect=wparam?&reinterpret_cast<NCCALCSIZE_PARAMS *>(lparam)->rgrc[0]:reinterpret_cast<RECT *>(lparam);
                int padding=metric_for_dpi(SM_CXPADDEDBORDER,handle);
                int x=metric_for_dpi(SM_CXFRAME,handle)+padding;
                int y=metric_for_dpi(SM_CYFRAME,handle)+padding;
                rect->left+=x; rect->right-=x; rect->top+=y; rect->bottom-=y;
            }
            return wparam?WVR_REDRAW:0;
        }
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
        PAINTSTRUCT paint{}; HDC dc=BeginPaint(handle,&paint);
        if(window->surface && window->surface->softwarePresentation()) {
            // Expose only repaints the immutable completed software frontbuffer.
            // It must not feed another Go View/submission back into WM_PAINT.
            if(!window->surface->repaint(dc)) window->error=window->surface->error;
        } else window->request_frame();
        EndPaint(handle,&paint); return 0;
    }
    case WM_ERASEBKGND: return 1;
    case WM_SIZE:
        window->layoutValid=false;
        if(window->surface && window->surface->softwarePresentation()) window->request_frame();
        InvalidateRect(handle,nullptr,FALSE); return 0;
    case WM_DPICHANGED: {
        window->layoutValid=false;
        if(window->surface && window->surface->softwarePresentation()) window->request_frame();
        auto suggested=reinterpret_cast<RECT *>(lparam);
        SetWindowPos(handle,nullptr,suggested->left,suggested->top,suggested->right-suggested->left,suggested->bottom-suggested->top,SWP_NOZORDER|SWP_NOACTIVATE);
        InvalidateRect(handle,nullptr,FALSE); return 0;
    }
    case WM_LBUTTONDOWN:
        SetFocus(handle); SetCapture(handle);
        window->ensure_input_layout();
        gd_go_event(2,GET_X_LPARAM(lparam)*96.0f/dpi(handle),GET_Y_LPARAM(lparam)*96.0f/dpi(handle),0,input_modifiers()); return 0;
    case WM_LBUTTONUP:
        window->ensure_input_layout();
        gd_go_event(3,GET_X_LPARAM(lparam)*96.0f/dpi(handle),GET_Y_LPARAM(lparam)*96.0f/dpi(handle),0,input_modifiers());
        // A normal release is already acknowledged by PointerReleased. Its
        // synchronous WM_CAPTURECHANGED must not cancel a newly opened menu.
        if(GetCapture()==handle) { window->normal_capture_release=true; ReleaseCapture(); window->normal_capture_release=false; } return 0;
    case WM_MOUSEMOVE:
        window->ensure_input_layout(); gd_go_event(8,GET_X_LPARAM(lparam)*96.0f/dpi(handle),GET_Y_LPARAM(lparam)*96.0f/dpi(handle),0,input_modifiers());
        return 0;
    case WM_CAPTURECHANGED:
        if(window->normal_capture_release) return 0;
        gd_go_event(5,0,0,0,0); return 0;
    case WM_KILLFOCUS:
        gd_go_event(5,0,0,0,0); return 0;
    case WM_ACTIVATE: {
        // WM_KILLFOCUS can be caused by an owned child control, and losing
        // mouse capture is unrelated to top-level activation. Preserve those
        // existing cancellation events without treating them as window blur.
        const WORD activation=LOWORD(wparam);
        if(activation==WA_INACTIVE || activation==WA_ACTIVE || activation==WA_CLICKACTIVE) {
            const bool focused=activation!=WA_INACTIVE;
            if(window->focused!=focused) {
                window->focused=focused;
                gd_go_event(10,0,0,focused?1:0,0);
            }
        }
        // Preserve DefWindowProc's ordinary keyboard-focus behavior on active
        // non-minimized windows. The input callback does not replace USER32.
        break;
    }
    case WM_SYSKEYDOWN:
        if(wparam==VK_F4 && (lparam&(1LL<<29))) break; // preserve guarded Alt+F4
        gd_go_event(4,0,0,static_cast<int>(wparam),input_modifiers()|((lparam&(1LL<<30))?16:0)); return 0;
    case WM_KEYDOWN:
        gd_go_event(4,0,0,static_cast<int>(wparam),input_modifiers()|((lparam&(1LL<<30))?16:0));
        return 0;
    case WM_KEYUP:
    case WM_SYSKEYUP:
        gd_go_event(9,0,0,static_cast<int>(wparam),input_modifiers());
        return 0;
    case WM_SYSCHAR: return 0; // custom menubar owns Alt mnemonics, no system beep
    case WM_CHAR: {
        unsigned value=static_cast<unsigned>(wparam);
        if(value>=0xd800 && value<=0xdbff) { window->high_surrogate=value; return 0; }
        if(value>=0xdc00 && value<=0xdfff) { if(!window->high_surrogate) return 0; value=0x10000+((window->high_surrogate-0xd800)<<10)+(value-0xdc00); }
        window->high_surrogate=0;
        if(value>=32 && value!=127) gd_go_event(6,0,0,static_cast<int>(value),input_modifiers());
        return 0;
    }
    case WM_MOUSEWHEEL:
    case WM_MOUSEHWHEEL: {
        window->ensure_input_layout();
        POINT point{GET_X_LPARAM(lparam),GET_Y_LPARAM(lparam)};
        if(!ScreenToClient(handle,&point)) return 0;
        float delta=GET_WHEEL_DELTA_WPARAM(wparam)/40.0f,scale=96.0f/dpi(handle);
        int modifiers=input_modifiers()|((wparam&MK_SHIFT)?1:0)|((wparam&MK_CONTROL)?2:0);
        gd_go_scroll(message==WM_MOUSEHWHEEL?delta:0,message==WM_MOUSEWHEEL?delta:0,point.x*scale,point.y*scale,modifiers);
        return 0;
    }
    case WM_GETMINMAXINFO:
        if(diagnostic_dpi) {
            // Readback fixtures may exceed a small CI monitor's physical size.
            // Allow their owned client target without changing system settings.
            auto info=reinterpret_cast<MINMAXINFO *>(lparam);
            info->ptMaxTrackSize={16384,16384}; return 0;
        }
        if(window->custom_titlebar) {
            MONITORINFO monitor{sizeof(MONITORINFO)};
            if(GetMonitorInfoW(MonitorFromWindow(handle,MONITOR_DEFAULTTONEAREST),&monitor)) {
                auto info=reinterpret_cast<MINMAXINFO *>(lparam);
                int padding=metric_for_dpi(SM_CXPADDEDBORDER,handle);
                int x=metric_for_dpi(SM_CXFRAME,handle)+padding;
                int y=metric_for_dpi(SM_CYFRAME,handle)+padding;
                info->ptMaxPosition={monitor.rcWork.left-monitor.rcMonitor.left-x,monitor.rcWork.top-monitor.rcMonitor.top-y};
                info->ptMaxSize={monitor.rcWork.right-monitor.rcWork.left+2*x,monitor.rcWork.bottom-monitor.rcWork.top+2*y};
                info->ptMinTrackSize={static_cast<LONG>(640*dpi(handle)/96),static_cast<LONG>(420*dpi(handle)/96)};
                return 0;
            }
        }
        break;
    case wake_message:
        // Worker receipts must reach the UI even without a drawable (minimized
        // or hidden). Keep layout/GPU work behind the event loop's visible guard.
        gd_go_event(11,0,0,0,0);
        window->request_frame(); return 0;
    case action_message:
        if(wparam==1) ShowWindow(handle,SW_MINIMIZE);
        if(wparam==2) ShowWindow(handle,IsZoomed(handle)?SW_RESTORE:SW_MAXIMIZE);
        if(wparam==3) SendMessageW(handle,WM_CLOSE,0,0);
        if(wparam==4) { window->close_surface(); DestroyWindow(handle); }
        return 0;
    case WM_CLOSE: if(gd_go_should_close()) { window->close_surface(); DestroyWindow(handle); } return 0;
    case WM_DESTROY:
        active_window.store(nullptr); PostQuitMessage(0); return 0;
    }
    return DefWindowProcW(handle,message,wparam,lparam);
}
}

extern "C" const char *gd_run(const char *title,float width,float height,GDColor background,int custom_titlebar) {
    static std::string last_error;
    last_error.clear();
    struct DiagnosticDensity {
        ~DiagnosticDensity() { diagnostic_dpi=0; }
    } densityScope;
    diagnostic_dpi=0;
    wchar_t density[16]{},readback[2]{};
    DWORD densityLength=GetEnvironmentVariableW(L"GODESKTOP_TEST_DRAWABLE_SCALE",density,16);
    if(densityLength) {
        wchar_t *end=nullptr; double value=wcstod(density,&end);
        GetEnvironmentVariableW(L"GODESKTOP_READBACK",readback,2);
        if(densityLength>=16 || !end || *end || (value!=1 && value!=1.5 && value!=2) || readback[0]!=L'1')
            return "Diagnostic drawable scale requires GPU readback and a value of 1, 1.5 or 2";
        diagnostic_dpi=float(value*96);
    }
    rendered_frames.store(0);
    { std::lock_guard<std::mutex> lock(stats_mutex); latest_stats={}; latest_stats.backend=3; latest_stats.frame_slots=3; latest_stats.frame_clock=3; }
    HRESULT initialized=CoInitializeEx(nullptr,COINIT_APARTMENTTHREADED);
    if(FAILED(initialized)) return "CoInitializeEx failed: the UI thread must use a single-threaded COM apartment";
    {
        Window window;
        window.background=background;
        window.custom_titlebar=custom_titlebar!=0;
        wchar_t readback_option[2]{};
        window.readback=GetEnvironmentVariableW(L"GODESKTOP_READBACK",readback_option,2)>0 && readback_option[0]==L'1';
        wchar_t isolation_option[2]{};
        window.test_input_isolation=GetEnvironmentVariableW(L"GODESKTOP_TEST_INPUT_ISOLATION",isolation_option,2)>0 && isolation_option[0]==L'1';
        HRESULT hr=DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED,__uuidof(IDWriteFactory),reinterpret_cast<IUnknown **>(&window.text_factory));
        if(FAILED(hr)) { window.fail("Initialize DirectWrite",hr); }
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
                UINT initial_dpi=diagnostic_dpi?UINT(diagnostic_dpi):(get_dpi?get_dpi():96);
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
                    if(work.right>work.left && work.bottom>work.top) {
                        bounds.right=std::min(bounds.right,work.right-work.left);
                        bounds.bottom=std::min(bounds.bottom,work.bottom-work.top);
                    }
                    x=work.left+std::max(0L,(work.right-work.left-(bounds.right-bounds.left))/2);
                    y=work.top+std::max(0L,(work.bottom-work.top-(bounds.bottom-bounds.top))/2);
                }
                HWND handle=CreateWindowExW(0,cls.lpszClassName,caption.c_str(),style,x,y,bounds.right-bounds.left,bounds.bottom-bounds.top,nullptr,nullptr,instance,&window);
                if(!handle) window.error="CreateWindowExW failed";
                else {
                    active_window.store(handle);
                    if(diagnostic_dpi) {
                        RECT outer{},client{}; GetWindowRect(handle,&outer); GetClientRect(handle,&client);
                        // The OS caption still uses the monitor's DPI. Correct
                        // only the owned fixture's client size before submission.
                        SetWindowPos(handle,nullptr,0,0,LONG(width*diagnostic_dpi/96)+(outer.right-outer.left-client.right),
                            LONG(height*diagnostic_dpi/96)+(outer.bottom-outer.top-client.bottom),SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE);
                    }
                    RECT client{}; GetClientRect(handle,&client);
                    wchar_t adapter[32]{},debug[2]{};
                    GetEnvironmentVariableW(L"GODESKTOP_GPU_ADAPTER",adapter,32);
                    GetEnvironmentVariableW(L"GODESKTOP_GPU_DEBUG",debug,2);
                    window.hardwareOnly=wcscmp(adapter,L"hardware")==0; window.warpOnly=wcscmp(adapter,L"warp")==0; window.debug=debug[0]==L'1';
                    wchar_t removal[32]{}; GetEnvironmentVariableW(L"GODESKTOP_TEST_DEVICE_REMOVAL",removal,32);
                    window.removeAfter=_wcstoui64(removal,nullptr,10);
                    wchar_t completion[2]{}; GetEnvironmentVariableW(L"GODESKTOP_TEST_COMPLETION_RACE",completion,2);
                    window.completionRace=completion[0]==L'1';
                    wchar_t shutdown[2]{}; GetEnvironmentVariableW(L"GODESKTOP_TEST_SHUTDOWN_TIMEOUT",shutdown,2);
                    window.shutdownTimeout=shutdown[0]==L'1';
                    if(window.shutdownTimeout && !window.test_input_isolation) window.error="Owned shutdown timeout probe requires isolated native input";
                    else if(window.readback && !window.capture.open(handle)) window.error="GPU readback mapping creation failed";
                    else if(!window.open_surface(client.right,client.bottom)) window.error=window.surface->error;
                    else {
                        ShowWindow(handle,SW_SHOW); UpdateWindow(handle);
                        if(window.readback) SetWindowPos(handle,HWND_TOPMOST,0,0,0,0,SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE);
                        window.event_loop();
                    }
                    window.close_surface();
                    RemovePropW(handle,L"godesktop.shutdown-pending");
                    RemovePropW(handle,L"godesktop.backend");
                    if(IsWindow(handle)) DestroyWindow(handle);
                    // An initialization/rendering error can leave the WM_QUIT
                    // posted by our DestroyWindow after the loop has returned.
                    MSG quit{};
                    while(PeekMessageW(&quit,nullptr,WM_QUIT,WM_QUIT,PM_REMOVE)) {}
                }
                active_window.store(nullptr);
                current=nullptr;
            }
        }
        window.close_surface();
        last_error=window.error;
    }
    CoUninitialize();
    gd_images_clear();
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
extern "C" float gd_text_advance(const char *text,size_t length,float size,const char *font,size_t font_length) {
    float width,height; gd_measure(text,length,size,font,font_length,&width,&height);
    return width;
}
extern "C" void gd_wake() { if(auto handle=active_window.load()) PostMessageW(handle,wake_message,0,0); }
extern "C" void gd_quit() { if(auto handle=active_window.load()) PostMessageW(handle,action_message,4,0); }
extern "C" void gd_window_action(int action) { if(auto handle=active_window.load()) PostMessageW(handle,action_message,action,0); }
extern "C" GDRenderStats gd_render_stats(void) {
    std::lock_guard<std::mutex> lock(stats_mutex);
    return latest_stats;
}
extern "C" uint64_t gd_rendered_frames() { return rendered_frames.load(); }
