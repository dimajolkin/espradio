// Minimal ROM function declarations for ESP32-C3, extracted from ESP-IDF.
// Only the small subset actually used by espradio is declared here.

#pragma once

#include <stdint.h>

// Interrupt controller / ISR helpers.
void intr_matrix_set(uint32_t cpu_no, uint32_t model_num, uint32_t intr_num);
void ets_isr_attach(uint32_t intr_num, void (*fn)(void *), void *arg);
void ets_isr_mask(uint32_t mask);
void ets_isr_unmask(uint32_t mask);

// Global ROM lock / printf hooks.
void ets_install_uart_printf(void);
void ets_install_lock(void (*lock)(void), void (*unlock)(void));
void ets_intr_lock(void);
void ets_intr_unlock(void);

