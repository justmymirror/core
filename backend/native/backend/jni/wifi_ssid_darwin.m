//go:build darwin

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <CoreFoundation/CoreFoundation.h>
#import <CoreLocation/CoreLocation.h>
#import <CoreWLAN/CoreWLAN.h>
#import <dispatch/dispatch.h>
#import <stdatomic.h>
#import <stdlib.h>
#import <string.h>

// CoreWLAN only returns the SSID to a user-session process that has created
// NSApplication and whose bundle has been granted Location. AppKit is
// weak-linked so the root daemon, which never calls these entry points, does
// not load it.
//
// The agent has no AppKit run loop of its own. It runs this thread's
// CoreFoundation run loop while waiting, so
// locationManagerDidChangeAuthorization can fire here when Location is granted.

static CLLocationManager *gManager;
static id<CLLocationManagerDelegate> gDelegate;
static atomic_int gAuthGen;
static atomic_int gExternalWake;
static _Atomic(CFRunLoopSourceRef) gWakeSource;

static void signalWaiters(void);

static void
wakePerform(void *info)
{
	(void)info;
	CFRunLoopStop(CFRunLoopGetMain());
}

static void
ensureWakeSource(void)
{
	CFRunLoopSourceContext ctx;
	CFRunLoopSourceRef source;

	if (atomic_load(&gWakeSource) != NULL)
		return;
	memset(&ctx, 0, sizeof(ctx));
	ctx.version = 0;
	ctx.perform = wakePerform;
	source = CFRunLoopSourceCreate(kCFAllocatorDefault, 0, &ctx);
	CFRunLoopAddSource(CFRunLoopGetMain(), source, kCFRunLoopDefaultMode);
	atomic_store(&gWakeSource, source);
}

static void
signalWaiters(void)
{
	CFRunLoopSourceRef source = atomic_load(&gWakeSource);
	if (source == NULL)
		return;
	CFRunLoopSourceSignal(source);
	CFRunLoopWakeUp(CFRunLoopGetMain());
}

@interface WgWifiLocationDelegate : NSObject <CLLocationManagerDelegate>
@end

@implementation WgWifiLocationDelegate
- (void)locationManagerDidChangeAuthorization:(CLLocationManager *)manager
{
	(void)manager;
	atomic_fetch_add(&gAuthGen, 1);
	signalWaiters();
}
@end

static void
ensureManagerOnMain(void)
{
	if (gManager != nil)
		return;
	gDelegate = [[WgWifiLocationDelegate alloc] init];
	gManager = [[CLLocationManager alloc] init];
	gManager.delegate = gDelegate;
}

static void
onMain(void (^block)(void))
{
	if ([NSThread isMainThread]) {
		block();
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), block);
}

/* CoreWLAN returns a nil name until this process has created NSApplication. */
static void
ensureAppKit(void)
{
	[NSApplication sharedApplication];
}

static void
activateForPrompt(void)
{
	if (@available(macOS 14.0, *)) {
		[NSApp activate];
		return;
	}
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
	[NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
}

static int
codeForStatus(CLAuthorizationStatus status)
{
	switch (status) {
	case kCLAuthorizationStatusNotDetermined:
		return 0;
	case kCLAuthorizationStatusRestricted:
		return 1;
	case kCLAuthorizationStatusDenied:
		return 2;
	case kCLAuthorizationStatusAuthorizedAlways:
		return 3;
	default:
		/*
		 * kCLAuthorizationStatusAuthorizedWhenInUse is iOS only
		 */
		return -1;
	}
}

static int
authorizationOnMain(void)
{
	__block int code = -1;
	onMain(^{
		CLAuthorizationStatus status;
		ensureAppKit();
		ensureManagerOnMain();
		if (@available(macOS 11.0, *)) {
			status = gManager.authorizationStatus;
		} else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
			status = [CLLocationManager authorizationStatus];
#pragma clang diagnostic pop
		}
		/*
		 * The instance property stays notDetermined until this process
		 * calls requestAlwaysAuthorization. A grant already recorded in
		 * System Settings is visible on the class method. macOS 13 still
		 * has that method. An initial notDetermined callback must not
		 * cover a later authorized read: callers re-read this function.
		 */
		if (status == kCLAuthorizationStatusNotDetermined) {
			CLAuthorizationStatus recorded;
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
			recorded = [CLLocationManager authorizationStatus];
#pragma clang diagnostic pop
			if (recorded != kCLAuthorizationStatusNotDetermined)
				status = recorded;
		}
		code = codeForStatus(status);
	});
	return code;
}

int
wgtunnel_wifi_authorization(void)
{
	/*
	 * locationServicesEnabled can block. Keep it off the main thread.
	 * 5 is "services off", distinct from a per-app denial.
	 */
	if (![CLLocationManager locationServicesEnabled])
		return 5;
	return authorizationOnMain();
}

static void
requestOnMain(void)
{
	ensureAppKit();
	activateForPrompt();
	ensureManagerOnMain();
	[gManager requestAlwaysAuthorization];
}

void
wgtunnel_wifi_request_authorization(void)
{
	/*
	 * Present on a later turn of the main queue. Calling this synchronously
	 * from the click returns before AppKit will show the panel.
	 */
	dispatch_async(dispatch_get_main_queue(), ^{
		requestOnMain();
	});
}

/*
 * Block this thread until timeoutSeconds elapses, the daemon socket asks for
 * a wake, or Location authorization changes. timeoutSeconds < 0 waits until
 * one of those wakes. Returns 1 when authorization changed. Main thread only:
 * the CoreLocation callback is delivered on the run loop started here.
 */
int
wgtunnel_wifi_wait(double timeoutSeconds)
{
	CFAbsoluteTime deadline = 0;
	int authBefore;

	if (![NSThread isMainThread])
		return 0;
	ensureWakeSource();
	ensureAppKit();
	ensureManagerOnMain();
	authBefore = atomic_load(&gAuthGen);
	if (timeoutSeconds >= 0)
		deadline = CFAbsoluteTimeGetCurrent() + timeoutSeconds;
	for (;;) {
		int authChanged = atomic_load(&gAuthGen) != authBefore;
		int external = atomic_exchange(&gExternalWake, 0) == 1;
		if (authChanged)
			return 1;
		if (external)
			return 0;
		if (timeoutSeconds >= 0 && CFAbsoluteTimeGetCurrent() >= deadline)
			return 0;
		{
			double slice = timeoutSeconds < 0 ? 3600.0 : deadline - CFAbsoluteTimeGetCurrent();
			if (slice <= 0)
				return 0;
			CFRunLoopRunInMode(kCFRunLoopDefaultMode, slice, false);
		}
	}
}

/* Wakes wgtunnel_wifi_wait from the daemon-socket thread. */
void
wgtunnel_wifi_wake_wait(void)
{
	atomic_store(&gExternalWake, 1);
	signalWaiters();
}

static NSString *
ssidFromInterface(CWInterface *iface)
{
	NSData *data;

	if (iface == nil)
		return nil;
	/*
	 * macOS 15 deprecates the NSString ssid property. Prefer ssidData, and
	 * keep the string property when that data is empty or not UTF-8.
	 */
	if (@available(macOS 15.0, *)) {
		NSString *decoded;
		data = iface.ssidData;
		if (data.length > 0) {
			decoded = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
			if (decoded.length > 0)
				return decoded;
		}
	}
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
	return iface.ssid;
#pragma clang diagnostic pop
}

int
wgtunnel_wifi_associated(void)
{
	__block int associated = 0;
	onMain(^{
		ensureAppKit();
		@autoreleasepool {
			CWInterface *iface = [[CWWiFiClient sharedWiFiClient] interface];
			if (iface == nil || !iface.powerOn)
				return;
			associated = iface.interfaceMode != kCWInterfaceModeNone;
		}
	});
	return associated;
}

char *
wgtunnel_wifi_current_ssid(void)
{
	__block char *out = NULL;
	onMain(^{
		ensureAppKit();
		@autoreleasepool {
			CWInterface *iface = [[CWWiFiClient sharedWiFiClient] interface];
			NSString *ssid = ssidFromInterface(iface);
			const char *utf;
			if (ssid.length == 0)
				return;
			utf = ssid.UTF8String;
			if (utf == NULL)
				return;
			out = strdup(utf);
		}
	});
	return out;
}
