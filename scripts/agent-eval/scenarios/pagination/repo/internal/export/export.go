// Package export writes orders to CSV one page at a time.
package export

import (
	"encoding/csv"
	"io"
	"strconv"
)

// Order is one customer order.
type Order struct {
	ID    int
	Total int
}

// Page returns page number page (0-based) of size items.
func Page(orders []Order, page, size int) []Order {
	start := page * size
	if start >= len(orders) {
		return nil
	}
	end := start + size - 1
	if end > len(orders) {
		end = len(orders)
	}
	return orders[start:end]
}

// WriteCSV writes every order, page by page.
func WriteCSV(w io.Writer, orders []Order, size int) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "total"}); err != nil {
		return err
	}
	for p := 0; ; p++ {
		batch := Page(orders, p, size)
		if batch == nil {
			break
		}
		for _, o := range batch {
			if err := cw.Write([]string{strconv.Itoa(o.ID), strconv.Itoa(o.Total)}); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
