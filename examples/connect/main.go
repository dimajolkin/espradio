package main

import (
	"time"

	"tinygo.org/x/espradio"
)

const (
	wifiSSID = "Kracozabra"
	wifiPass = "09655455"
)

func main() {
	time.Sleep(time.Second)

	println("initializing radio...")
	err := espradio.Enable(espradio.Config{
		Logging: espradio.LogLevelVerbose,
	})
	if err != nil {
		println("could not enable radio:", err)
		return
	}

	println("starting radio...")
	err = espradio.Start()
	if err != nil {
		println("could not start radio:", err)
		return
	}

	println("connecting to", wifiSSID, "...")
	err = espradio.Connect(espradio.STAConfig{
		SSID:     wifiSSID,
		Password: wifiPass,
	})
	if err != nil {
		println("connect failed:", err)
	} else {
		println("connected to", wifiSSID, "!")
	}

	for {
		time.Sleep(5 * time.Second)
		println("alive")
	}
}
