// +build linux,arm

package main

import(
	"arrase/raspiducky/hid"
)

func main() {
	hidCtl, err := hid.NewHIDController(context.Background(),"/dev/hidg0", "keymaps", "/dev/hidg1")
}
