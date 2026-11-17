package checkout

import "testing"

func TestCheckoutTotals(t *testing.T) {
	t.Parallel()
	seedCart("cart-1", 3)
	pricingRoundTrip()
	lines, cents, err := Totals("cart-1")
	if err != nil {
		t.Fatal(err)
	}
	if lines != 3 {
		t.Fatalf("expected 3 line items, got %d", lines)
	}
	if cents != 3000 {
		t.Fatalf("expected 3000 cents, got %d", cents)
	}
}
