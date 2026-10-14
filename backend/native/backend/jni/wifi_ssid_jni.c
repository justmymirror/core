//go:build !android

#include <jni.h>
#include <stdlib.h>

extern int wgtunnel_wifi_authorization(void);
extern void wgtunnel_wifi_request_authorization(void);
extern int wgtunnel_wifi_associated(void);
extern char *wgtunnel_wifi_current_ssid(void);
extern int wgtunnel_wifi_wait(double timeoutSeconds);
extern void wgtunnel_wifi_wake_wait(void);
extern void setObservedWifiSSID(char *ssid);

JNIEXPORT jint JNICALL
Java_com_zaneschepke_wireguardautotunnel_core_wifi_WifiSsidNative_authorization(
	JNIEnv *env, jclass c)
{
	(void)env;
	(void)c;
	return (jint)wgtunnel_wifi_authorization();
}

JNIEXPORT void JNICALL
Java_com_zaneschepke_wireguardautotunnel_core_wifi_WifiSsidNative_requestAuthorization(
	JNIEnv *env, jclass c)
{
	(void)env;
	(void)c;
	wgtunnel_wifi_request_authorization();
}

JNIEXPORT jboolean JNICALL
Java_com_zaneschepke_wireguardautotunnel_core_wifi_WifiSsidNative_associated(
	JNIEnv *env, jclass c)
{
	(void)env;
	(void)c;
	return wgtunnel_wifi_associated() ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT jstring JNICALL
Java_com_zaneschepke_wireguardautotunnel_core_wifi_WifiSsidNative_currentSsid(
	JNIEnv *env, jclass c)
{
	char *ssid;
	jstring ret;
	(void)c;

	ssid = wgtunnel_wifi_current_ssid();
	if (ssid == NULL)
		return (*env)->NewStringUTF(env, "");
	ret = (*env)->NewStringUTF(env, ssid);
	free(ssid);
	return ret;
}

JNIEXPORT jboolean JNICALL
Java_com_zaneschepke_wireguardautotunnel_core_wifi_WifiSsidNative_waitForEvents(
	JNIEnv *env, jclass c, jdouble timeoutSeconds)
{
	(void)env;
	(void)c;
	return wgtunnel_wifi_wait((double)timeoutSeconds) ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT void JNICALL
Java_com_zaneschepke_wireguardautotunnel_core_wifi_WifiSsidNative_wakeWait(
	JNIEnv *env, jclass c)
{
	(void)env;
	(void)c;
	wgtunnel_wifi_wake_wait();
}

JNIEXPORT void JNICALL
Java_com_zaneschepke_wireguardautotunnel_core_wifi_WifiSsidNative_setObserved(
	JNIEnv *env, jclass c, jstring ssid)
{
	const char *utf = NULL;
	(void)c;

	if (ssid != NULL)
		utf = (*env)->GetStringUTFChars(env, ssid, NULL);
	setObservedWifiSSID((char *)(utf != NULL ? utf : ""));
	if (utf != NULL)
		(*env)->ReleaseStringUTFChars(env, ssid, utf);
}
