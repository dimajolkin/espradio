package main

import (
	"time"

	"tinygo.org/x/espradio"
)

func main() {
	time.Sleep(time.Second)

	println("ap: enabling radio...")
	if err := espradio.Enable(espradio.Config{Logging: espradio.LogLevelInfo}); err != nil {
		println("ap: enable err:", err)
		return
	}

	println("ap: starting AP...")
	err := espradio.StartAP(espradio.APConfig{
		SSID:     "espradio-ap",
		Password: "12345678",
		Channel:  6,
		AuthOpen: false,
	})
	if err != nil {
		println("ap: start err:", err)
		return
	}

	println("ap: AP is running, connect to espradio-ap")
	for {
		time.Sleep(10 * time.Second)
	}
}
