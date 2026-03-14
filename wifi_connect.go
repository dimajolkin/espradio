//go:build esp32c3

package espradio

/*
#include "include.h"
#include <string.h>

// wifi_sta_config_t has complex bitfields that TinyGo CGo can't parse,
// so we build the config in C and call esp_wifi_set_config here.
static esp_err_t espradio_sta_set_config(const char *ssid, int ssid_len,
                                         const char *pwd, int pwd_len) {
    wifi_config_t cfg;
    memset(&cfg, 0, sizeof(cfg));
    if (ssid_len > 32) ssid_len = 32;
    memcpy(cfg.sta.ssid, ssid, ssid_len);
    if (pwd_len > 64) pwd_len = 64;
    memcpy(cfg.sta.password, pwd, pwd_len);
    if (pwd_len > 0)
        cfg.sta.threshold.authmode = WIFI_AUTH_WPA2_PSK;
    return esp_wifi_set_config(WIFI_IF_STA, &cfg);
}

extern esp_err_t esp_wifi_connect_internal(void);
*/
import "C"

import (
	"sync"
	"time"
	"unsafe"
)

// STAConfig holds Wi-Fi station connection parameters.
type STAConfig struct {
	SSID     string
	Password string
}

// ConnectResult is delivered via channel when a connect/disconnect event arrives.
type ConnectResult struct {
	Connected bool
	SSID      string
	Channel   uint8
	Reason    uint8
}

var (
	connectMu     sync.Mutex
	connectResult chan ConnectResult
)

// Connect configures STA credentials and initiates association.
// Blocks until CONNECTED, DISCONNECTED or timeout.
func Connect(cfg STAConfig) error {
	connectMu.Lock()
	connectResult = make(chan ConnectResult, 1)
	connectMu.Unlock()

	code := C.espradio_sta_set_config(
		C.CString(cfg.SSID), C.int(len(cfg.SSID)),
		C.CString(cfg.Password), C.int(len(cfg.Password)),
	)
	if code != C.ESP_OK {
		println("wifi_connect: set_config err", int32(code))
		return makeError(code)
	}
	println("wifi_connect: config set, ssid=", cfg.SSID)

	if code := C.esp_wifi_connect_internal(); code != C.ESP_OK {
		println("wifi_connect: esp_wifi_connect err", int32(code))
		return makeError(code)
	}
	println("wifi_connect: connecting...")

	select {
	case res := <-connectResult:
		if res.Connected {
			println("wifi_connect: connected to", res.SSID, "ch=", int(res.Channel))
			return nil
		}
		println("wifi_connect: failed, reason=", int(res.Reason))
		return makeError(C.esp_err_t(res.Reason))
	case <-time.After(15 * time.Second):
		println("wifi_connect: timeout")
		return makeError(C.ESP_ERR_TIMEOUT)
	}
}

// espradio_on_wifi_event is called from C (osi.c) for every WIFI_EVENT.
//
//export espradio_on_wifi_event
func espradio_on_wifi_event(eventID int32, data unsafe.Pointer) {
	switch eventID {
	case C.WIFI_EVENT_STA_CONNECTED:
		ev := (*C.wifi_event_sta_connected_t)(data)
		ssidLen := int(ev.ssid_len)
		if ssidLen > 32 {
			ssidLen = 32
		}
		ssid := C.GoBytes(unsafe.Pointer(&ev.ssid[0]), C.int(ssidLen))
		println("wifi_event: STA_CONNECTED ssid=", string(ssid), "ch=", int(ev.channel))
		connectMu.Lock()
		ch := connectResult
		connectMu.Unlock()
		if ch != nil {
			select {
			case ch <- ConnectResult{Connected: true, SSID: string(ssid), Channel: uint8(ev.channel)}:
			default:
			}
		}

	case C.WIFI_EVENT_STA_DISCONNECTED:
		ev := (*C.wifi_event_sta_disconnected_t)(data)
		println("wifi_event: STA_DISCONNECTED reason=", int(ev.reason))
		connectMu.Lock()
		ch := connectResult
		connectMu.Unlock()
		if ch != nil {
			select {
			case ch <- ConnectResult{Connected: false, Reason: uint8(ev.reason)}:
			default:
			}
		}

	case C.WIFI_EVENT_STA_START:
		println("wifi_event: STA_START")
	}
}
