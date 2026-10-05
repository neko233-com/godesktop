#ifndef GODESKTOP_GPU_CAPTURE_WINDOWS_H
#define GODESKTOP_GPU_CAPTURE_WINDOWS_H

namespace gd_dx12 {
// Test-only mapping: the producer copies a completed GPU readback. Consumers
// verify PID/HWND, dimensions and the sequence before/after reading pixels.
struct CaptureHeader {
    uint32_t magic=0x32314447,version=1,pid=0,width=0,height=0,stride=0,reserved[2]{};
    uint64_t hwnd=0;
    volatile LONG64 sequence=0;
    uint64_t frame=0,bytes=0;
};
static_assert(sizeof(CaptureHeader)==64 && offsetof(CaptureHeader,sequence)==40,"GPU capture ABI changed");
class Capture {
    HANDLE mapping=nullptr;
    CaptureHeader *header=nullptr;
public:
    static constexpr size_t capacity=64*1024*1024;
    uint64_t lastFrame=0;
    ~Capture() { if(header) UnmapViewOfFile(header); if(mapping) CloseHandle(mapping); }
    bool open(HWND window) {
        wchar_t name[128];
        std::swprintf(name,128,L"Local\\godesktop.gpu.%lu.%llu",GetCurrentProcessId(),static_cast<unsigned long long>(reinterpret_cast<uintptr_t>(window)));
        mapping=CreateFileMappingW(INVALID_HANDLE_VALUE,nullptr,PAGE_READWRITE,0,capacity+sizeof(CaptureHeader),name);
        if(!mapping || GetLastError()==ERROR_ALREADY_EXISTS) return false;
        header=static_cast<CaptureHeader *>(MapViewOfFile(mapping,FILE_MAP_WRITE,0,0,capacity+sizeof(CaptureHeader)));
        if(!header) return false;
        new(header) CaptureHeader{};
        header->pid=GetCurrentProcessId(); header->hwnd=reinterpret_cast<uintptr_t>(window);
        return true;
    }
    void publish(UINT width,UINT height,const void *data,UINT pitch,uint64_t frame) {
        if(!header || frame<=lastFrame || uint64_t(width)*height*4>capacity) return;
        InterlockedIncrement64(&header->sequence);
        auto target=reinterpret_cast<unsigned char *>(header+1);
        for(UINT y=0;y<height;y++) std::memcpy(target+y*width*4,static_cast<const unsigned char *>(data)+y*pitch,width*4);
        header->width=width; header->height=height; header->stride=width*4;
        header->frame=frame; header->bytes=uint64_t(width)*height*4;
        MemoryBarrier(); InterlockedIncrement64(&header->sequence);
        lastFrame=frame;
    }
};
}
#endif
