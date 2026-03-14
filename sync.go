package espradio

/*
#include "include.h"
unsigned long espradio_stack_remaining(void);
int espradio_fire_one_pending_timer(void);
void espradio_ensure_osi_ptr(void);
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// debugOSI enables println in OSI callbacks (queue, mutex, semaphore, event_group).
// Set to true for tracing; may increase stack usage in WiFi task.
const debugOSI = true

// Various functions related to locks, mutexes, semaphores, and queues.
//
// WiFi task queue: driver messages in the queue are 8 bytes [cmd, p1..p7].
// cmd is the internal API command type (not documented in public IDF headers).
// Observed during init: 6 (right after wifi task start), 15 (right after semaphore creation).
// Implementation lives in binary blobs; we only log cmd and do not block waiting for a reply.

func wifiCmdString(cmd byte) string {
	switch cmd {
	case 6:
		return "6 (wifi_task init step / set_log?)"
	case 15:
		return "15 (wifi_task init step)"
	default:
		return fmt.Sprintf("%d", cmd)
	}
}

// Use a single fake spinlock. This is also how the Rust port does it.
var fakeSpinLock uint8

//export espradio_spin_lock_create
func espradio_spin_lock_create() unsafe.Pointer {
	if debugOSI {
		println("osi: spin_lock_create")
	}
	return unsafe.Pointer(&fakeSpinLock)
}

//export espradio_spin_lock_delete
func espradio_spin_lock_delete(lock unsafe.Pointer) {
	if debugOSI {
		println("osi: spin_lock_delete")
	}
}

// Use a small pool of recursive mutexes.
type recursiveMutex struct {
	state sync.Mutex
	owner unsafe.Pointer
	count uint32
}

var mutexes [8]recursiveMutex
var mutexInUse [8]uint32

//export espradio_recursive_mutex_create
func espradio_recursive_mutex_create() unsafe.Pointer {
	if debugOSI {
		println("osi: recursive_mutex_create")
	}
	for i := range mutexes {
		if atomic.CompareAndSwapUint32(&mutexInUse[i], 0, 1) {
			return unsafe.Pointer(&mutexes[i])
		}
	}
	panic("espradio: too many mutexes")
}

//export espradio_mutex_delete
func espradio_mutex_delete(cmut unsafe.Pointer) {
	if debugOSI {
		println("osi: mutex_delete")
	}
	mut := (*recursiveMutex)(cmut)
	mut.state.Lock()
	mut.owner = nil
	mut.count = 0
	mut.state.Unlock()
	for i := range mutexes {
		if mut == &mutexes[i] {
			atomic.StoreUint32(&mutexInUse[i], 0)
			return
		}
	}
}

//export espradio_mutex_lock
func espradio_mutex_lock(cmut unsafe.Pointer) int32 {
	if debugOSI {
		println("osi: mutex_lock", cmut)
	}
	mut := (*recursiveMutex)(cmut)
	me := tinygo_task_current()
	waitSpins := uint32(0)
	for {
		mut.state.Lock()
		if mut.count == 0 || mut.owner == me {
			mut.owner = me
			mut.count++
			mut.state.Unlock()
			return 1
		}
		waitSpins++
		if debugOSI && (waitSpins&0x0f) == 0 {
			println("osi: mutex_lock waiting", cmut, "owner=", mut.owner, "me=", me, "count=", mut.count, "spins=", waitSpins)
		}
		mut.state.Unlock()
		runtime.Gosched()
	}
}

//export espradio_mutex_unlock
func espradio_mutex_unlock(cmut unsafe.Pointer) int32 {
	if debugOSI {
		println("osi: mutex_unlock", cmut)
	}
	mut := (*recursiveMutex)(cmut)
	me := tinygo_task_current()
	mut.state.Lock()
	if mut.count > 0 && mut.owner == me {
		mut.count--
		if mut.count == 0 {
			mut.owner = nil
		}
		mut.state.Unlock()
		return 1
	}
	mut.state.Unlock()
	return 0
}

type semaphore struct {
	count uint32
}

var semaphores [4]semaphore
var semaphoreIndex uint32

var (
	wifiThreadSemMu   sync.Mutex
	wifiThreadSemByTH = map[unsafe.Pointer]*semaphore{}
	wifiThreadSemNil  semaphore
)

func wifiThreadSemOwner(semphr unsafe.Pointer) unsafe.Pointer {
	wifiThreadSemMu.Lock()
	defer wifiThreadSemMu.Unlock()
	for th, sem := range wifiThreadSemByTH {
		if unsafe.Pointer(sem) == semphr {
			return th
		}
	}
	return nil
}

func debugDumpCmd6(where string, cmd [8]byte) {
	if !debugOSI || cmd[0] != 6 {
		return
	}
	p := binary.LittleEndian.Uint32(cmd[4:8])
	println("osi:", where, "cmd6 ptr=", p)
	if p < 0x3fc00000 || p >= 0x40000000 {
		println("osi:", where, "cmd6 ptr out of DRAM range")
		return
	}
	base := uintptr(p)
	w0 := *(*uint32)(unsafe.Pointer(base + 0))
	w1 := *(*uint32)(unsafe.Pointer(base + 4))
	w2 := *(*uint32)(unsafe.Pointer(base + 8))
	w3 := *(*uint32)(unsafe.Pointer(base + 12))
	w4 := *(*uint32)(unsafe.Pointer(base + 16))
	w5 := *(*uint32)(unsafe.Pointer(base + 20))
	println("osi:", where, "cmd6 words=", w0, w1, w2, w3, w4, w5)
}

func debugDumpCmd0(where string, cmd [8]byte) {
	if !debugOSI || cmd[0] != 0 {
		return
	}
	println("osi:", where, "cmd0 bytes=", cmd[0], cmd[1], cmd[2], cmd[3], cmd[4], cmd[5], cmd[6], cmd[7])
	p := binary.LittleEndian.Uint32(cmd[4:8])
	println("osi:", where, "cmd0 ptr=", p)
	if p < 0x3fc00000 || p >= 0x40000000 {
		return
	}
	base := uintptr(p)
	w0 := *(*uint32)(unsafe.Pointer(base + 0))
	w1 := *(*uint32)(unsafe.Pointer(base + 4))
	w2 := *(*uint32)(unsafe.Pointer(base + 8))
	w3 := *(*uint32)(unsafe.Pointer(base + 12))
	println("osi:", where, "cmd0 words=", w0, w1, w2, w3)
}

func semTryTake(sem *semaphore) bool {
	for {
		cur := atomic.LoadUint32(&sem.count)
		if cur == 0 {
			return false
		}
		if atomic.CompareAndSwapUint32(&sem.count, cur, cur-1) {
			return true
		}
	}
}

//export espradio_semphr_create
func espradio_semphr_create(max, init uint32) unsafe.Pointer {
	i := atomic.AddUint32(&semaphoreIndex, 1) - 1
	if i >= uint32(len(semaphores)) {
		panic("espradio: too many semaphores")
	}
	semaphores[i] = semaphore{count: init}
	ptr := unsafe.Pointer(&semaphores[i])
	if debugOSI {
		println("espradio_semphr_create", max, init, "->", ptr)
	}
	return ptr
}

//export espradio_semphr_take
func espradio_semphr_take(semphr unsafe.Pointer, block_time_tick uint32) int32 {
	sem := (*semaphore)(semphr)
	owner := wifiThreadSemOwner(semphr)
	if debugOSI && owner != nil {
		println("osi: semphr_take wifi_thread_sem sem=", semphr, "owner_task=", owner, "caller_task=", tinygo_task_current())
	}
	if block_time_tick == 0 {
		if semTryTake(sem) {
			if debugOSI {
				println("osi: semphr_take nonblock got sem=", semphr, "count=", atomic.LoadUint32(&sem.count))
			}
			return 1
		}
		if debugOSI {
			println("osi: semphr_take nonblock miss sem=", semphr, "count=", atomic.LoadUint32(&sem.count))
		}
		return 0
	}

	forever := block_time_tick == C.OSI_FUNCS_TIME_BLOCKING
	start := time.Now()
	var timeout time.Duration
	if !forever {
		timeout = time.Duration(block_time_tick) * time.Millisecond
	}

	if debugOSI {
		println("osi: semphr_take blocking sem=", semphr, "count=", atomic.LoadUint32(&sem.count), "task=", tinygo_task_current())
	}

	waitSpins := uint32(0)
	for {
		if semTryTake(sem) {
			if debugOSI {
				println("osi: semphr_take got it sem=", semphr, "count=", atomic.LoadUint32(&sem.count), "task=", tinygo_task_current())
			}
			return 1
		}
		waitSpins++
		if debugOSI && (waitSpins&0x0f) == 0 {
			println("osi: semphr_take waiting sem=", semphr, "count=", atomic.LoadUint32(&sem.count), "spins=", waitSpins, "task=", tinygo_task_current())
		}
		if !forever && time.Since(start) >= timeout {
			if debugOSI {
				println("osi: semphr_take timeout sem=", semphr, "spins=", waitSpins, "task=", tinygo_task_current())
			}
			return 0
		}
		runtime.Gosched()
	}
}

//export espradio_semphr_give
func espradio_semphr_give(semphr unsafe.Pointer) int32 {
	sem := (*semaphore)(semphr)
	owner := wifiThreadSemOwner(semphr)
	if debugOSI && owner != nil {
		println("osi: semphr_give wifi_thread_sem sem=", semphr, "owner_task=", owner, "caller_task=", tinygo_task_current())
	}
	if debugOSI {
		println("osi: semphr_give sem=", semphr, "count_before=", atomic.LoadUint32(&sem.count))
	}
	atomic.AddUint32(&sem.count, 1)
	if debugOSI {
		println("osi: semphr_give done sem=", semphr, "count_after=", atomic.LoadUint32(&sem.count))
	}
	return 1
}

//export espradio_semphr_delete
func espradio_semphr_delete(semphr unsafe.Pointer) {
	if debugOSI {
		println("osi: semphr_delete")
	}
	sem := (*semaphore)(semphr)
	atomic.StoreUint32(&sem.count, 0)
}

//export espradio_wifi_thread_semphr_get
func espradio_wifi_thread_semphr_get() unsafe.Pointer {
	task := tinygo_task_current()
	wifiThreadSemMu.Lock()
	defer wifiThreadSemMu.Unlock()
	if task == nil {
		return unsafe.Pointer(&wifiThreadSemNil)
	}
	sem := wifiThreadSemByTH[task]
	if sem == nil {
		sem = &semaphore{}
		wifiThreadSemByTH[task] = sem
	}
	return unsafe.Pointer(sem)
}

type queue struct {
	mu      sync.Mutex
	storage [][8]byte
	read    int
	write   int
	count   int
}

func newQueue(capacity int) *queue {
	return &queue{
		storage: make([][8]byte, capacity),
	}
}

func (q *queue) enqueue(cmd [8]byte) int32 {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.count == len(q.storage) {
		return 0
	}
	q.storage[q.write] = cmd
	q.write++
	if q.write == len(q.storage) {
		q.write = 0
	}
	q.count++
	return 1
}

func (q *queue) dequeue(out *[8]byte) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.count == 0 {
		return false
	}
	*out = q.storage[q.read]
	q.read++
	if q.read == len(q.storage) {
		q.read = 0
	}
	q.count--
	return true
}

func (q *queue) length() uint32 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return uint32(q.count)
}

var (
	wifiQueueObj    *queue
	wifiQueuePtr    unsafe.Pointer
	wifiQueueHandle unsafe.Pointer
	queueMapMu      sync.Mutex
	queueByAlias    = map[unsafe.Pointer]*queue{}
	queueByHandle   = map[unsafe.Pointer]*queue{}
)

func registerQueue(handle unsafe.Pointer, alias unsafe.Pointer, q *queue) {
	queueMapMu.Lock()
	queueByHandle[handle] = q
	if alias != nil {
		queueByAlias[alias] = q
	}
	queueMapMu.Unlock()
}

func unregisterQueue(handle unsafe.Pointer, alias unsafe.Pointer) {
	queueMapMu.Lock()
	delete(queueByHandle, handle)
	if alias != nil {
		delete(queueByAlias, alias)
	}
	queueMapMu.Unlock()
}

func kickTimerWorker() {
	// Timers are polled from the single scheduler goroutine in startSchedTicker.
}

//export espradio_generic_queue_create
func espradio_generic_queue_create(queue_len, item_size int) unsafe.Pointer {
	if item_size != 8 {
		println("espradio: queue_create item_size=", item_size, "forcing 8")
		item_size = 8
	}
	if queue_len < 1 {
		queue_len = 1
	}
	q := newQueue(queue_len)
	handle := unsafe.Pointer(q)
	ptr := unsafe.Pointer(&handle)
	registerQueue(handle, ptr, q)
	if debugOSI {
		println("espradio_generic_queue_create len=", queue_len, "size=", item_size, "ptr=", ptr)
	}
	return ptr
}

//export espradio_generic_queue_delete
func espradio_generic_queue_delete(ptr unsafe.Pointer) {
	if debugOSI {
		println("espradio_generic_queue_delete", ptr)
	}
	if ptr != nil {
		unregisterQueue(ptr, nil)
	}
}

//export espradio_wifi_create_queue
func espradio_wifi_create_queue(queue_len, item_size int) unsafe.Pointer {
	if item_size != 8 {
		panic("espradio: unexpected queue item_size")
	}
	if queue_len < 1 {
		queue_len = 1
	}
	wifiQueueObj = newQueue(queue_len)
	wifiQueueHandle = unsafe.Pointer(wifiQueueObj)
	wifiQueuePtr = unsafe.Pointer(&wifiQueueHandle)
	registerQueue(wifiQueueHandle, wifiQueuePtr, wifiQueueObj)
	if debugOSI {
		println("espradio_wifi_create_queue", queue_len, item_size, "wifiQueue", wifiQueuePtr)
	}
	return wifiQueuePtr
}

//export espradio_wifi_delete_queue
func espradio_wifi_delete_queue(ptr unsafe.Pointer) {
	if debugOSI {
		println("espradio_wifi_delete_queue")
	}
	unregisterQueue(wifiQueueHandle, wifiQueuePtr)
	if ptr != nil && ptr != wifiQueuePtr {
		unregisterQueue(ptr, nil)
	}
	wifiQueueObj = nil
	wifiQueueHandle = nil
	wifiQueuePtr = nil
}

func queueFromPtr(ptr unsafe.Pointer) *queue {
	queueMapMu.Lock()
	q := queueByAlias[ptr]
	if q == nil {
		q = queueByHandle[ptr]
	}
	queueMapMu.Unlock()
	if q != nil {
		return q
	}
	if ptr != nil {
		handle := *(*unsafe.Pointer)(ptr)
		queueMapMu.Lock()
		q = queueByHandle[handle]
		queueMapMu.Unlock()
		if q != nil {
			if debugOSI {
				println("osi: queueFromPtr deref ptr=", ptr, "handle=", handle, "q=", q)
			}
			return q
		}
	}
	if debugOSI {
		println("osi: queueFromPtr unknown ptr=", ptr)
	}
	return nil
}

//export espradio_yield_and_fire_pending_timers
func espradio_yield_and_fire_pending_timers() {
	// 1:1 with esp-wifi timer_compat: setfn should not execute callbacks.
}

//export espradio_queue_recv
func espradio_queue_recv(ptr unsafe.Pointer, item unsafe.Pointer, block_time_tick uint32) int32 {
	q := queueFromPtr(ptr)
	if q == nil {
		if debugOSI {
			println("osi: queue_recv nil queue ptr=", ptr)
		}
		return 0
	}
	forever := block_time_tick == C.OSI_FUNCS_TIME_BLOCKING
	start := time.Now()
	var timeout time.Duration
	if !forever {
		timeout = time.Duration(block_time_tick) * time.Millisecond
	}

	if debugOSI {
		println("osi: queue_recv in ptr=", ptr, "q=", q, "qlen=", q.length(), "stack_left=", C.espradio_stack_remaining(), "task=", tinygo_task_current())
	}

	var cmd [8]byte
	waitSpins := uint32(0)
	for {
		if q.dequeue(&cmd) {
			goto got
		}
		waitSpins++
		if debugOSI && (waitSpins&0xfffff) == 0 {
			println("osi: queue_recv waiting spins=", waitSpins, "qlen=", q.length())
		}
		kickTimerWorker()
		if !forever && time.Since(start) >= timeout {
			if debugOSI {
				println("osi: queue_recv timeout ptr=", ptr, "qlen=", q.length(), "task=", tinygo_task_current())
			}
			return 0
		}
		runtime.Gosched()
	}

got:
	println("osi: queue_recv got cmd=", cmd[0])
	debugDumpCmd6("queue_recv", cmd)
	debugDumpCmd0("queue_recv", cmd)
	if debugOSI && cmd[0] == 7 {
		println("osi: queue_recv cmd7 bytes=", cmd[0], cmd[1], cmd[2], cmd[3], cmd[4], cmd[5], cmd[6], cmd[7], "task=", tinygo_task_current())
	}
	if debugOSI {
		println("osi: queue_recv out cmd=", cmd[0], "stack_left=", C.espradio_stack_remaining(), "task=", tinygo_task_current())
	}
	if cmd[0] == 6 && binary.LittleEndian.Uint32(cmd[4:8]) == 0 {
		if debugOSI {
			println("osi: queue_recv type=6 ptr=0, dropping to avoid pc:nil")
		}
		return 0
	}
	*(*[8]byte)(item) = cmd
	C.espradio_ensure_osi_ptr()
	if debugOSI {
		println("osi: queue_recv qlen_after=", q.length())
	}
	return 1
}

//export espradio_queue_send
func espradio_queue_send(ptr unsafe.Pointer, item unsafe.Pointer, block_time_tick uint32) int32 {
	q := queueFromPtr(ptr)
	if q == nil {
		if debugOSI {
			println("osi: queue_send nil queue ptr=", ptr)
		}
		return 0
	}
	cmd := *(*[8]byte)(item)
	for i := 0; i < 100 && cmd[0] == 6 && binary.LittleEndian.Uint32(cmd[4:8]) == 0; i++ {
		runtime.Gosched()
		cmd = *(*[8]byte)(item)
	}
	if debugOSI {
		if cmd[0] == 6 {
			println("osi: queue_send type=6 ptr=", binary.LittleEndian.Uint32(cmd[4:8]))
		} else {
			println("osi: queue_send cmd=", cmd[0])
		}
	}
	debugDumpCmd6("queue_send", cmd)
	debugDumpCmd0("queue_send", cmd)

	_ = block_time_tick
	rc := q.enqueue(cmd)
	if debugOSI && cmd[0] == 7 {
		println("osi: queue_send cmd=7 ptr=", ptr, "q=", q, "rc=", rc, "qlen=", q.length(), "task=", tinygo_task_current(),
			"bytes=", cmd[0], cmd[1], cmd[2], cmd[3], cmd[4], cmd[5], cmd[6], cmd[7])
	}
	return rc
}

//export espradio_queue_len
func espradio_queue_len(ptr unsafe.Pointer) uint32 {
	q := queueFromPtr(ptr)
	if q == nil {
		return 0
	}
	return q.length()
}

type eventGroup struct {
	mu   sync.Mutex
	bits uint32
}

func newEventGroup() *eventGroup {
	return &eventGroup{}
}

//export espradio_event_group_create
func espradio_event_group_create() unsafe.Pointer {
	eg := newEventGroup()
	if debugOSI {
		println("espradio_event_group_create", eg)
	}
	return unsafe.Pointer(eg)
}

//export espradio_event_group_delete
func espradio_event_group_delete(ptr unsafe.Pointer) {
	if debugOSI {
		println("espradio_event_group_delete", ptr)
	}
	eg := (*eventGroup)(ptr)
	eg.mu.Lock()
	eg.bits = 0
	eg.mu.Unlock()
}

//export espradio_event_group_set_bits
func espradio_event_group_set_bits(ptr unsafe.Pointer, bits uint32) uint32 {
	eg := (*eventGroup)(ptr)
	eg.mu.Lock()
	eg.bits |= bits
	cur := eg.bits
	eg.mu.Unlock()
	if debugOSI {
		println("espradio_event_group_set_bits", ptr, "bits", bits, "->", cur)
	}
	return cur
}

//export espradio_event_group_clear_bits
func espradio_event_group_clear_bits(ptr unsafe.Pointer, bits uint32) uint32 {
	eg := (*eventGroup)(ptr)
	eg.mu.Lock()
	eg.bits &^= bits
	cur := eg.bits
	eg.mu.Unlock()
	if debugOSI {
		println("espradio_event_group_clear_bits", ptr, "bits", bits, "->", cur)
	}
	return cur
}

//export espradio_event_group_wait_bits
func espradio_event_group_wait_bits(ptr unsafe.Pointer, bitsToWaitFor uint32, clearOnExit int32, waitForAllBits int32, blockTimeTick uint32) uint32 {
	eg := (*eventGroup)(ptr)
	want := bitsToWaitFor
	const foreverFallback = 200 * time.Millisecond

	matches := func(bits uint32) bool {
		if waitForAllBits != 0 {
			return bits&want == want
		}
		return bits&want != 0
	}

	if debugOSI {
		eg.mu.Lock()
		cur := eg.bits
		eg.mu.Unlock()
		println("espradio_event_group_wait_bits enter", ptr, "want", want, "bits", cur, "block", blockTimeTick)
	}

	forever := blockTimeTick == C.OSI_FUNCS_TIME_BLOCKING
	start := time.Now()
	var timeout time.Duration
	if !forever {
		timeout = time.Duration(blockTimeTick) * time.Millisecond
	}

	var snapshot uint32
	for {
		eg.mu.Lock()
		snapshot = eg.bits
		ok := matches(snapshot)
		if ok {
			if clearOnExit != 0 {
				eg.bits &^= want
				if debugOSI {
					println("espradio_event_group_wait_bits clearOnExit", ptr, "clear", want, "->", eg.bits)
				}
			}
			eg.mu.Unlock()
			if debugOSI {
				println("espradio_event_group_wait_bits exit", ptr, "want", want, "got", snapshot)
			}
			// FreeRTOS-compatible: return bits before clearOnExit mutation.
			return snapshot
		}
		eg.mu.Unlock()
		println("osi: event_group_wait_bits waiting for bits", want, "bits", snapshot)

		if blockTimeTick == 0 || (!forever && time.Since(start) >= timeout) {
			if debugOSI {
				println("espradio_event_group_wait_bits exit", ptr, "want", want, "got", snapshot)
			}
			return snapshot
		}
		runtime.Gosched()
	}
}
