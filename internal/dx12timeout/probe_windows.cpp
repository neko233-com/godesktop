//go:build windows && cgo

#define NOMINMAX
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <cstdlib>
#include <cstring>
#include "../platform/dx12_timeout_probe_windows.h"

extern "C" int gd_private_dx12_timeout(char **text,size_t *length) {
    *text=nullptr; *length=0;
    std::string json;
    const int code=gd_dx12::timeout_fixture::run(json);
    if(json.size()>16384) return 2;
    auto output=static_cast<char *>(std::malloc(json.size()+1));
    if(!output) return 2;
    std::memcpy(output,json.data(),json.size()); output[json.size()]=0;
    *text=output; *length=json.size();
    return code;
}
