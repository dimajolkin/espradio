package main

import (
	"time"

	"tinygo.org/x/espradio"
)

func main() {
	time.Sleep(time.Second)

	println("rf_rx_test: enable")
	if err := espradio.Enable(espradio.Config{Logging: espradio.LogLevelInfo}); err != nil {
		println("rf_rx_test: enable err:", err)
		return
	}

	println("rf_rx_test: start")
	if err := espradio.Start(); err != nil {
		println("rf_rx_test: start err:", err)
		return
	}

	channels := []uint8{1, 6, 11}
	total := uint32(0)
	for _, ch := range channels {
		pkts, err := espradio.SniffCountOnChannel(ch, 1500*time.Millisecond)
		if err != nil {
			println("rf_rx_test: sniff ch", int(ch), "err:", err)
			return
		}
		total += pkts
		println("rf_rx_test: sniff ch", int(ch), "pkts", int(pkts))
	}

	println("rf_rx_test: scan")
	aps, err := espradio.Scan()
	if err != nil {
		println("rf_rx_test: scan err:", err)
		return
	}
	println("rf_rx_test: result promisc_total", int(total), "ap_num", len(aps))
	for _, ap := range aps {
		println("rf_rx_test: AP", ap.SSID, "RSSI", ap.RSSI)
	}

	for {
		time.Sleep(5 * time.Second)
	}
}
