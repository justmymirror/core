//go:build !darwin && !android

#include <stdlib.h>

int
wgtunnel_wifi_authorization(void)
{
	return -1;
}

void
wgtunnel_wifi_request_authorization(void)
{
}

int
wgtunnel_wifi_associated(void)
{
	return 0;
}

char *
wgtunnel_wifi_current_ssid(void)
{
	return NULL;
}

int
wgtunnel_wifi_wait(double timeoutSeconds)
{
	(void)timeoutSeconds;
	return 0;
}

void
wgtunnel_wifi_wake_wait(void)
{
}
