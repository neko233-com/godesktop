#import <AppKit/AppKit.h>
#include "bridge.h"
#include <stdlib.h>
#include <string.h>

const char *gd_read_clipboard(char **text) {
    *text=NULL;
    @autoreleasepool {
        NSString *value=[[NSPasteboard generalPasteboard] stringForType:NSPasteboardTypeString];
        if(!value) return "Clipboard does not contain text";
        const char *bytes=[value UTF8String];
        if(!bytes || strlen(bytes)>16u*1024u*1024u) return "Clipboard text exceeds limit";
        *text=strdup(bytes);
        return *text ? NULL : "Clipboard allocation failed";
    }
}
const char *gd_write_clipboard(const char *text) {
    @autoreleasepool {
        NSString *value=[NSString stringWithUTF8String:text];
        if(!value) return "Invalid clipboard text";
        NSPasteboard *board=[NSPasteboard generalPasteboard];
        [board clearContents];
        return [board setString:value forType:NSPasteboardTypeString] ? NULL : "Could not write clipboard";
    }
}
