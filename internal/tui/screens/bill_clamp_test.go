package screens

import "testing"

// billToPay must never return a negative amount due — a cheap cart whose coupon
// exceeds item+delivery clamps to 0 instead of showing "to pay ₹-20".
func TestBillToPayNeverNegative(t *testing.T) {
	if got := billToPay(10); got != 0 {
		t.Fatalf("sub-coupon cart must clamp to 0, got %d", got)
	}
	if got := billToPay(100); got != 100+DeliveryFee-CouponAmount {
		t.Fatalf("normal cart math must be unchanged, got %d", got)
	}
	if got := billToPay(0); got != 0 {
		t.Fatalf("empty cart is 0, got %d", got)
	}
}
