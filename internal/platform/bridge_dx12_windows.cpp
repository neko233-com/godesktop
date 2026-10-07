//go:build windows && cgo

#define NOMINMAX
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <sstream>
#include <cstdlib>
#include <iomanip>
#include <thread>
#include "dx12_device.h"

namespace {
uint64_t probe_nanos() {
    static const double scale=[] { LARGE_INTEGER value; QueryPerformanceFrequency(&value); return 1000000000.0/value.QuadPart; }();
    LARGE_INTEGER value; QueryPerformanceCounter(&value);
    return static_cast<uint64_t>(value.QuadPart*scale);
}
std::string json_string(const std::string &value) {
    std::ostringstream out; out << '"';
    for(unsigned char c:value) {
        if(c=='"' || c=='\\') out << '\\' << c;
        else if(c<32) out << "\\u" << std::hex << std::setw(4) << std::setfill('0') << static_cast<unsigned>(c);
        else out << c;
    }
    out << '"'; return out.str();
}
std::array<GDGPUInstance,8> probe_scene(unsigned frame) {
    GDRect all{0,0,192,128};
    std::array<GDGPUInstance,8> scene{};
    auto quad=[&](unsigned index,GDRect bounds,GDColor color,float radius=0) {
        auto &v=scene[index]; v.bounds=bounds; v.clip=all; v.color=color;
        v.radius=radius; v.kind=1; v.uv={0,0,1,1};
    };
    quad(0,{8,8,56,40},frame%2?GDColor{0.8f,0.2f,0.1f,1}:GDColor{0.1f,0.3f,0.8f,1});
    quad(1,{24,16,32,24},{1,0,0,0.5f}); scene[1].clip={32,20,16,16};
    quad(2,{80,8,40,40},{0,1,0,1},12);
    quad(3,{8,80,56,0},{1,1,1,1},4); scene[3].kind=3;
    quad(4,{80,64,32,32},{1,1,1,1}); scene[4].kind=2;
    quad(5,{160,50,20,20},{1,0,1,1}); scene[5].clip={0,0,0,0};
    quad(6,{128,8,40,40},{1,1,0,1},2); scene[6].kind=3;
    quad(7,{150,90,24,24},{1,0.5f,0,1});
    return scene;
}
}

extern "C" const char *gd_dx12_probe(uint32_t flags,uint32_t frameCount,GDGPUProbe *result) {
    static thread_local std::string failure;
    failure.clear(); *result={};
    if(frameCount<6 || frameCount>240 || (flags&3)==3 || (flags&~15u)) return "Invalid Direct3D 12 probe options";
    gd_dx12::Device renderer;
    if(!renderer.open(flags&1,flags&2,flags&4,flags&8)) { failure=renderer.error; return failure.c_str(); }
    // Preserve an actual older GPU completion notification while a later fence
    // is blocked behind an independent queue gate. Shutdown/readback must wait
    // for the requested value rather than treating the old wakeup as failure.
    if(!renderer.watch(renderer.completion()->GetCompletedValue())) { failure=renderer.error; return failure.c_str(); }
    ID3D12Fence *delayed=nullptr;
    HRESULT hr=renderer.native()->CreateFence(0,D3D12_FENCE_FLAG_NONE,IID_PPV_ARGS(&delayed));
    if(FAILED(hr)) return "Create stale completion queue gate failed";
    hr=renderer.commands()->Wait(delayed,1);
    UINT64 target=0;
    if(FAILED(hr) || !renderer.signal(&target)) { delayed->Signal(1); gd_dx12::drop(delayed); return "Queue stale completion regression failed"; }
    std::thread release([delayed] { Sleep(100); delayed->Signal(1); });
    bool waited=renderer.wait(target,false);
    UINT64 returnedFence=renderer.completion()->GetCompletedValue();
    release.join();
    gd_dx12::drop(delayed);
    if(!waited || returnedFence==UINT64_MAX || returnedFence<target) { failure=renderer.error.empty()?"Stale completion bypassed the requested GPU fence":renderer.error; return failure.c_str(); }
    result->width=renderer.width; result->height=renderer.height;
    result->frames=frameCount; result->stride=renderer.width*4;
    size_t frameBytes=result->stride*result->height;
    result->pixels=static_cast<unsigned char *>(std::malloc(frameBytes*frameCount));
    if(!result->pixels) return "D3D12 probe image allocation failed";
    std::vector<uint64_t> cpuTimes,gpuTimes;
    std::array<unsigned,3> pending{};
    if(!renderer.hold()) { failure=renderer.error; return failure.c_str(); }
    const float clear[4]={0.05f,0.1f,0.2f,1};
    for(unsigned first=0;first<frameCount;first+=3) {
        unsigned count=std::min(3u,frameCount-first);
        for(unsigned slot=0;slot<count;slot++) {
            uint64_t started=probe_nanos();
            auto scene=probe_scene(first+slot); pending[slot]=first+slot;
            if(!renderer.submit(slot,scene.data(),scene.size(),clear,started,probe_nanos)) { failure=renderer.error.empty()?"D3D12 submission unexpectedly deferred":renderer.error; return failure.c_str(); }
            cpuTimes.push_back(renderer.frames[slot].cpuNanos);
        }
        if(first==0) {
            auto poison=probe_scene(999);
            if(renderer.submit(0,poison.data(),poison.size(),clear,probe_nanos(),probe_nanos) || !renderer.error.empty()) { failure="In-flight D3D12 allocator/upload buffer was not protected"; return failure.c_str(); }
            if(!renderer.resume()) { failure=renderer.error; return failure.c_str(); }
        }
        for(unsigned slot=0;slot<count;slot++) {
            uint64_t gpuTime=0;
            if(!renderer.collect(slot,result->pixels+pending[slot]*frameBytes,&gpuTime)) { failure=renderer.error; return failure.c_str(); }
            gpuTimes.push_back(gpuTime);
        }
    }
    if(!renderer.drain() || !renderer.validate_messages()) { failure=renderer.error; return failure.c_str(); }
    auto numbers=[](const std::vector<uint64_t> &values) {
        std::ostringstream out; out << '[';
        for(size_t i=0;i<values.size();i++) { if(i) out << ','; out << values[i]; }
        out << ']'; return out.str();
    };
    std::ostringstream report;
    report << "{\"backend\":\"direct3d12\",\"adapter\":" << json_string(renderer.adapterName)
        << ",\"software\":" << (renderer.software?"true":"false")
        << ",\"vendor_id\":" << renderer.vendorID << ",\"device_id\":" << renderer.deviceID << ",\"adapter_flags\":" << renderer.adapterFlags
        << ",\"debug_layer\":" << (renderer.debugLayer?"true":"false")
        << ",\"gpu_validation\":" << (renderer.gpuValidation?"true":"false")
        << ",\"shader_model\":\"6.0\",\"frame_slots\":3,\"used_slots_mask\":" << renderer.usedSlots
        << ",\"max_in_flight\":" << renderer.maxInFlight
        << ",\"submitted\":" << renderer.submitted << ",\"completed\":" << renderer.completed
        << ",\"ownership_deferrals\":" << renderer.ownershipDeferrals
        << ",\"diagnostic_readback_waits\":" << renderer.readbackWaits
        << ",\"stale_completion_checks\":1,\"diagnostic_queue_hold_ms\":100"
        << ",\"instances_per_frame\":8,\"draw_calls_per_frame\":1,\"instance_bytes_per_frame\":" << 8*sizeof(GDGPUInstance)
        << ",\"cpu_samples_nanos\":" << numbers(cpuTimes) << ",\"gpu_samples_nanos\":" << numbers(gpuTimes) << '}';
    result->json=strdup(report.str().c_str());
    if(!result->json) return "D3D12 report allocation failed";
    return nullptr;
}
