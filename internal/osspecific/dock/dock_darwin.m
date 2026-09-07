//go:build darwin && cgo

#import <Cocoa/Cocoa.h>

extern void nativeStart(void);

static id savedDelegate = nil;

void SaveAppDelegate(void) {
    if ([NSThread isMainThread]) {
        savedDelegate = [[NSApplication sharedApplication] delegate];
    } else {
        dispatch_sync(dispatch_get_main_queue(), ^{
            savedDelegate = [[NSApplication sharedApplication] delegate];
        });
    }
}

void RestoreAppDelegate(void) {
    if ([NSThread isMainThread]) {
        if (savedDelegate != nil) {
            [[NSApplication sharedApplication] setDelegate:savedDelegate];
        }
    } else {
        dispatch_sync(dispatch_get_main_queue(), ^{
            if (savedDelegate != nil) {
                [[NSApplication sharedApplication] setDelegate:savedDelegate];
            }
        });
    }
}

void SafeStartSystray(void) {
    void (^block)(void) = ^{
        savedDelegate = [[NSApplication sharedApplication] delegate];
        nativeStart();
        if (savedDelegate != nil) {
            [[NSApplication sharedApplication] setDelegate:savedDelegate];
        }
    };

    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}
