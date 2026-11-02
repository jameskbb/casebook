package checkout

import "testing"

func TestCheckoutDiscounts(t *testing.T) {
	t.Parallel()
	loadPriceTable()
	seedCart("cart-1", 2)
	got, err := ApplyDiscount("cart-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1800 {
		t.Fatalf("expected 1800 cents after 10%% off, got %d", got)
	}
}
