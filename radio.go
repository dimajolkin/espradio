package espradio

/*
#cgo CFLAGS: -Iblobs/include
#cgo CFLAGS: -Iblobs/include/esp32c3
#cgo CFLAGS: -Iblobs/include/local
#cgo CFLAGS: -Iblobs/headers
#cgo CFLAGS: -DCONFIG_SOC_WIFI_NAN_SUPPORT=0
#cgo CFLAGS: -DESPRADIO_PHY_PATCH_ROMFUNCS=0
#cgo LDFLAGS: -Lblobs/libs/esp32c3 -lcoexist -lcore -lmesh -lnet80211 -lespnow -lregulatory -lphy -lpp -lwpa_supplicant

#include "espradio.h"
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

var isrKick chan struct{}

func startSchedTicker() {
	isrKick = make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(schedTickerMs * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-isrKick:
			}
			schedOnce()
		}
	}()
}

func schedOnce() {
	for C.espradio_isr_ring_tail() != C.espradio_isr_ring_head() {
		idx := C.espradio_isr_ring_tail()
		q := C.espradio_isr_ring_entry_queue(idx)
		itemPtr := C.espradio_isr_ring_entry_item(idx)
		C.espradio_queue_send(q, itemPtr, 0)
		C.espradio_isr_ring_advance_tail()
	}

	for i := 0; i < 4; i++ {
		C.espradio_event_loop_run_once()
	}
	for i := 0; i < 4; i++ {
		if C.espradio_timer_poll_due(8) == 0 {
			break
		}
	}
	for i := 0; i < 4; i++ {
		if C.espradio_esp_timer_poll_due(8) == 0 {
			break
		}
	}
}

func kickSched() {
	select {
	case isrKick <- struct{}{}:
	default:
	}
}

// Enable and configure the radio.
func Enable(config Config) error {
	startSchedTicker()
	time.Sleep(schedTickerMs * time.Millisecond)
	initHardware()
	C.espradio_ensure_osi_ptr()

	wifiISR = interrupt.New(1, func(interrupt.Interrupt) {
		C.espradio_call_saved_isr(1)
		kickSched()
	})
	wifiISR.Enable()

	C.espradio_event_register_default_cb()
	C.espradio_set_blob_log_level(C.uint32_t(config.Logging))

	mask := interrupt.Disable()
	C.espradio_hal_init_clocks_go()
	interrupt.Restore(mask)

	errCode := C.espradio_wifi_init()
	if errCode != 0 {
		return makeError(errCode)
	}
	C.espradio_wifi_init_completed()

	return nil
}

// ─── Start / Scan ────────────────────────────────────────────────────────────

func Start() error {
	var mode C.wifi_mode_t
	if code := C.esp_wifi_get_mode(&mode); code != C.ESP_OK {
		return makeError(code)
	}
	if mode != C.WIFI_MODE_STA {
		if code := C.esp_wifi_set_mode(C.WIFI_MODE_STA); code != C.ESP_OK {
			return makeError(code)
		}
	}

	if code := C.espradio_esp_wifi_start(); code != C.ESP_OK {
		return makeError(code)
	}

	return nil
}

// Scan performs a single Wi-Fi scan pass and returns the list of discovered access points.
func Scan() ([]AccessPoint, error) {
	C.espradio_ensure_osi_ptr()
	C.esp_wifi_set_ps(C.WIFI_PS_NONE)
	C.espradio_set_country_eu_manual()

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
	if code := C.esp_wifi_scan_start(&scanCfg, true); code != C.ESP_OK {
		return nil, makeError(code)
	}

	var num C.uint16_t
	if code := C.esp_wifi_scan_get_ap_num(&num); code != C.ESP_OK {
		return nil, makeError(code)
	}
	if num == 0 {
		return nil, nil
	}

	recs := make([]C.wifi_ap_record_t, int(num))
	if code := C.esp_wifi_scan_get_ap_records(
		&num,
		(*C.wifi_ap_record_t)(unsafe.Pointer(&recs[0])),
	); code != C.ESP_OK {
		return nil, makeError(code)
	}

	aps := make([]AccessPoint, int(num))
	for i := 0; i < int(num); i++ {
		raw := C.GoBytes(unsafe.Pointer(&recs[i].ssid[0]), C.int(len(recs[i].ssid)))
		if idx := bytes.IndexByte(raw, 0); idx >= 0 {
			raw = raw[:idx]
		}
		aps[i] = AccessPoint{
			SSID: string(raw),
			RSSI: int(recs[i].rssi),
		}
	}

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
		return makeError(code)
	}

	if code := C.esp_wifi_connect_internal(); code != C.ESP_OK {
		return makeError(code)
	}

	select {
	case res := <-connectResult:
		if res.Connected {
			return nil
		}
		return makeError(C.esp_err_t(res.Reason))
	case <-time.After(15 * time.Second):
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
	}
}

// ─── Soft-AP ─────────────────────────────────────────────────────────────────

// StartAP starts the radio in soft-AP mode with the given configuration.
func StartAP(cfg APConfig) error {
	if code := C.esp_wifi_set_mode(C.WIFI_MODE_AP); code != C.ESP_OK {
		return makeError(code)
	}

	ssid := cfg.SSID
	if len(ssid) == 0 {
		ssid = "espradio-ap"
	}
	code := C.espradio_ap_set_config(
		C.CString(ssid), C.int(len(ssid)),
		C.CString(cfg.Password), C.int(len(cfg.Password)),
		C.uint8_t(cfg.Channel), C.int(boolToInt(cfg.AuthOpen)),
	)
	if code != C.ESP_OK {
		return makeError(code)
	}

	if code := C.espradio_esp_wifi_start(); code != C.ESP_OK {
		return makeError(code)
	}
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
		C.espradio_set_task_stack_top(C.ulong(top))
		println("wifi_task: stack top=", top, "bottom=", bottom, "size=", taskStackSize)
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

func safeGosched() {
	if wifiIntsOff > 0 {
		return
	}
	runtime.Gosched()
}

//export espradio_task_yield_go
func espradio_task_yield_go() {
	for i := 0; i < 4; i++ {
		if C.espradio_timer_poll_due(8) == 0 {
			break
		}
	}
	for i := 0; i < 4; i++ {
		if C.espradio_esp_timer_poll_due(8) == 0 {
			break
		}
	}
	kickSched()
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

var wifiIntsOff uint32

//export espradio_wifi_int_disable
func espradio_wifi_int_disable(wifi_int_mux unsafe.Pointer) uint32 {
	s := uint32(interrupt.Disable())
	wifiIntsOff++
	return s
}

//export espradio_wifi_int_restore
func espradio_wifi_int_restore(wifi_int_mux unsafe.Pointer, tmp uint32) {
	if wifiIntsOff > 0 {
		wifiIntsOff--
	}
	interrupt.Restore(interrupt.State(tmp))
}

var wifiISR interrupt.Interrupt

// ─── Helpers ─────────────────────────────────────────────────────────────────

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
