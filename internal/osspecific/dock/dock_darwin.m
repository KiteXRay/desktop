//go:build darwin && cgo

#import <Cocoa/Cocoa.h>


void SetApplicationIconFromPNG(const void* bytes, int length) {
    if (!bytes || length <= 0) return;
    NSData *data = [NSData dataWithBytes:bytes length:length];
    NSImage *image = [[NSImage alloc] initWithData:data];
    if (image) {
        void (^setIconBlock)(void) = ^{
            [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
            [NSApp setApplicationIconImage:image];
        };
        if ([NSThread isMainThread]) {
            setIconBlock();
        } else {
            dispatch_sync(dispatch_get_main_queue(), setIconBlock);
        }
    }
}
