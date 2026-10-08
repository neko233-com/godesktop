#ifndef GODESKTOP_DX12_TIMEOUT_PROBE_WINDOWS_H
#define GODESKTOP_DX12_TIMEOUT_PROBE_WINDOWS_H

// Private native acceptance fixture. Uses the production Device and resources;
// no HWND, alternate wait implementation or simulated cancellation is involved.
#include <sstream>
#include <iomanip>
#include "dx12_device.h"

namespace gd_dx12 {
namespace timeout_fixture {
inline UINT64 nanos() {
    static const double scale=[] { LARGE_INTEGER f; QueryPerformanceFrequency(&f); return 1e9/f.QuadPart; }();
    LARGE_INTEGER value; QueryPerformanceCounter(&value);
    return static_cast<UINT64>(value.QuadPart*scale);
}
inline std::string quoted(const std::string &value) {
    std::string result="\"";
    for(unsigned char c:value) {
        if(c=='"' || c=='\\') { result+='\\'; result+=c; }
        else if(c=='\n') result+="\\n";
        else if(c=='\r') result+="\\r";
        else if(c=='\t') result+="\\t";
        else if(c<32) result+='?';
        else result+=c;
    }
    return result+'"';
}
inline int run(std::string &json) {
    bool software=false,debug=false,initial=false,held=false,submitted=false,drained=true;
    bool drainAgain=true,waitAgain=true,signalAgain=true,submitAgain=true,preserved=false,retired=false;
    UINT64 before=0,after=0,submittedFence=0,submissions=0,completions=0;
    ULONGLONG waitMS=0,terminalMS=0,retireMS=0,destructMS=0,beforeDestruct=0;
    HRESULT reason=S_OK;
    std::string first,failure;
    auto exercise=[&]() {
        Device device;
        if(!device.open(false,true,false,false,true)) { failure="open: "+device.error; return false; }
        software=device.software; debug=device.debugLayer;
        if(!software || debug) { failure="expected actual non-debug WARP"; return false; }
        initial=device.drain();
        if(!initial) { failure="initial drain: "+device.error; return false; }
        held=device.hold(); // Reopens the successfully drained healthy queue.
        if(!held) { failure="hold: "+device.error; return false; }
        GDGPUInstance instance{};
        instance.bounds={16,16,64,48}; instance.clip={0,0,192,128};
        instance.color={0.8f,0.2f,0.1f,1}; instance.uv={0,0,1,1}; instance.kind=1;
        const float clear[4]={0.05f,0.1f,0.2f,1};
        submitted=device.submit(0,&instance,1,clear,nanos(),nanos);
        if(!submitted) { failure="actual held submission: "+device.error; return false; }
        before=device.completion()->GetCompletedValue(); submittedFence=device.frames[0].fence;
        if(before==UINT64_MAX || before>=submittedFence) { failure="held real submission was not pending"; return false; }
        const auto start=GetTickCount64();
        drained=device.drain(); // The production five-second deadline/cancellation.
        waitMS=GetTickCount64()-start; first=device.error;
        after=device.completion()->GetCompletedValue(); reason=device.native()->GetDeviceRemovedReason();
        submissions=device.submitted; completions=device.completed;
        const auto terminalStart=GetTickCount64();
        drainAgain=device.drain(); preserved=device.error==first;
        waitAgain=device.wait(submittedFence,false); preserved=preserved && device.error==first;
        UINT64 rejectedFence=0;
        signalAgain=device.signal(&rejectedFence); preserved=preserved && device.error==first;
        submitAgain=device.submit(1,&instance,1,clear,nanos(),nanos); preserved=preserved && device.error==first;
        terminalMS=GetTickCount64()-terminalStart;
        const auto retireStart=GetTickCount64();
        retired=device.retireQueue(); retireMS=GetTickCount64()-retireStart;
        preserved=preserved && device.error==first;
        // No resume or early gate signal is used to make the real wait pass.
        beforeDestruct=GetTickCount64();
        return true; // Device destruction happens before exercise returns.
    };
    const bool reached=exercise();
    if(reached) destructMS=GetTickCount64()-beforeDestruct;
    const bool pass=reached && software && !debug && initial && held && submitted && !drained &&
        waitMS>=4500 && waitMS<=7000 && after==UINT64_MAX && FAILED(reason) &&
        submissions==1 && completions==0 && !drainAgain && !waitAgain && !signalAgain && !submitAgain &&
        preserved && first=="GPU fence did not complete within five seconds" &&
        terminalMS<1000 && retired && retireMS<1000 && destructMS<1000;
    std::ostringstream out;
    out<<std::boolalpha<<"{\"schema\":1,\"pid\":"<<GetCurrentProcessId()<<",\"software\":"<<software<<",\"debug\":"<<debug
       <<",\"initialDrain\":"<<initial<<",\"held\":"<<held<<",\"actualSubmitted\":"<<submissions<<",\"actualCompleted\":"<<completions
       <<",\"firstDrain\":"<<drained<<",\"drainMS\":"<<waitMS<<",\"fenceBefore\":"<<before<<",\"submittedFence\":"<<submittedFence
       <<",\"fenceAfter\":"<<after<<",\"removedReason\":\"0x"<<std::hex<<std::setw(8)<<std::setfill('0')<<static_cast<unsigned long>(reason)<<std::dec
       <<"\",\"terminalDrain\":"<<drainAgain<<",\"terminalWait\":"<<waitAgain<<",\"terminalSignal\":"<<signalAgain<<",\"terminalSubmit\":"<<submitAgain
       <<",\"firstErrorPreserved\":"<<preserved<<",\"terminalMS\":"<<terminalMS<<",\"retired\":"<<retired<<",\"retireMS\":"<<retireMS
       <<",\"destructMS\":"<<destructMS<<",\"firstError\":"<<quoted(first)<<",\"failure\":"<<quoted(failure)<<",\"pass\":"<<pass<<"}";
    json=out.str();
    return pass?0:1;
}
}
}
#endif
