package checkout

import (
	"fmt"
	"math/rand"
	"time"
)

// seedCart loads a cart with n one-unit line items at 10.00 each, all added
// today, and returns it.
func seedCart(id string, n int) *Cart {
	day := time.Now().UTC().Truncate(24 * time.Hour)
	c := &Cart{ID: id}
	for i := 0; i < n; i++ {
		c.Items = append(c.Items, LineItem{SKU: fmt.Sprintf("sku-%d", i), Qty: 1, PriceCents: 1000, AddedOn: day})
	}
	cacheCart(c)
	return c
}

// pricingRoundTrip stands in for the call to the pricing service: usually
// about a millisecond, occasionally slower.
func pricingRoundTrip() {
	d := time.Millisecond
	if rand.Intn(20) == 0 {
		d = 5 * time.Millisecond
	}
	time.Sleep(d)
}

// loadPriceTable stands in for reading the discount price table.
func loadPriceTable() { time.Sleep(3 * time.Millisecond) }
