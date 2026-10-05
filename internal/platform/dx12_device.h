#ifndef GODESKTOP_DX12_DEVICE_H
#define GODESKTOP_DX12_DEVICE_H

// Original Direct3D 12 pipeline used by GPU acceptance tests. Window swapchain
// integration is a separate, unfinished step; this is never a Direct2D fallback.
#include <d3d12.h>
#include <d3d12sdklayers.h>
#include <dxgi1_6.h>
#include <algorithm>
#include <array>
#include <string>
#include <vector>
#include <cstring>
#include <cstdio>
#include "gpu_scene.h"
#include "gpu_shader_dx12.inc"

namespace gd_dx12 {
template<class T> void drop(T *&value) { if(value) { value->Release(); value=nullptr; } }
inline bool software_adapter(const DXGI_ADAPTER_DESC1 &description) {
    // DXGI's primary Basic Render adapter may have display outputs and omit
    // SOFTWARE. Microsoft documents its stable vendor/device identity:
    // https://learn.microsoft.com/windows/win32/direct3ddxgi/d3d10-graphics-programming-guide-dxgi
    return (description.Flags&DXGI_ADAPTER_FLAG_SOFTWARE)!=0 || (description.VendorId==0x1414 && description.DeviceId==0x8c);
}
inline D3D12_RESOURCE_DESC buffer_desc(UINT64 size) {
    D3D12_RESOURCE_DESC d{};
    d.Dimension=D3D12_RESOURCE_DIMENSION_BUFFER; d.Width=size;
    d.Height=1; d.DepthOrArraySize=1; d.MipLevels=1;
    d.SampleDesc.Count=1; d.Layout=D3D12_TEXTURE_LAYOUT_ROW_MAJOR;
    return d;
}
inline D3D12_RESOURCE_DESC texture_desc(UINT width,UINT height,DXGI_FORMAT format,D3D12_RESOURCE_FLAGS flags) {
    D3D12_RESOURCE_DESC d{};
    d.Dimension=D3D12_RESOURCE_DIMENSION_TEXTURE2D; d.Width=width;
    d.Height=height; d.DepthOrArraySize=1; d.MipLevels=1;
    d.Format=format; d.SampleDesc.Count=1; d.Flags=flags;
    return d;
}
inline D3D12_RESOURCE_BARRIER barrier(ID3D12Resource *resource,D3D12_RESOURCE_STATES before,D3D12_RESOURCE_STATES after) {
    D3D12_RESOURCE_BARRIER b{};
    b.Type=D3D12_RESOURCE_BARRIER_TYPE_TRANSITION;
    b.Transition={resource,D3D12_RESOURCE_BARRIER_ALL_SUBRESOURCES,before,after};
    return b;
}
struct Frame {
    ID3D12CommandAllocator *allocator=nullptr;
    ID3D12Resource *instances=nullptr,*target=nullptr,*readback=nullptr;
    UINT64 fence=0,cpuNanos=0;
    D3D12_PLACED_SUBRESOURCE_FOOTPRINT footprint{};
    UINT64 pixelBytes=0;
    D3D12_CPU_DESCRIPTOR_HANDLE rtv{};
    void *mapped=nullptr;
    ~Frame() {
        if(instances && mapped) instances->Unmap(0,nullptr);
        drop(readback); drop(target); drop(instances); drop(allocator);
    }
    Frame()=default;
    Frame(const Frame&)=delete;
    Frame& operator=(const Frame&)=delete;
};

class Device {
    IDXGIFactory4 *factory=nullptr;
    IDXGIAdapter1 *adapter=nullptr;
    ID3D12Device *device=nullptr;
    ID3D12CommandQueue *queue=nullptr;
    ID3D12GraphicsCommandList *list=nullptr;
    ID3D12RootSignature *root=nullptr;
    ID3D12PipelineState *pipeline=nullptr;
    ID3D12DescriptorHeap *rtvs=nullptr,*textures=nullptr;
    ID3D12Resource *mask=nullptr,*maskUpload=nullptr;
    ID3D12Fence *fence=nullptr;
    ID3D12Fence *gate=nullptr;
    ID3D12QueryHeap *queries=nullptr;
    ID3D12InfoQueue *diagnostics=nullptr;
    HANDLE event=nullptr;
    UINT64 sequence=0,frequency=0;
    bool closed=false;
    bool ok(HRESULT value,const char *operation) {
        if(SUCCEEDED(value)) return true;
        char detail[192];
        std::snprintf(detail,sizeof(detail),"%s failed (HRESULT 0x%08lx)",operation,static_cast<unsigned long>(value));
        error=detail; return false;
    }
    bool resource(const D3D12_RESOURCE_DESC &desc,D3D12_HEAP_TYPE type,D3D12_RESOURCE_STATES state,ID3D12Resource **out,const D3D12_CLEAR_VALUE *clear=nullptr) {
        D3D12_HEAP_PROPERTIES heap{}; heap.Type=type;
        return ok(device->CreateCommittedResource(&heap,D3D12_HEAP_FLAG_NONE,&desc,state,clear,IID_PPV_ARGS(out)),"CreateCommittedResource");
    }
    bool select(IDXGIAdapter1 *candidate) {
        ID3D12Device *selected=nullptr;
        if(FAILED(D3D12CreateDevice(candidate,D3D_FEATURE_LEVEL_11_0,IID_PPV_ARGS(&selected)))) return false;
        D3D12_FEATURE_DATA_SHADER_MODEL model{D3D_SHADER_MODEL_6_0};
        if(FAILED(selected->CheckFeatureSupport(D3D12_FEATURE_SHADER_MODEL,&model,sizeof(model))) || model.HighestShaderModel<D3D_SHADER_MODEL_6_0) { drop(selected); return false; }
        device=selected; adapter=candidate; adapter->AddRef();
        DXGI_ADAPTER_DESC1 description{}; adapter->GetDesc1(&description);
        software=software_adapter(description);
        vendorID=description.VendorId; deviceID=description.DeviceId; adapterFlags=description.Flags;
        int count=WideCharToMultiByte(CP_UTF8,0,description.Description,-1,nullptr,0,nullptr,nullptr);
        std::vector<char> name(count);
        WideCharToMultiByte(CP_UTF8,0,description.Description,-1,name.data(),count,nullptr,nullptr);
        adapterName=name.data();
        return true;
    }
    bool make_pipeline() {
        D3D12_DESCRIPTOR_RANGE range{D3D12_DESCRIPTOR_RANGE_TYPE_SRV,1,1,0,D3D12_DESCRIPTOR_RANGE_OFFSET_APPEND};
        D3D12_ROOT_PARAMETER parameters[3]{};
        parameters[0].ParameterType=D3D12_ROOT_PARAMETER_TYPE_SRV;
        parameters[0].Descriptor={0,0}; parameters[0].ShaderVisibility=D3D12_SHADER_VISIBILITY_VERTEX;
        parameters[1].ParameterType=D3D12_ROOT_PARAMETER_TYPE_32BIT_CONSTANTS;
        parameters[1].Constants={0,0,4}; parameters[1].ShaderVisibility=D3D12_SHADER_VISIBILITY_VERTEX;
        parameters[2].ParameterType=D3D12_ROOT_PARAMETER_TYPE_DESCRIPTOR_TABLE;
        parameters[2].DescriptorTable={1,&range}; parameters[2].ShaderVisibility=D3D12_SHADER_VISIBILITY_PIXEL;
        D3D12_STATIC_SAMPLER_DESC sampler{};
        sampler.Filter=D3D12_FILTER_MIN_MAG_MIP_LINEAR;
        sampler.AddressU=sampler.AddressV=sampler.AddressW=D3D12_TEXTURE_ADDRESS_MODE_CLAMP;
        sampler.ComparisonFunc=D3D12_COMPARISON_FUNC_ALWAYS;
        sampler.MaxLOD=D3D12_FLOAT32_MAX; sampler.ShaderVisibility=D3D12_SHADER_VISIBILITY_PIXEL;
        D3D12_ROOT_SIGNATURE_DESC signature{3,parameters,1,&sampler,D3D12_ROOT_SIGNATURE_FLAG_NONE};
        ID3DBlob *serialized=nullptr,*messages=nullptr;
        HRESULT hr=D3D12SerializeRootSignature(&signature,D3D_ROOT_SIGNATURE_VERSION_1,&serialized,&messages);
        if(FAILED(hr)) {
            ok(hr,"D3D12SerializeRootSignature");
            if(messages) error.append(": ").append(static_cast<const char *>(messages->GetBufferPointer()),messages->GetBufferSize());
            drop(messages); drop(serialized); return false;
        }
        hr=device->CreateRootSignature(0,serialized->GetBufferPointer(),serialized->GetBufferSize(),IID_PPV_ARGS(&root));
        drop(messages); drop(serialized);
        if(!ok(hr,"CreateRootSignature")) return false;
        D3D12_GRAPHICS_PIPELINE_STATE_DESC p{};
        p.pRootSignature=root;
        p.VS={gd_dx12_vertex,sizeof(gd_dx12_vertex)}; p.PS={gd_dx12_fragment,sizeof(gd_dx12_fragment)};
        p.BlendState.RenderTarget[0].BlendEnable=TRUE;
        p.BlendState.RenderTarget[0].SrcBlend=p.BlendState.RenderTarget[0].SrcBlendAlpha=D3D12_BLEND_ONE;
        p.BlendState.RenderTarget[0].DestBlend=p.BlendState.RenderTarget[0].DestBlendAlpha=D3D12_BLEND_INV_SRC_ALPHA;
        p.BlendState.RenderTarget[0].BlendOp=p.BlendState.RenderTarget[0].BlendOpAlpha=D3D12_BLEND_OP_ADD;
        p.BlendState.RenderTarget[0].LogicOp=D3D12_LOGIC_OP_NOOP;
        p.BlendState.RenderTarget[0].RenderTargetWriteMask=D3D12_COLOR_WRITE_ENABLE_ALL;
        p.SampleMask=UINT_MAX;
        p.RasterizerState.FillMode=D3D12_FILL_MODE_SOLID; p.RasterizerState.CullMode=D3D12_CULL_MODE_NONE;
        p.RasterizerState.DepthClipEnable=TRUE;
        p.DepthStencilState.DepthFunc=D3D12_COMPARISON_FUNC_ALWAYS;
        p.DepthStencilState.StencilReadMask=p.DepthStencilState.StencilWriteMask=0xff;
        p.DepthStencilState.FrontFace=p.DepthStencilState.BackFace={D3D12_STENCIL_OP_KEEP,D3D12_STENCIL_OP_KEEP,D3D12_STENCIL_OP_KEEP,D3D12_COMPARISON_FUNC_ALWAYS};
        p.PrimitiveTopologyType=D3D12_PRIMITIVE_TOPOLOGY_TYPE_TRIANGLE;
        p.NumRenderTargets=1; p.RTVFormats[0]=DXGI_FORMAT_B8G8R8A8_UNORM; p.SampleDesc.Count=1;
        return ok(device->CreateGraphicsPipelineState(&p,IID_PPV_ARGS(&pipeline)),"CreateGraphicsPipelineState/DXIL");
    }
public:
    static constexpr UINT width=192,height=128,slots=3,capacity=8192;
    std::array<Frame,slots> frames;
    std::string error,adapterName;
    bool software=false,debugLayer=false,gpuValidation=false;
    UINT vendorID=0,deviceID=0,adapterFlags=0;
    UINT usedSlots=0,maxInFlight=0;
    UINT64 submitted=0,completed=0,readbackWaits=0,ownershipDeferrals=0;
    Device()=default;
    Device(const Device&)=delete;
    Device& operator=(const Device&)=delete;
    ~Device() {
        if(gate) gate->Signal(1);
        if(queue && fence && event && !closed) drain();
        // A completed fence is required before allocators, upload memory and
        // descriptors can be destroyed. Normal draw/submit never waits.
        drop(diagnostics); drop(queries); drop(maskUpload); drop(mask);
        drop(textures); drop(rtvs); drop(pipeline); drop(root); drop(list);
        drop(gate); drop(fence); drop(queue); drop(adapter); drop(factory); drop(device);
        if(event) CloseHandle(event);
    }
    bool wait(UINT64 value,bool readback) {
        UINT64 done=fence->GetCompletedValue();
        if(done==UINT64_MAX) return ok(device->GetDeviceRemovedReason(),"GPU device removed");
        if(done>=value) return true;
        if(readback) readbackWaits++;
        if(!ok(fence->SetEventOnCompletion(value,event),"SetEventOnCompletion")) return false;
        if(WaitForSingleObject(event,5000)!=WAIT_OBJECT_0) { error="GPU fence did not complete within five seconds"; return false; }
        done=fence->GetCompletedValue();
        if(done==UINT64_MAX) return ok(device->GetDeviceRemovedReason(),"GPU device removed");
        if(done<value) { error="GPU fence event signalled before completion"; return false; }
        return true;
    }
    bool open(bool hardwareOnly,bool warpOnly,bool debug,bool requireDebug) {
        if(debug || requireDebug) {
            ID3D12Debug *layer=nullptr;
            HRESULT hr=D3D12GetDebugInterface(IID_PPV_ARGS(&layer));
            if(SUCCEEDED(hr)) {
                layer->EnableDebugLayer(); debugLayer=true;
                ID3D12Debug1 *validation=nullptr;
                if(SUCCEEDED(layer->QueryInterface(IID_PPV_ARGS(&validation)))) { validation->SetEnableGPUBasedValidation(TRUE); gpuValidation=true; drop(validation); }
                drop(layer);
            } else if(requireDebug) return ok(hr,"Required D3D12 debug layer");
        }
        if(!ok(CreateDXGIFactory2(0,IID_PPV_ARGS(&factory)),"CreateDXGIFactory2")) return false;
        if(!warpOnly) {
            IDXGIFactory6 *preferred=nullptr;
            factory->QueryInterface(IID_PPV_ARGS(&preferred));
            for(UINT index=0;;index++) {
                IDXGIAdapter1 *candidate=nullptr;
                HRESULT hr=preferred?preferred->EnumAdapterByGpuPreference(index,DXGI_GPU_PREFERENCE_HIGH_PERFORMANCE,IID_PPV_ARGS(&candidate)):factory->EnumAdapters1(index,&candidate);
                if(hr==DXGI_ERROR_NOT_FOUND) break;
                if(FAILED(hr)) { drop(preferred); return ok(hr,"Enumerate DXGI adapters"); }
                DXGI_ADAPTER_DESC1 description{}; candidate->GetDesc1(&description);
                bool selected=!software_adapter(description) && select(candidate);
                drop(candidate);
                if(selected) break;
            }
            drop(preferred);
        }
        if(!device && !hardwareOnly) {
            IDXGIAdapter1 *warp=nullptr;
            if(!ok(factory->EnumWarpAdapter(IID_PPV_ARGS(&warp)),"EnumWarpAdapter")) return false;
            bool selected=select(warp); drop(warp);
            if(!selected) { error="WARP does not support D3D12 / Shader Model 6.0"; return false; }
        }
        if(!device) { error="No hardware adapter supports D3D12 / Shader Model 6.0"; return false; }
        if(debugLayer) device->QueryInterface(IID_PPV_ARGS(&diagnostics));
        D3D12_COMMAND_QUEUE_DESC q{}; q.Type=D3D12_COMMAND_LIST_TYPE_DIRECT;
        if(!ok(device->CreateCommandQueue(&q,IID_PPV_ARGS(&queue)),"CreateCommandQueue") || !ok(queue->GetTimestampFrequency(&frequency),"GetTimestampFrequency")) return false;
        if(!ok(device->CreateFence(0,D3D12_FENCE_FLAG_NONE,IID_PPV_ARGS(&fence)),"CreateFence")) return false;
        event=CreateEventW(nullptr,FALSE,FALSE,nullptr);
        if(!event) { error="CreateEventW failed"; return false; }
        if(!make_pipeline()) return false;
        D3D12_DESCRIPTOR_HEAP_DESC heap{}; heap.Type=D3D12_DESCRIPTOR_HEAP_TYPE_RTV; heap.NumDescriptors=slots;
        if(!ok(device->CreateDescriptorHeap(&heap,IID_PPV_ARGS(&rtvs)),"Create RTV heap")) return false;
        heap.Type=D3D12_DESCRIPTOR_HEAP_TYPE_CBV_SRV_UAV; heap.NumDescriptors=1; heap.Flags=D3D12_DESCRIPTOR_HEAP_FLAG_SHADER_VISIBLE;
        if(!ok(device->CreateDescriptorHeap(&heap,IID_PPV_ARGS(&textures)),"Create texture heap")) return false;
        D3D12_QUERY_HEAP_DESC query{}; query.Type=D3D12_QUERY_HEAP_TYPE_TIMESTAMP; query.Count=slots*2;
        if(!ok(device->CreateQueryHeap(&query,IID_PPV_ARGS(&queries)),"Create timestamp heap")) return false;
        D3D12_CLEAR_VALUE clear{}; clear.Format=DXGI_FORMAT_B8G8R8A8_UNORM;
        clear.Color[0]=0.05f; clear.Color[1]=0.1f; clear.Color[2]=0.2f; clear.Color[3]=1;
        const auto target=texture_desc(width,height,clear.Format,D3D12_RESOURCE_FLAG_ALLOW_RENDER_TARGET);
        auto rtv=rtvs->GetCPUDescriptorHandleForHeapStart();
        UINT increment=device->GetDescriptorHandleIncrementSize(D3D12_DESCRIPTOR_HEAP_TYPE_RTV);
        for(auto &frame:frames) {
            if(!ok(device->CreateCommandAllocator(D3D12_COMMAND_LIST_TYPE_DIRECT,IID_PPV_ARGS(&frame.allocator)),"CreateCommandAllocator")) return false;
            if(!resource(buffer_desc(capacity*sizeof(GDGPUInstance)),D3D12_HEAP_TYPE_UPLOAD,D3D12_RESOURCE_STATE_GENERIC_READ,&frame.instances)) return false;
            D3D12_RANGE read{0,0};
            if(!ok(frame.instances->Map(0,&read,&frame.mapped),"Map upload buffer")) return false;
            if(!resource(target,D3D12_HEAP_TYPE_DEFAULT,D3D12_RESOURCE_STATE_COPY_SOURCE,&frame.target,&clear)) return false;
            device->GetCopyableFootprints(&target,0,1,0,&frame.footprint,nullptr,nullptr,&frame.pixelBytes);
            frame.pixelBytes=(frame.pixelBytes+255)&~UINT64(255);
            if(!resource(buffer_desc(frame.pixelBytes+16),D3D12_HEAP_TYPE_READBACK,D3D12_RESOURCE_STATE_COPY_DEST,&frame.readback)) return false;
            frame.rtv=rtv; device->CreateRenderTargetView(frame.target,nullptr,rtv); rtv.ptr+=increment;
        }
        if(!ok(device->CreateCommandList(0,D3D12_COMMAND_LIST_TYPE_DIRECT,frames[0].allocator,nullptr,IID_PPV_ARGS(&list)),"CreateCommandList")) return false;
        const auto maskDesc=texture_desc(8,8,DXGI_FORMAT_R8_UNORM,D3D12_RESOURCE_FLAG_NONE);
        if(!resource(maskDesc,D3D12_HEAP_TYPE_DEFAULT,D3D12_RESOURCE_STATE_COPY_DEST,&mask)) return false;
        D3D12_PLACED_SUBRESOURCE_FOOTPRINT footprint{}; UINT64 bytes;
        device->GetCopyableFootprints(&maskDesc,0,1,0,&footprint,nullptr,nullptr,&bytes);
        if(!resource(buffer_desc(bytes),D3D12_HEAP_TYPE_UPLOAD,D3D12_RESOURCE_STATE_GENERIC_READ,&maskUpload)) return false;
        void *mapped=nullptr; D3D12_RANGE empty{0,0};
        if(!ok(maskUpload->Map(0,&empty,&mapped),"Map mask upload")) return false;
        for(UINT y=0;y<8;y++) for(UINT x=0;x<8;x++) static_cast<unsigned char *>(mapped)[footprint.Offset+y*footprint.Footprint.RowPitch+x]=x<4?128:255;
        maskUpload->Unmap(0,nullptr);
        D3D12_TEXTURE_COPY_LOCATION dest{}; dest.pResource=mask; dest.Type=D3D12_TEXTURE_COPY_TYPE_SUBRESOURCE_INDEX;
        D3D12_TEXTURE_COPY_LOCATION source{}; source.pResource=maskUpload; source.Type=D3D12_TEXTURE_COPY_TYPE_PLACED_FOOTPRINT; source.PlacedFootprint=footprint;
        list->CopyTextureRegion(&dest,0,0,0,&source,nullptr);
        auto ready=barrier(mask,D3D12_RESOURCE_STATE_COPY_DEST,D3D12_RESOURCE_STATE_PIXEL_SHADER_RESOURCE); list->ResourceBarrier(1,&ready);
        if(!ok(list->Close(),"Close mask upload")) return false;
        ID3D12CommandList *commands[]={list}; queue->ExecuteCommandLists(1,commands);
        if(!ok(queue->Signal(fence,++sequence),"Signal mask upload") || !wait(sequence,false)) return false;
        D3D12_SHADER_RESOURCE_VIEW_DESC srv{}; srv.Format=DXGI_FORMAT_R8_UNORM; srv.ViewDimension=D3D12_SRV_DIMENSION_TEXTURE2D;
        srv.Shader4ComponentMapping=D3D12_DEFAULT_SHADER_4_COMPONENT_MAPPING; srv.Texture2D.MipLevels=1;
        device->CreateShaderResourceView(mask,&srv,textures->GetCPUDescriptorHandleForHeapStart());
        return true;
    }
    // Returns false with an empty error when the caller must defer submission.
    // Reusing an allocator or writing its upload bytes before its fence completes
    // is forbidden, independently of command-list object reuse.
    bool submit(UINT slot,const GDGPUInstance *instances,UINT count,const float clear[4],UINT64 cpuStarted,UINT64 (*clock)()) {
        if(slot>=slots || !count || count>capacity) { error="Invalid D3D12 frame capacity"; return false; }
        auto &frame=frames[slot];
        UINT64 done=fence->GetCompletedValue();
        if(done==UINT64_MAX) return ok(device->GetDeviceRemovedReason(),"GPU device removed");
        if(done<frame.fence) { ownershipDeferrals++; return false; }
        if(!ok(frame.allocator->Reset(),"Reset allocator") || !ok(list->Reset(frame.allocator,pipeline),"Reset command list")) return false;
        std::memcpy(frame.mapped,instances,count*sizeof(GDGPUInstance));
        auto render=barrier(frame.target,D3D12_RESOURCE_STATE_COPY_SOURCE,D3D12_RESOURCE_STATE_RENDER_TARGET); list->ResourceBarrier(1,&render);
        list->EndQuery(queries,D3D12_QUERY_TYPE_TIMESTAMP,slot*2);
        list->OMSetRenderTargets(1,&frame.rtv,FALSE,nullptr);
        list->ClearRenderTargetView(frame.rtv,clear,0,nullptr);
        list->SetGraphicsRootSignature(root);
        list->SetGraphicsRootShaderResourceView(0,frame.instances->GetGPUVirtualAddress());
        float viewport[4]={static_cast<float>(width),static_cast<float>(height),0,0};
        list->SetGraphicsRoot32BitConstants(1,4,viewport,0);
        ID3D12DescriptorHeap *heaps[]={textures}; list->SetDescriptorHeaps(1,heaps);
        list->SetGraphicsRootDescriptorTable(2,textures->GetGPUDescriptorHandleForHeapStart());
        D3D12_VIEWPORT vp{0,0,static_cast<float>(width),static_cast<float>(height),0,1}; D3D12_RECT scissor{0,0,width,height};
        list->RSSetViewports(1,&vp); list->RSSetScissorRects(1,&scissor);
        list->IASetPrimitiveTopology(D3D_PRIMITIVE_TOPOLOGY_TRIANGLELIST);
        list->DrawInstanced(6,count,0,0);
        list->EndQuery(queries,D3D12_QUERY_TYPE_TIMESTAMP,slot*2+1);
        auto read=barrier(frame.target,D3D12_RESOURCE_STATE_RENDER_TARGET,D3D12_RESOURCE_STATE_COPY_SOURCE); list->ResourceBarrier(1,&read);
        D3D12_TEXTURE_COPY_LOCATION destination{}; destination.pResource=frame.readback; destination.Type=D3D12_TEXTURE_COPY_TYPE_PLACED_FOOTPRINT; destination.PlacedFootprint=frame.footprint;
        D3D12_TEXTURE_COPY_LOCATION source{}; source.pResource=frame.target; source.Type=D3D12_TEXTURE_COPY_TYPE_SUBRESOURCE_INDEX;
        list->CopyTextureRegion(&destination,0,0,0,&source,nullptr);
        list->ResolveQueryData(queries,D3D12_QUERY_TYPE_TIMESTAMP,slot*2,2,frame.readback,frame.pixelBytes);
        if(!ok(list->Close(),"Close frame")) return false;
        ID3D12CommandList *commands[]={list}; queue->ExecuteCommandLists(1,commands);
        frame.fence=++sequence;
        if(!ok(queue->Signal(fence,frame.fence),"Signal frame")) return false;
        frame.cpuNanos=clock()-cpuStarted;
        submitted++; usedSlots|=1u<<slot;
        UINT flight=0; done=fence->GetCompletedValue();
        for(const auto &candidate:frames) if(candidate.fence>done) flight++;
        maxInFlight=std::max(maxInFlight,flight);
        return true;
    }
    bool collect(UINT slot,unsigned char *bgra,UINT64 *gpuNanos) {
        if(slot>=slots || !frames[slot].fence) { error="Invalid D3D12 readback slot"; return false; }
        auto &frame=frames[slot];
        if(!wait(frame.fence,true)) return false;
        D3D12_RANGE range{0,static_cast<SIZE_T>(frame.pixelBytes+16)}; void *mapped=nullptr;
        if(!ok(frame.readback->Map(0,&range,&mapped),"Map completed GPU readback")) return false;
        for(UINT y=0;y<height;y++) std::memcpy(bgra+y*width*4,static_cast<unsigned char *>(mapped)+frame.footprint.Offset+y*frame.footprint.Footprint.RowPitch,width*4);
        UINT64 timestamps[2]; std::memcpy(timestamps,static_cast<unsigned char *>(mapped)+frame.pixelBytes,sizeof(timestamps));
        D3D12_RANGE written{0,0}; frame.readback->Unmap(0,&written);
        if(timestamps[1]<timestamps[0] || !frequency) { error="Invalid GPU timestamp order"; return false; }
        *gpuNanos=static_cast<UINT64>((timestamps[1]-timestamps[0])*(1000000000.0/frequency));
        completed++; return true;
    }
    bool drain() {
        if(!queue || !fence || !event) return false;
        if(!ok(queue->Signal(fence,++sequence),"Signal shutdown") || !wait(sequence,false)) return false;
        closed=true; return true;
    }
    // Acceptance tests hold the queue briefly to prove that all three upload
    // buffers stay distinct and that a fourth submission cannot overwrite one.
    bool hold() {
        return ok(device->CreateFence(0,D3D12_FENCE_FLAG_NONE,IID_PPV_ARGS(&gate)),"Create ownership test gate") && ok(queue->Wait(gate,1),"Hold ownership test queue");
    }
    bool resume() { return gate && ok(gate->Signal(1),"Release ownership test queue"); }
    bool validate_messages() {
        if(!diagnostics) return true;
        UINT64 count=diagnostics->GetNumStoredMessagesAllowedByRetrievalFilter();
        for(UINT64 i=0;i<count;i++) {
            SIZE_T length=0;
            if(FAILED(diagnostics->GetMessage(i,nullptr,&length))) continue;
            std::vector<unsigned char> data(length);
            auto message=reinterpret_cast<D3D12_MESSAGE *>(data.data());
            if(FAILED(diagnostics->GetMessage(i,message,&length))) continue;
            if(message->Severity==D3D12_MESSAGE_SEVERITY_ERROR || message->Severity==D3D12_MESSAGE_SEVERITY_CORRUPTION) { error="D3D12 validation: "; error.append(message->pDescription,message->DescriptionByteLength); return false; }
        }
        return true;
    }
};
}
#endif
