//go:build esp32c3

package esp32c3

// #cgo CFLAGS: -I../blobs/include
// #cgo CFLAGS: -I../blobs/include/esp32c3
// #cgo CFLAGS: -I../blobs/include/local
// #cgo CFLAGS: -I../blobs/headers
// #cgo CFLAGS: -I..
// #cgo CFLAGS: -DCONFIG_SOC_WIFI_NAN_SUPPORT=0
// #cgo CFLAGS: -DESPRADIO_PHY_PATCH_ROMFUNCS=0
import "C"
