#include "bridge.h"
#include <windows.h>
#include <cstdlib>
#include <cstring>

extern "C" const char *gd_read_clipboard(char **text) {
    *text=nullptr;
    if(!OpenClipboard(GetActiveWindow())) return "Could not open clipboard";
    HANDLE data=GetClipboardData(CF_UNICODETEXT);
    if(!data) { CloseClipboard(); return "Clipboard does not contain text"; }
    const size_t bytes=GlobalSize(data);
    if(bytes>32u*1024u*1024u) { CloseClipboard(); return "Clipboard text exceeds limit"; }
    const wchar_t *value=(const wchar_t *)GlobalLock(data);
    if(!value) { CloseClipboard(); return "Could not read clipboard"; }
    size_t length=0;
    while(length<bytes/sizeof(wchar_t) && value[length]) ++length;
    const char *error=nullptr;
    if(length==bytes/sizeof(wchar_t)) error="Invalid clipboard text";
    else if(!length) { *text=(char*)malloc(1); if(*text) **text=0; else error="Clipboard allocation failed"; }
    else {
        int size=WideCharToMultiByte(CP_UTF8,WC_ERR_INVALID_CHARS,value,(int)length,nullptr,0,nullptr,nullptr);
        if(!size || size>16*1024*1024) error="Invalid clipboard text";
        else { *text=(char*)malloc(size+1); if(!*text) error="Clipboard allocation failed";
            else { WideCharToMultiByte(CP_UTF8,WC_ERR_INVALID_CHARS,value,(int)length,*text,size,nullptr,nullptr); (*text)[size]=0; } }
    }
    GlobalUnlock(data); CloseClipboard(); return error;
}

extern "C" const char *gd_write_clipboard(const char *text) {
    int length=MultiByteToWideChar(CP_UTF8,MB_ERR_INVALID_CHARS,text,-1,nullptr,0);
    if(!length) return "Invalid clipboard text";
    HGLOBAL block=GlobalAlloc(GMEM_MOVEABLE,length*sizeof(wchar_t));
    if(!block) return "Clipboard allocation failed";
    wchar_t *value=(wchar_t*)GlobalLock(block);
    if(!value) { GlobalFree(block); return "Clipboard allocation failed"; }
    MultiByteToWideChar(CP_UTF8,MB_ERR_INVALID_CHARS,text,-1,value,length); GlobalUnlock(block);
    if(!OpenClipboard(GetActiveWindow())) { GlobalFree(block); return "Could not open clipboard"; }
    const char *error=nullptr;
    if(!EmptyClipboard() || !SetClipboardData(CF_UNICODETEXT,block)) { error="Could not write clipboard"; GlobalFree(block); }
    CloseClipboard(); return error;
}
