//go:build esp32c3

#include "include.h"
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>

#ifndef ESPRADIO_OSI_DEBUG
#define ESPRADIO_OSI_DEBUG 1
#endif

extern unsigned long espradio_stack_remaining(void);
extern void espradio_wdev_last_desc_reset_prepare(void);

int espradio_coex_init(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_init\n");
#endif
    return 0;
}

void espradio_coex_deinit(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_deinit\n");
#endif
}

int espradio_coex_enable(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_enable\n");
#endif
    return 0;
}

void espradio_coex_disable(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_disable\n");
#endif
}

extern void espradio_ensure_osi_ptr(void);

uint32_t espradio_coex_status_get(void) {
    espradio_ensure_osi_ptr();
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_status_get -> 0\n");
#endif
    return 0;
}

void espradio_coex_condition_set(uint32_t type, bool dissatisfy) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_condition_set type=%lu dissatisfy=%d\n",
           (unsigned long)type, (int)dissatisfy);
#endif
    (void)type;
    (void)dissatisfy;
}

int espradio_coex_wifi_request(uint32_t event, uint32_t latency, uint32_t duration) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_wifi_request event=%lu lat=%lu dur=%lu\n",
           (unsigned long)event, (unsigned long)latency, (unsigned long)duration);
#endif
    (void)event;
    (void)latency;
    (void)duration;
    return 0;
}

int espradio_coex_wifi_release(uint32_t event) {
    espradio_wdev_last_desc_reset_prepare();
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_wifi_release event=%lu stack_left=%lu\n",
           (unsigned long)event, (unsigned long)espradio_stack_remaining());
#endif
    (void)event;
    return 0;
}

int espradio_coex_wifi_channel_set(uint8_t primary, uint8_t secondary) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_wifi_channel_set primary=%u secondary=%u\n",
           (unsigned)primary, (unsigned)secondary);
#endif
    (void)primary;
    (void)secondary;
    return 0;
}

int espradio_coex_event_duration_get(uint32_t event, uint32_t *duration) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_event_duration_get event=%lu duration_ptr=%p\n",
           (unsigned long)event, (void *)duration);
#endif
    if (duration) {
        *duration = 0;
    }
    return 0;
}

int espradio_coex_pti_get(uint32_t event, uint8_t *pti) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_pti_get event=%lu pti_ptr=%p\n",
           (unsigned long)event, (void *)pti);
#endif
    if (pti) {
        *pti = 0;
    }
    return 0;
}

void espradio_coex_schm_status_bit_clear(uint32_t type, uint32_t status) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_status_bit_clear type=%lu status=%lu\n",
           (unsigned long)type, (unsigned long)status);
#endif
    (void)type;
    (void)status;
}

void espradio_coex_schm_status_bit_set(uint32_t type, uint32_t status) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_status_bit_set type=%lu status=%lu\n",
           (unsigned long)type, (unsigned long)status);
#endif
    (void)type;
    (void)status;
}

int espradio_coex_schm_interval_set(uint32_t interval) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_interval_set interval=%lu\n", (unsigned long)interval);
#endif
    (void)interval;
    return 0;
}

uint32_t espradio_coex_schm_interval_get(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_interval_get\n");
#endif
    return 0;
}

uint8_t espradio_coex_schm_curr_period_get(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_curr_period_get\n");
#endif
    return 0;
}

void *espradio_coex_schm_curr_phase_get(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_curr_phase_get\n");
#endif
    return NULL;
}

int espradio_coex_schm_process_restart(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_process_restart\n");
#endif
    return 0;
}

int espradio_coex_schm_register_cb(int type, int (*cb)(int)) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_register_cb type=%d cb=%p\n", type, (void *)cb);
#endif
    (void)type;
    (void)cb;
    return 0;
}

int espradio_coex_register_start_cb(int (*cb)(void)) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_register_start_cb cb=%p\n", (void *)cb);
#endif
    (void)cb;
    return 0;
}

int espradio_coex_schm_flexible_period_set(uint8_t period) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_flexible_period_set period=%u\n", (unsigned)period);
#endif
    (void)period;
    return 0;
}

uint8_t espradio_coex_schm_flexible_period_get(void) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_flexible_period_get\n");
#endif
    return 1;
}

void *espradio_coex_schm_get_phase_by_idx(int idx) {
#if ESPRADIO_OSI_DEBUG
    printf("osi: coex_schm_get_phase_by_idx idx=%d\n", idx);
#endif
    (void)idx;
    return NULL;
}
