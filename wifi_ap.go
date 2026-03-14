//go:build esp32c3

package espradio

/*
#include <stdlib.h>
#include <stddef.h>
#include <stdint.h>
typedef int esp_err_t;
int espradio_start_ap_impl(const char* ssid, size_t ssid_len,
    const char* password, size_t pwd_len, uint8_t channel, int auth_open);
*/
import "C"

import "unsafe"

// APConfig holds soft-AP configuration.
type APConfig struct {
	SSID     string
	Password string
	Channel  uint8
	AuthOpen bool
}

// StartAP starts the radio in soft-AP mode with the given configuration.
// Must be called after Enable.
func StartAP(cfg APConfig) error {
	ssid := cfg.SSID
	pwd := cfg.Password
	if len(ssid) == 0 {
		ssid = "espradio-ap"
	}
	ssidC := C.CString(ssid)
	defer C.free(unsafe.Pointer(ssidC))
	pwdC := C.CString(pwd)
	defer C.free(unsafe.Pointer(pwdC))
	code := C.espradio_start_ap_impl(ssidC, C.size_t(len(ssid)),
		pwdC, C.size_t(len(pwd)),
		C.uint8_t(cfg.Channel), C.int(boolToInt(cfg.AuthOpen)))
	if code != 0 {
		return makeError(C.esp_err_t(code))
	}
	println("wifi_ap: AP started", cfg.SSID)
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
