#ifndef GODESKTOP_DX12_SURFACE_H
#define GODESKTOP_DX12_SURFACE_H

#include <cwchar>
#include "dx12_glyphs.h"
#include "gpu_capture_windows.h"

namespace gd_dx12 {
inline uint64_t monotonic_nanos() {
    static double scale=[] { LARGE_INTEGER frequency; QueryPerformanceFrequency(&frequency); return 1e9/frequency.QuadPart; }();
    LARGE_INTEGER now; QueryPerformanceCounter(&now); return static_cast<uint64_t>(now.QuadPart*scale);
}
struct WindowFrame {
    ID3D12CommandAllocator *allocator=nullptr;
    ID3D12Resource *target=nullptr,*instances=nullptr,*readback=nullptr;
    ID3D12DescriptorHeap *descriptors=nullptr;
    std::vector<ID3D12Resource *> uploads;
    std::vector<std::shared_ptr<AtlasPage>> pages;
    void *mapped=nullptr;
    UINT64 capacity=0,fence=0,serial=0,pixelBytes=0;
    UINT width=0,height=0;
    bool collected=true;
    D3D12_PLACED_SUBRESOURCE_FOOTPRINT footprint{};
    D3D12_CPU_DESCRIPTOR_HANDLE rtv{};
    void retire() {
        for(auto &upload:uploads) drop(upload);
        uploads.clear(); pages.clear();
    }
    void release_target() { drop(target); drop(readback); fence=0; collected=true; retire(); }
    ~WindowFrame() {
        retire(); release_target();
        if(instances && mapped) instances->Unmap(0,nullptr);
        drop(instances); drop(descriptors); drop(allocator);
    }
    WindowFrame()=default;
    WindowFrame(const WindowFrame&)=delete;
    WindowFrame& operator=(const WindowFrame&)=delete;
};
class Surface {
    struct RemovalWatch {
        HANDLE event=nullptr;
        ~RemovalWatch() { if(event) CloseHandle(event); }
    } removal; // Destroyed after the device and its fence registrations.
    Device engine;
    std::array<WindowFrame,3> frames;
    IDXGISwapChain3 *swapchain=nullptr;
    ID3D12DescriptorHeap *rtvs=nullptr;
    ID3D12GraphicsCommandList *list=nullptr;
    ID3D12QueryHeap *queries=nullptr;
    HANDLE latency=nullptr;
    bool diagnostic=false,latencyReady=false;
    UINT pixelWidth=0,pixelHeight=0;
    UINT64 lastFence=0;
    Capture *capture=nullptr; // Window-owned mapping survives device rebuilding.
    bool ok(HRESULT hr,const char *operation) {
        if(SUCCEEDED(hr)) return true;
        char message[192]; std::snprintf(message,sizeof(message),"%s failed (HRESULT 0x%08lx)",operation,static_cast<unsigned long>(hr));
        error=message; return false;
    }
    bool resource(const D3D12_RESOURCE_DESC &desc,D3D12_HEAP_TYPE type,D3D12_RESOURCE_STATES state,ID3D12Resource **out) {
        D3D12_HEAP_PROPERTIES heap{}; heap.Type=type;
        return ok(engine.native()->CreateCommittedResource(&heap,D3D12_HEAP_FLAG_NONE,&desc,state,nullptr,IID_PPV_ARGS(out)),"Allocate window GPU resource");
    }
    bool targets(UINT width,UINT height) {
        if(diagnostic && uint64_t(width)*height*4>Capture::capacity) { error="GPU diagnostic capture exceeds 64 MiB"; return false; }
        auto rtv=rtvs->GetCPUDescriptorHandleForHeapStart();
        UINT increment=engine.native()->GetDescriptorHandleIncrementSize(D3D12_DESCRIPTOR_HEAP_TYPE_RTV);
        for(UINT index=0;index<frames.size();index++) {
            auto &frame=frames[index];
            if(!ok(swapchain->GetBuffer(index,IID_PPV_ARGS(&frame.target)),"Get swapchain buffer")) return false;
            frame.rtv=rtv; engine.native()->CreateRenderTargetView(frame.target,nullptr,rtv); rtv.ptr+=increment;
            frame.width=width; frame.height=height;
            frame.pixelBytes=0;
            if(diagnostic) {
                auto description=frame.target->GetDesc();
                engine.native()->GetCopyableFootprints(&description,0,1,0,&frame.footprint,nullptr,nullptr,&frame.pixelBytes);
                frame.pixelBytes=(frame.pixelBytes+255)&~UINT64(255);
            }
            if(!resource(buffer_desc(frame.pixelBytes+16),D3D12_HEAP_TYPE_READBACK,D3D12_RESOURCE_STATE_COPY_DEST,&frame.readback)) return false;
        }
        pixelWidth=width; pixelHeight=height;
        return true;
    }
    bool upload_page(WindowFrame &frame,const std::shared_ptr<AtlasPage> &page) {
        if(page->uploaded==page->version) return true;
        auto desc=texture_desc(page->width,page->height,page->channels==4?DXGI_FORMAT_R8G8B8A8_UNORM:DXGI_FORMAT_R8_UNORM,D3D12_RESOURCE_FLAG_NONE);
        if(!page->texture && !resource(desc,D3D12_HEAP_TYPE_DEFAULT,D3D12_RESOURCE_STATE_COPY_DEST,&page->texture)) return false;
        D3D12_PLACED_SUBRESOURCE_FOOTPRINT footprint{}; UINT64 bytes=0;
        engine.native()->GetCopyableFootprints(&desc,0,1,0,&footprint,nullptr,nullptr,&bytes);
        ID3D12Resource *upload=nullptr;
        if(!resource(buffer_desc(bytes),D3D12_HEAP_TYPE_UPLOAD,D3D12_RESOURCE_STATE_GENERIC_READ,&upload)) return false;
        frame.uploads.push_back(upload);
        void *mapped=nullptr; D3D12_RANGE read{0,0};
        if(!ok(upload->Map(0,&read,&mapped),"Map glyph staging")) return false;
        const GDImage *image=page->bitmapID?gd_image_get(page->bitmapID):nullptr;
        if(page->bitmapID && !image) {upload->Unmap(0,nullptr);error="Bitmap was evicted before texture upload";return false;}
        const unsigned char *pixels=image?image->pixels:page->pixels.data();
        UINT rowBytes=page->width*page->channels;
        for(UINT y=0;y<page->height;y++) std::memcpy(static_cast<unsigned char *>(mapped)+footprint.Offset+y*footprint.Footprint.RowPitch,pixels+y*rowBytes,rowBytes);
        upload->Unmap(0,nullptr);
        if(page->shaderState) { auto b=barrier(page->texture,D3D12_RESOURCE_STATE_PIXEL_SHADER_RESOURCE,D3D12_RESOURCE_STATE_COPY_DEST); list->ResourceBarrier(1,&b); }
        D3D12_TEXTURE_COPY_LOCATION dest{}; dest.pResource=page->texture; dest.Type=D3D12_TEXTURE_COPY_TYPE_SUBRESOURCE_INDEX;
        D3D12_TEXTURE_COPY_LOCATION source{}; source.pResource=upload; source.Type=D3D12_TEXTURE_COPY_TYPE_PLACED_FOOTPRINT; source.PlacedFootprint=footprint;
        list->CopyTextureRegion(&dest,0,0,0,&source,nullptr);
        auto b=barrier(page->texture,D3D12_RESOURCE_STATE_COPY_DEST,D3D12_RESOURCE_STATE_PIXEL_SHADER_RESOURCE); list->ResourceBarrier(1,&b);
        page->shaderState=true; page->uploaded=page->version;
        if(!page->bitmapID) stats.glyph_uploaded_bytes+=uint64_t(page->width)*page->height*page->channels;
        else {stats.bitmap_uploads++;stats.bitmap_uploaded_bytes+=uint64_t(page->width)*page->height*4;}
        return true;
    }
public:
    GDRenderStats stats{};
    std::string error;
    Surface() { stats.backend=3; stats.frame_slots=3; stats.frame_clock=3; }
    ~Surface() {
        if(swapchain) finish();
        drop(queries); drop(list); drop(rtvs); drop(swapchain);
        if(latency) CloseHandle(latency);
    }
    bool open(HWND window,UINT width,UINT height,Capture *readback,bool hardwareOnly,bool warpOnly,bool debug) {
        capture=readback;
        diagnostic=readback;
        if(!engine.open(hardwareOnly,warpOnly,debug,false,false)) { error=engine.error; return false; }
        removal.event=CreateEventW(nullptr,FALSE,FALSE,nullptr);
        if(!removal.event) { error="Create device removal event failed"; return false; }
        if(!ok(engine.completion()->SetEventOnCompletion(UINT64_MAX,removal.event),"Watch device removal")) return false;
        DXGI_SWAP_CHAIN_DESC1 description{};
        description.Width=width; description.Height=height; description.Format=DXGI_FORMAT_B8G8R8A8_UNORM;
        description.SampleDesc.Count=1; description.BufferUsage=DXGI_USAGE_RENDER_TARGET_OUTPUT;
        description.BufferCount=3; description.Scaling=DXGI_SCALING_STRETCH;
        description.SwapEffect=DXGI_SWAP_EFFECT_FLIP_DISCARD; description.AlphaMode=DXGI_ALPHA_MODE_IGNORE;
        description.Flags=DXGI_SWAP_CHAIN_FLAG_FRAME_LATENCY_WAITABLE_OBJECT;
        IDXGISwapChain1 *created=nullptr;
        HRESULT hr=engine.dxgi()->CreateSwapChainForHwnd(engine.commands(),window,&description,nullptr,nullptr,&created);
        if(SUCCEEDED(hr)) hr=created->QueryInterface(IID_PPV_ARGS(&swapchain));
        drop(created);
        if(!ok(hr,"Create D3D12 HWND flip swapchain")) return false;
        if(!ok(engine.dxgi()->MakeWindowAssociation(window,DXGI_MWA_NO_ALT_ENTER),"Set window association") || !ok(swapchain->SetMaximumFrameLatency(2),"Set swapchain frame latency")) return false;
        latency=swapchain->GetFrameLatencyWaitableObject();
        if(!latency) { error="DXGI frame latency object is unavailable"; return false; }
        D3D12_DESCRIPTOR_HEAP_DESC heap{}; heap.Type=D3D12_DESCRIPTOR_HEAP_TYPE_RTV; heap.NumDescriptors=3;
        if(!ok(engine.native()->CreateDescriptorHeap(&heap,IID_PPV_ARGS(&rtvs)),"Create window RTV descriptors")) return false;
        heap.Type=D3D12_DESCRIPTOR_HEAP_TYPE_CBV_SRV_UAV; heap.NumDescriptors=2*GlyphAtlas::maxPages+GDImageLimit; heap.Flags=D3D12_DESCRIPTOR_HEAP_FLAG_SHADER_VISIBLE;
        for(auto &frame:frames) {
            if(!ok(engine.native()->CreateCommandAllocator(D3D12_COMMAND_LIST_TYPE_DIRECT,IID_PPV_ARGS(&frame.allocator)),"Create window command allocator") || !ok(engine.native()->CreateDescriptorHeap(&heap,IID_PPV_ARGS(&frame.descriptors)),"Create frame glyph descriptors")) return false;
        }
        if(!ok(engine.native()->CreateCommandList(0,D3D12_COMMAND_LIST_TYPE_DIRECT,frames[0].allocator,engine.state(),IID_PPV_ARGS(&list)),"Create window command list") || !ok(list->Close(),"Close initial window list")) return false;
        D3D12_QUERY_HEAP_DESC query{}; query.Type=D3D12_QUERY_HEAP_TYPE_TIMESTAMP; query.Count=6;
        if(!ok(engine.native()->CreateQueryHeap(&query,IID_PPV_ARGS(&queries)),"Create window GPU timestamps")) return false;
        if(!targets(width,height)) return false;
        if(!SetPropW(window,L"godesktop.backend",reinterpret_cast<HANDLE>(uintptr_t(3)))) { error="Publish D3D12 HWND identity failed"; return false; }
        return true;
    }
    bool poll() {
        if(!swapchain) return true;
        UINT64 done=engine.completion()->GetCompletedValue();
        if(done==UINT64_MAX) { error="D3D12 device removed while rendering the window"; return false; }
        for(auto &frame:frames) if(!frame.collected && frame.fence<=done) {
            void *mapped=nullptr; D3D12_RANGE range{0,static_cast<SIZE_T>(frame.pixelBytes+16)};
            if(!ok(frame.readback->Map(0,&range,&mapped),"Map fenced window readback")) return false;
            UINT64 times[2]; std::memcpy(times,static_cast<unsigned char *>(mapped)+frame.pixelBytes,16);
            if(times[1]>=times[0] && engine.timestampFrequency()) stats.gpu_nanos=static_cast<UINT64>((times[1]-times[0])*(1e9/engine.timestampFrequency()));
            if(diagnostic) capture->publish(frame.width,frame.height,static_cast<unsigned char *>(mapped)+frame.footprint.Offset,frame.footprint.Footprint.RowPitch,frame.serial);
            D3D12_RANGE written{0,0}; frame.readback->Unmap(0,&written);
            frame.collected=true; frame.retire(); stats.completed++; stats.in_flight--;
        }
        return true;
    }
    bool resize(UINT width,UINT height) {
        if(width==pixelWidth && height==pixelHeight) return true;
        if(!engine.drain()) { error=engine.error; return false; }
        if(!poll()) return false;
        if(!ok(frames[0].allocator->Reset(),"Reset allocator for resize") || !ok(list->Reset(frames[0].allocator,engine.state()),"Reset list for resize") || !ok(list->Close(),"Close resize list")) return false;
        for(auto &frame:frames) frame.release_target();
        if(!ok(swapchain->ResizeBuffers(3,width,height,DXGI_FORMAT_B8G8R8A8_UNORM,DXGI_SWAP_CHAIN_FLAG_FRAME_LATENCY_WAITABLE_OBJECT),"Resize swapchain buffers")) return false;
        latencyReady=false;
        return targets(width,height);
    }
    bool ready() {
        if(!swapchain) return false;
        auto &frame=frames[swapchain->GetCurrentBackBufferIndex()];
        if(engine.completion()->GetCompletedValue()<frame.fence) return false;
        if(!latencyReady && WaitForSingleObject(latency,0)==WAIT_OBJECT_0) latencyReady=true;
        return latencyReady;
    }
    HANDLE frameClock() const { return latencyReady?nullptr:latency; }
    HANDLE removalEvent() const { return removal.event; }
    void clockSignalled() { latencyReady=true; }
    HANDLE pendingCompletion() {
        // Keep watching until poll collected the submission. The GPU can finish
        // between poll and this call; skipping an already-signalled fence would
        // leave CPU counters/resources pending while the UI sleeps indefinitely.
        if(stats.in_flight) {
            if(!engine.watch(lastFence)) { error=engine.error; return nullptr; }
            return engine.completionEvent();
        }
        return nullptr;
    }
    // Acceptance-only: finish the GPU after poll but before registering the
    // idle wait. Normal rendering never calls this synchronous diagnostic hook.
    bool diagnosticCompleteBeforeWatch() {
        if(!stats.in_flight) { error="Completion race probe has no submitted frame"; return false; }
        if(!engine.watch(lastFence)) { error=engine.error; return false; }
        if(WaitForSingleObject(engine.completionEvent(),5000)!=WAIT_OBJECT_0) { error="Completion race probe did not finish within five seconds"; return false; }
        // The production watcher must still return an event even though the GPU
        // is now complete and the CPU has not yet collected the submission.
        HANDLE event=pendingCompletion();
        if(!event || WaitForSingleObject(event,100)!=WAIT_OBJECT_0) { error="Completed submission lost its CPU collection notification"; return false; }
        return true;
    }
    bool submit(const Scene &scene,float dipWidth,float dipHeight,GDColor background,UINT64 started,UINT64 sceneEnd) {
        // ready() already proved the reused slot's fence complete. Collect it
        // again here so completion between the earlier poll and ready cannot
        // overwrite an uncollected slot's serial/readback or inflate in_flight.
        if(!poll()) return false;
        unsigned index=swapchain->GetCurrentBackBufferIndex();
        auto &frame=frames[index];
        if(!latencyReady || engine.completion()->GetCompletedValue()<frame.fence) { error="D3D12 frame submitted before DXGI/fence readiness"; return false; }
        if(scene.pages.size()>GlyphAtlas::maxPages+GDImageLimit || scene.instances.size()>16*1024*1024/sizeof(GDGPUInstance)) { error="Window scene exceeds GPU resource budget"; return false; }
        frame.retire();
        UINT64 needed=std::max<size_t>(1,scene.instances.size())*sizeof(GDGPUInstance);
        if(needed>frame.capacity) {
            if(frame.instances && frame.mapped) frame.instances->Unmap(0,nullptr);
            frame.mapped=nullptr; drop(frame.instances);
            frame.capacity=std::min<UINT64>(16*1024*1024,std::max<UINT64>(needed,frame.capacity*2+4096));
            if(!resource(buffer_desc(frame.capacity),D3D12_HEAP_TYPE_UPLOAD,D3D12_RESOURCE_STATE_GENERIC_READ,&frame.instances)) return false;
            D3D12_RANGE empty{0,0}; if(!ok(frame.instances->Map(0,&empty,&frame.mapped),"Map frame instances")) return false;
        }
        UINT64 acquired=monotonic_nanos();
        if(!ok(frame.allocator->Reset(),"Reset frame allocator") || !ok(list->Reset(frame.allocator,engine.state()),"Reset frame command list")) return false;
        if(!scene.instances.empty()) std::memcpy(frame.mapped,scene.instances.data(),scene.instances.size()*sizeof(GDGPUInstance));
        frame.pages=scene.pages;
        list->EndQuery(queries,D3D12_QUERY_TYPE_TIMESTAMP,index*2);
        auto descriptorBase=frame.descriptors->GetCPUDescriptorHandleForHeapStart();
        UINT descriptorSize=engine.native()->GetDescriptorHandleIncrementSize(D3D12_DESCRIPTOR_HEAP_TYPE_CBV_SRV_UAV);
        unsigned glyphSlot=0;
        if(!stats.submitted) trace_stage("window glyph upload begin");
        for(unsigned i=0;i<frame.pages.size();i++) {
            const auto &page=frame.pages[i];
            if(!upload_page(frame,page)) return false;
            D3D12_SHADER_RESOURCE_VIEW_DESC srv{}; srv.Format=page->channels==4?DXGI_FORMAT_R8G8B8A8_UNORM:DXGI_FORMAT_R8_UNORM; srv.ViewDimension=D3D12_SRV_DIMENSION_TEXTURE2D;
            srv.Shader4ComponentMapping=D3D12_DEFAULT_SHADER_4_COMPONENT_MAPPING; srv.Texture2D.MipLevels=1;
            auto descriptor=descriptorBase;
            descriptor.ptr+=(page->bitmapID?GlyphAtlas::maxPages+i:glyphSlot++)*descriptorSize;
            engine.native()->CreateShaderResourceView(page->texture,&srv,descriptor);
        }
        if(glyphSlot>GlyphAtlas::maxPages) { error="Scene exceeds the glyph descriptor table"; return false; }
        if(!stats.submitted) trace_stage("window glyph upload built");
        D3D12_SHADER_RESOURCE_VIEW_DESC white{}; white.Format=DXGI_FORMAT_R8_UNORM; white.ViewDimension=D3D12_SRV_DIMENSION_TEXTURE2D;
        white.Shader4ComponentMapping=D3D12_DEFAULT_SHADER_4_COMPONENT_MAPPING; white.Texture2D.MipLevels=1;
        // Every declared slot is valid, including the unused default bitmap slot.
        for(unsigned i=glyphSlot;i<=GlyphAtlas::maxPages;i++) {
            auto descriptor=descriptorBase; descriptor.ptr+=i*descriptorSize;
            engine.native()->CreateShaderResourceView(frame.pages[0]->texture,&white,descriptor);
        }
        auto render=barrier(frame.target,D3D12_RESOURCE_STATE_PRESENT,D3D12_RESOURCE_STATE_RENDER_TARGET); list->ResourceBarrier(1,&render);
        list->OMSetRenderTargets(1,&frame.rtv,FALSE,nullptr);
        float clear[4]={background.r,background.g,background.b,background.a}; list->ClearRenderTargetView(frame.rtv,clear,0,nullptr);
        list->SetGraphicsRootSignature(engine.signature());
        float viewport[4]={dipWidth,dipHeight,0,0}; list->SetGraphicsRoot32BitConstants(1,4,viewport,0);
        ID3D12DescriptorHeap *heaps[]={frame.descriptors}; list->SetDescriptorHeaps(1,heaps);
        list->SetGraphicsRootDescriptorTable(2,frame.descriptors->GetGPUDescriptorHandleForHeapStart());
        D3D12_VIEWPORT vp{0,0,float(pixelWidth),float(pixelHeight),0,1}; D3D12_RECT scissor{0,0,LONG(pixelWidth),LONG(pixelHeight)};
        list->RSSetViewports(1,&vp); list->RSSetScissorRects(1,&scissor); list->IASetPrimitiveTopology(D3D_PRIMITIVE_TOPOLOGY_TRIANGLELIST);
        for(const auto &batch:scene.batches) {
            list->SetGraphicsRootShaderResourceView(0,frame.instances->GetGPUVirtualAddress()+batch.start*sizeof(GDGPUInstance));
            auto texture=frame.descriptors->GetGPUDescriptorHandleForHeapStart();
            texture.ptr+=(GlyphAtlas::maxPages+batch.page)*descriptorSize;
            list->SetGraphicsRootDescriptorTable(3,texture); list->DrawInstanced(6,batch.count,0,0);
        }
        list->EndQuery(queries,D3D12_QUERY_TYPE_TIMESTAMP,index*2+1);
        if(diagnostic) {
            auto read=barrier(frame.target,D3D12_RESOURCE_STATE_RENDER_TARGET,D3D12_RESOURCE_STATE_COPY_SOURCE); list->ResourceBarrier(1,&read);
            D3D12_TEXTURE_COPY_LOCATION dest{}; dest.pResource=frame.readback; dest.Type=D3D12_TEXTURE_COPY_TYPE_PLACED_FOOTPRINT; dest.PlacedFootprint=frame.footprint;
            D3D12_TEXTURE_COPY_LOCATION source{}; source.pResource=frame.target; source.Type=D3D12_TEXTURE_COPY_TYPE_SUBRESOURCE_INDEX;
            list->CopyTextureRegion(&dest,0,0,0,&source,nullptr);
            auto present=barrier(frame.target,D3D12_RESOURCE_STATE_COPY_SOURCE,D3D12_RESOURCE_STATE_PRESENT); list->ResourceBarrier(1,&present);
        } else { auto present=barrier(frame.target,D3D12_RESOURCE_STATE_RENDER_TARGET,D3D12_RESOURCE_STATE_PRESENT); list->ResourceBarrier(1,&present); }
        list->ResolveQueryData(queries,D3D12_QUERY_TYPE_TIMESTAMP,index*2,2,frame.readback,frame.pixelBytes);
        if(!ok(list->Close(),"Close window submission")) return false;
        if(!stats.submitted) trace_stage("window ExecuteCommandLists begin");
        ID3D12CommandList *commands[]={list}; engine.commands()->ExecuteCommandLists(1,commands);
        if(!stats.submitted) trace_stage("window ExecuteCommandLists returned");
        HRESULT presented=swapchain->Present(1,0); latencyReady=false;
        if(!stats.submitted) trace_stage("window Present returned");
        if(!ok(presented,"Present D3D12 swapchain")) return false;
        if(!engine.signal(&frame.fence)) { error=engine.error; return false; }
        lastFence=frame.fence; frame.collected=false; frame.serial=++stats.submitted;
        stats.used_slots_mask|=1u<<index; stats.in_flight++; stats.max_in_flight=std::max(stats.max_in_flight,stats.in_flight);
        stats.instances=scene.instances.size(); stats.draw_calls=scene.batches.size(); stats.uploaded_bytes=scene.instances.size()*sizeof(GDGPUInstance);
        stats.frame_ticks++;
        UINT64 finished=monotonic_nanos(); stats.cpu_nanos=finished-started;
        stats.scene_nanos=sceneEnd-started; stats.acquire_nanos=acquired-sceneEnd; stats.encode_nanos=finished-acquired;
        return true;
    }
    bool finish() {
        if(!swapchain) return true;
        if(!engine.drain()) { error=engine.error; return false; }
        if(!poll()) return false;
        if(!engine.validate_messages()) { error=engine.error; return false; }
        return true;
    }
    bool deviceLost() const { return engine.native() && FAILED(engine.native()->GetDeviceRemovedReason()); }
    // Diagnostic fault injection removes only this renderer's D3D12 device.
    // It does not trigger a machine-wide TDR or touch other application windows.
    bool removeDevice() {
        ID3D12Device5 *device=nullptr;
        if(!ok(engine.native()->QueryInterface(IID_PPV_ARGS(&device)),"D3D12 device removal test interface")) return false;
        device->RemoveDevice(); drop(device); return true;
    }
};
}
#endif
