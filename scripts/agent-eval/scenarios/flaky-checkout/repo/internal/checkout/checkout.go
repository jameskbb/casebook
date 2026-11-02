// Package checkout prices carts at checkout.
package checkout

import (
	"errors"
	"time"
)

// LineItem is one product line in a cart.
type LineItem struct {
	SKU        string
	Qty        int
	PriceCents int
	AddedOn    time.Time
}

// Cart is a shopper's cart.
type Cart struct {
	ID    string
	Items []LineItem
}

// ErrNoCart is returned when the cart is not loaded.
var ErrNoCart = errors.New("checkout: cart not loaded")

// pricingDay is the start of the current pricing day in UTC. Items added
// before it are repriced elsewhere and skipped here.
func pricingDay() time.Time { return time.Now().UTC().Truncate(24 * time.Hour) }

// Totals returns the number of priced line items and the total in cents.
func Totals(cartID string) (lines int, cents int, err error) {
	c, ok := cachedCart(cartID)
	if !ok {
		return 0, 0, ErrNoCart
	}
	day := pricingDay()
	for _, it := range c.Items {
		if it.AddedOn.Before(day) {
			continue
		}
		lines++
		cents += it.Qty * it.PriceCents
	}
	return lines, cents, nil
}

// ApplyDiscount returns the cart total after a percentage discount.
func ApplyDiscount(cartID string, pct int) (int, error) {
	_, cents, err := Totals(cartID)
	if err != nil {
		return 0, err
	}
	return cents * (100 - pct) / 100, nil
}
