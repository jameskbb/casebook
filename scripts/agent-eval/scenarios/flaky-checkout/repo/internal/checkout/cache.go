package checkout

// The cart cache keeps loaded carts in memory so repeated lookups during one
// checkout skip the cart store. Carts are keyed by cart id.
//
// Loading goes through LoadCart in production; tests seed the cache
// directly with cacheCart.

// LoadCart puts c in the cache, replacing any cart with the same id.
func LoadCart(c *Cart) { cacheCart(c) }

func cacheCart(c *Cart) {
	cartCache[c.ID] = c
}

// cartCache holds the loaded carts.
var cartCache = map[string]*Cart{}

func cachedCart(id string) (*Cart, bool) {
	c, ok := cartCache[id]
	return c, ok
}
