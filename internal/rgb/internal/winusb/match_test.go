package winusb

import "testing"

func TestMatchIDsKeepsDevicesApart(t *testing.T) {
	frame := `\?\usb#vid_0416&pid_8050#6&1a2b3c#{1d4b2365-4749-48ea-b38a-7c6fdddd7e26}`
	display := `\?\usb#vid_1cbe&pid_a088#7&0f0f0f#{1d4b2365-4749-48ea-b38a-7c6fdddd7e26}`
	if !matchIDs(frame, 0x0416, 0x8050) || matchIDs(display, 0x0416, 0x8050) {
		t.Fatal("the frame filter must match the frame only")
	}
	if matchIDs(frame, 0x1CBE, 0xA088) {
		t.Fatal("the display filter matched the frame")
	}
}
