package lianli

import "testing"

// CaseHUD drives the LED frame only: the display (1CBE:A088) is another
// device and must never be matched or opened by this backend.
func TestOnlyTheFrameIsTargeted(t *testing.T) {
	if vendorID != 0x0416 || productRing88 != 0x8050 {
		t.Fatalf("target changed: %04x:%04x", vendorID, productRing88)
	}
	if vendorID == 0x1CBE || productRing88 == 0xA088 {
		t.Fatal("the display must not be targeted")
	}
}
