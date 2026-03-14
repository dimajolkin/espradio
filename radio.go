package espradio

/*
#cgo CFLAGS: -Iblobs/include
#cgo CFLAGS: -Iblobs/include/esp32c3
#cgo CFLAGS: -Iblobs/include/local
#cgo CFLAGS: -Iblobs/headers
#cgo CFLAGS: -DCONFIG_SOC_WIFI_NAN_SUPPORT=0
#cgo CFLAGS: -DESPRADIO_PHY_PATCH_ROMFUNCS=0
#cgo LDFLAGS: -Lblobs/libs/esp32c3 -lcoexist -lcore -lmesh -lnet80211 -lespnow -lregulatory -lphy -lpp -lwpa_supplicant

#include "include.h"
#include <string.h>
#include <stdlib.h>

void espradio_set_blob_log_level(uint32_t level);
esp_err_t espradio_wifi_init(void);
void espradio_wifi_init_completed(void);
void espradio_timer_fire(void *ptimer);
void espradio_event_register_default_cb(void);
void espradio_event_loop_run_once(void);
int espradio_fire_one_pending_timer(void);
int espradio_timer_poll_due(int max_fire);
void espradio_fire_pending_timers(void);
int espradio_esp_timer_poll_due(int max_fire);
void espradio_prepare_memory_for_wifi(void);
void espradio_ensure_osi_ptr(void);
void espradio_coex_adapter_init(void);
void espradio_call_saved_isr(int32_t n);
int32_t espradio_queue_send(void *queue, void *item, uint32_t block_time_tick);
uint32_t espradio_isr_ring_head(void);
uint32_t espradio_isr_ring_tail(void);
void     espradio_isr_ring_advance_tail(void);
void    *espradio_isr_ring_entry_queue(uint32_t idx);
void    *espradio_isr_ring_entry_item(uint32_t idx);
void espradio_set_task_stack_bottom(unsigned long bottom);
unsigned long espradio_stack_remaining(void);
uint32_t espradio_wifi_boot_state(void);
void espradio_hal_init_clocks_go(void);
void espradio_test_pll(void);
int rtc_get_reset_reason(int cpu_no);

int espradio_esp_wifi_start(void);

static esp_err_t espradio_set_country_eu_manual(void) {
	wifi_country_t c;
	esp_err_t rc = esp_wifi_get_country(&c);
	if (rc != ESP_OK) return rc;
	c.cc[0] = 'E'; c.cc[1] = 'U'; c.cc[2] = ' ';
	c.schan = 1; c.nchan = 13;
	c.policy = WIFI_COUNTRY_POLICY_MANUAL;
	return esp_wifi_set_country(&c);
}

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

int espradio_start_ap_impl(const char* ssid, size_t ssid_len,
    const char* password, size_t pwd_len, uint8_t channel, int auth_open);

static volatile uint32_t espradio_sniff_packets = 0;

static void espradio_promisc_rx_cb(void *buf, wifi_promiscuous_pkt_type_t type) {
	(void)buf; (void)type;
	espradio_sniff_packets++;
}

static esp_err_t espradio_sniff_begin(uint8_t channel) {
	wifi_promiscuous_filter_t filter;
	filter.filter_mask = WIFI_PROMIS_FILTER_MASK_MGMT | WIFI_PROMIS_FILTER_MASK_CTRL | WIFI_PROMIS_FILTER_MASK_DATA;
	espradio_sniff_packets = 0;
	esp_err_t rc = esp_wifi_set_promiscuous(false);
	(void)rc;
	rc = esp_wifi_set_channel(channel, WIFI_SECOND_CHAN_NONE);
	if (rc != ESP_OK) return rc;
	rc = esp_wifi_set_promiscuous_filter(&filter);
	if (rc != ESP_OK) return rc;
	rc = esp_wifi_set_promiscuous_rx_cb(espradio_promisc_rx_cb);
	if (rc != ESP_OK) return rc;
	return esp_wifi_set_promiscuous(true);
}

static esp_err_t espradio_sniff_end(void) {
	return esp_wifi_set_promiscuous(false);
}

static uint32_t espradio_sniff_count(void) {
	return espradio_sniff_packets;
}
*/
import "C"
import (
	"bytes"
	"runtime"
	"runtime/interrupt"
	"sync"
	"time"
	"unsafe"
)

// ─── Types ───────────────────────────────────────────────────────────────────

type LogLevel uint8

const (
	LogLevelNone    = C.WIFI_LOG_NONE
	LogLevelError   = C.WIFI_LOG_ERROR
	LogLevelWarning = C.WIFI_LOG_WARNING
	LogLevelInfo    = C.WIFI_LOG_INFO
	LogLevelDebug   = C.WIFI_LOG_DEBUG
	LogLevelVerbose = C.WIFI_LOG_VERBOSE
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelNone:
		return "NONE"
	case LogLevelError:
		return "ERROR"
	case LogLevelWarning:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelVerbose:
		return "VERBOSE"
	default:
		return "?"
	}
}

type Config struct {
	Logging LogLevel
}

type AccessPoint struct {
	SSID string
	RSSI int
}

type STAConfig struct {
	SSID     string
	Password string
}

type ConnectResult struct {
	Connected bool
	SSID      string
	Channel   uint8
	Reason    uint8
}

type APConfig struct {
	SSID     string
	Password string
	Channel  uint8
	AuthOpen bool
}

// ─── Enable ──────────────────────────────────────────────────────────────────

const schedTickerMs = 5

var eventLoopKick chan struct{}

func startSchedTicker() {
	eventLoopKick = make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(schedTickerMs * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-eventLoopKick:
			}

			for C.espradio_isr_ring_tail() != C.espradio_isr_ring_head() {
				idx := C.espradio_isr_ring_tail()
				q := C.espradio_isr_ring_entry_queue(idx)
				item := C.espradio_isr_ring_entry_item(idx)
				C.espradio_queue_send(q, item, 0)
				C.espradio_isr_ring_advance_tail()
			}

			for i := 0; i < 4; i++ {
				C.espradio_event_loop_run_once()
			}
			for i := 0; i < 4; i++ {
				fired := C.espradio_timer_poll_due(8)
				if fired == 0 {
					break
				}
			}
			for i := 0; i < 4; i++ {
				if C.espradio_esp_timer_poll_due(8) == 0 {
					break
				}
			}
		}
	}()
}

//export espradio_event_loop_kick_go
func espradio_event_loop_kick_go() {
	if eventLoopKick == nil {
		return
	}
	select {
	case eventLoopKick <- struct{}{}:
	default:
	}
}

// Enable and configure the radio.
func Enable(config Config) error {
	startSchedTicker()
	time.Sleep(schedTickerMs * time.Millisecond)
	initHardware()
	C.espradio_ensure_osi_ptr()
	initWiFiISR()
	C.espradio_event_register_default_cb()
	C.espradio_set_blob_log_level(C.uint32_t(config.Logging))

	mask := interrupt.Disable()
	C.espradio_hal_init_clocks_go()
	interrupt.Restore(mask)

	C.espradio_test_pll()

	errCode := C.espradio_wifi_init()
	if errCode != 0 {
		return makeError(errCode)
	}
	C.espradio_wifi_init_completed()
	C.espradio_test_pll()

	return nil
}

// ─── Start / Scan ────────────────────────────────────────────────────────────

func Start() error {
	var mode C.wifi_mode_t
	if code := C.esp_wifi_get_mode(&mode); code != C.ESP_OK {
		println("wifi_start: esp_wifi_get_mode err", int32(code))
	} else {
		println("wifi_start: mode before start =", int(mode))
	}
	if mode != C.WIFI_MODE_STA {
		println("wifi_start: set_mode STA")
		if code := C.esp_wifi_set_mode(C.WIFI_MODE_STA); code != C.ESP_OK {
			println("wifi_start: esp_wifi_set_mode err", int32(code))
			return makeError(code)
		}
	}

	if code := C.espradio_esp_wifi_start(); code != C.ESP_OK {
		println("wifi_start: esp_wifi_start err", int32(code))
		return makeError(code)
	}

	enableWiFiISR()

	return nil
}

// Scan performs a single Wi-Fi scan pass and returns the list of discovered access points.
func Scan() ([]AccessPoint, error) {
	println("wifi_scan: get_mode")
	var mode C.wifi_mode_t
	if code := C.esp_wifi_get_mode(&mode); code != C.ESP_OK {
		println("wifi_scan: esp_wifi_get_mode err", int32(code))
	} else {
		println("wifi_scan: mode after start =", int(mode))
	}

	println("wifi_scan: scan_start")
	C.espradio_ensure_osi_ptr()
	if code := C.esp_wifi_set_ps(C.WIFI_PS_NONE); code != C.ESP_OK {
		println("wifi_scan: esp_wifi_set_ps err", int32(code))
	}
	if code := C.espradio_set_country_eu_manual(); code != C.ESP_OK {
		println("wifi_scan: esp_wifi_set_country err", int32(code))
	} else {
		var ctry C.wifi_country_t
		if gc := C.esp_wifi_get_country(&ctry); gc == C.ESP_OK {
			println(
				"wifi_scan: country cc=", int32(ctry.cc[0]), int32(ctry.cc[1]), int32(ctry.cc[2]),
				"schan=", int(ctry.schan), "nchan=", int(ctry.nchan), "policy=", int(ctry.policy),
			)
		}
	}
	time.Sleep(250 * time.Millisecond)
	var scanCfg C.wifi_scan_config_t
	scanCfg.ssid = nil
	scanCfg.bssid = nil
	scanCfg.channel = 0
	scanCfg.show_hidden = false
	scanCfg.scan_type = C.WIFI_SCAN_TYPE_ACTIVE
	scanCfg.scan_time.active.min = 0
	scanCfg.scan_time.active.max = 300
	scanCfg.scan_time.passive = 500
	println(
		"wifi_scan: cfg ch=", int(scanCfg.channel),
		"hidden=", boolToInt(bool(scanCfg.show_hidden)),
		"type=", int(scanCfg.scan_type),
		"active[min,max]=", int(scanCfg.scan_time.active.min), int(scanCfg.scan_time.active.max),
		"passive=", int(scanCfg.scan_time.passive),
	)
	if code := C.esp_wifi_scan_start(&scanCfg, true); code != C.ESP_OK {
		println("wifi_scan: esp_wifi_scan_start err", int32(code))
		return nil, makeError(code)
	}

	var num C.uint16_t
	println("wifi_scan: get_ap_num")
	if code := C.esp_wifi_scan_get_ap_num(&num); code != C.ESP_OK {
		println("wifi_scan: esp_wifi_scan_get_ap_num err", int32(code))
		return nil, makeError(code)
	}
	println("wifi_scan: ap_num", int(num))
	if num == 0 {
		return nil, nil
	}

	recs := make([]C.wifi_ap_record_t, int(num))
	println("wifi_scan: get_ap_records")
	if code := C.esp_wifi_scan_get_ap_records(
		&num,
		(*C.wifi_ap_record_t)(unsafe.Pointer(&recs[0])),
	); code != C.ESP_OK {
		println("wifi_scan: esp_wifi_scan_get_ap_records err", int32(code))
		return nil, makeError(code)
	}
	println("wifi_scan: get_ap_records returned num=", int(num))

	aps := make([]AccessPoint, int(num))
	for i := 0; i < int(num); i++ {
		raw := C.GoBytes(unsafe.Pointer(&recs[i].ssid[0]), C.int(len(recs[i].ssid)))
		if idx := bytes.IndexByte(raw, 0); idx >= 0 {
			raw = raw[:idx]
		}
		println("wifi_scan: found AP", string(raw), "RSSI", int(recs[i].rssi))
		aps[i] = AccessPoint{
			SSID: string(raw),
			RSSI: int(recs[i].rssi),
		}
	}

	println("wifi_scan: done")
	return aps, nil
}

// ─── Connect ─────────────────────────────────────────────────────────────────

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

// ─── Soft-AP ─────────────────────────────────────────────────────────────────

// StartAP starts the radio in soft-AP mode with the given configuration.
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

// ─── RF diagnostics ─────────────────────────────────────────────────────────

func SniffCountOnChannel(channel uint8, duration time.Duration) (uint32, error) {
	if duration <= 0 {
		duration = 1500 * time.Millisecond
	}
	if code := C.espradio_sniff_begin(C.uint8_t(channel)); code != C.ESP_OK {
		return 0, makeError(code)
	}
	time.Sleep(duration)
	packets := uint32(C.espradio_sniff_count())
	if code := C.espradio_sniff_end(); code != C.ESP_OK {
		return packets, makeError(code)
	}
	return packets, nil
}

// ─── Tasks / timers / ISR ────────────────────────────────────────────────────

func millisecondsToTicks(ms uint32) uint32 {
	return ms * (ticksPerSecond / 1000)
}

func ticksToMilliseconds(ticks uint32) uint32 {
	return ticks / (ticksPerSecond / 1000)
}

//export espradio_panic
func espradio_panic(msg *C.char) {
	panic("espradio: " + C.GoString(msg))
}

//export espradio_log_timestamp
func espradio_log_timestamp() uint32 {
	return uint32(time.Now().UnixMilli())
}

//export espradio_run_task
func espradio_run_task(task_func, param unsafe.Pointer)

const taskStackSize = 8192

//export espradio_task_create_pinned_to_core
func espradio_task_create_pinned_to_core(task_func unsafe.Pointer, name *C.char, stack_depth uint32, param unsafe.Pointer, prio uint32, task_handle *unsafe.Pointer, core_id uint32) int32 {
	ch := make(chan struct{}, 1)
	go func() {
		var anchor byte
		top := uintptr(unsafe.Pointer(&anchor))
		bottom := top - taskStackSize
		C.espradio_set_task_stack_bottom(C.ulong(bottom))
		*task_handle = tinygo_task_current()
		close(ch)
		espradio_run_task(task_func, param)
	}()
	<-ch
	return 1
}

//export espradio_task_delete
func espradio_task_delete(task_handle unsafe.Pointer) {
}

//export tinygo_task_current
func tinygo_task_current() unsafe.Pointer

//export espradio_task_get_current_task
func espradio_task_get_current_task() unsafe.Pointer {
	return tinygo_task_current()
}

//export espradio_task_yield_go
func espradio_task_yield_go() {
	runtime.Gosched()
}

//export espradio_time_us_now
func espradio_time_us_now() uint64 {
	return uint64(time.Now().UnixMicro())
}

var (
	timerGenMu sync.Mutex
	timerGen   map[uintptr]uint32
)

func timerArmGeneration(timer unsafe.Pointer) uint32 {
	key := uintptr(timer)
	timerGenMu.Lock()
	defer timerGenMu.Unlock()
	if timerGen == nil {
		timerGen = make(map[uintptr]uint32)
	}
	g := timerGen[key] + 1
	timerGen[key] = g
	return g
}

func timerGenerationAlive(timer unsafe.Pointer, gen uint32) bool {
	key := uintptr(timer)
	timerGenMu.Lock()
	defer timerGenMu.Unlock()
	if timerGen == nil {
		return false
	}
	return timerGen[key] == gen
}

//export espradio_timer_cancel_go
func espradio_timer_cancel_go(timer unsafe.Pointer) {
	key := uintptr(timer)
	timerGenMu.Lock()
	if timerGen == nil {
		timerGen = make(map[uintptr]uint32)
	}
	timerGen[key] = timerGen[key] + 1
	timerGenMu.Unlock()
}

//export espradio_timer_arm_go
func espradio_timer_arm_go(timer unsafe.Pointer, tmout_ticks uint32, repeat int32) {
	ms := ticksToMilliseconds(tmout_ticks)
	if ms == 0 {
		ms = 1
	}
	gen := timerArmGeneration(timer)
	go func(gen uint32) {
		d := time.Duration(ms) * time.Millisecond
		if repeat != 0 {
			for {
				time.Sleep(d)
				if !timerGenerationAlive(timer, gen) {
					return
				}
				C.espradio_timer_fire(timer)
			}
		}
		time.Sleep(d)
		if !timerGenerationAlive(timer, gen) {
			return
		}
		C.espradio_timer_fire(timer)
	}(gen)
}

//export espradio_timer_arm_go_us
func espradio_timer_arm_go_us(timer unsafe.Pointer, us uint32, repeat int32) {
	if us == 0 {
		us = 1
	}
	gen := timerArmGeneration(timer)
	go func(gen uint32) {
		d := time.Duration(us) * time.Microsecond
		if repeat != 0 {
			for {
				time.Sleep(d)
				if !timerGenerationAlive(timer, gen) {
					return
				}
				C.espradio_timer_fire(timer)
			}
		}
		time.Sleep(d)
		if !timerGenerationAlive(timer, gen) {
			return
		}
		C.espradio_timer_fire(timer)
	}(gen)
}

//export espradio_task_delay
func espradio_task_delay(ticks uint32) {
	const ticksPerMillisecond = ticksPerSecond / 1000
	ms := (ticks + ticksPerMillisecond - 1) / ticksPerMillisecond
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

//export espradio_task_ms_to_tick
func espradio_task_ms_to_tick(ms uint32) int32 {
	return int32(millisecondsToTicks(ms))
}

//export espradio_wifi_int_disable
func espradio_wifi_int_disable(wifi_int_mux unsafe.Pointer) uint32 {
	return uint32(interrupt.Disable())
}

//export espradio_wifi_int_restore
func espradio_wifi_int_restore(wifi_int_mux unsafe.Pointer, tmp uint32) {
	interrupt.Restore(interrupt.State(tmp))
}

var wifiISR interrupt.Interrupt

func initWiFiISR() {
	wifiISR = interrupt.New(1, func(interrupt.Interrupt) {
		C.espradio_call_saved_isr(1)
	})
}

func enableWiFiISR() {
	wifiISR.Enable()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
