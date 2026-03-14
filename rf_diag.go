package espradio

/*
#include "include.h"

static volatile uint32_t espradio_sniff_packets = 0;

static void espradio_promisc_rx_cb(void *buf, wifi_promiscuous_pkt_type_t type) {
	(void)buf;
	(void)type;
	espradio_sniff_packets++;
}

static esp_err_t espradio_sniff_begin(uint8_t channel) {
	wifi_promiscuous_filter_t filter;
	filter.filter_mask = WIFI_PROMIS_FILTER_MASK_MGMT | WIFI_PROMIS_FILTER_MASK_CTRL | WIFI_PROMIS_FILTER_MASK_DATA;
	espradio_sniff_packets = 0;
	esp_err_t rc = esp_wifi_set_promiscuous(false);
	(void)rc;
	rc = esp_wifi_set_channel(channel, WIFI_SECOND_CHAN_NONE);
	if (rc != ESP_OK) {
		return rc;
	}
	rc = esp_wifi_set_promiscuous_filter(&filter);
	if (rc != ESP_OK) {
		return rc;
	}
	rc = esp_wifi_set_promiscuous_rx_cb(espradio_promisc_rx_cb);
	if (rc != ESP_OK) {
		return rc;
	}
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

import "time"

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
