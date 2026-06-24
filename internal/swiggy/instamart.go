package swiggy

import "context"

func (c *Client) SearchProducts(ctx context.Context, addressID, query string, offset int) ([]Product, error) {
	return decodeResult[[]Product](c.CallTool(ctx, "search_products", map[string]any{
		"addressId": addressID, "query": query, "offset": offset,
	}))
}

func (c *Client) YourGoToItems(ctx context.Context, addressID string, offset int) ([]Product, error) {
	return decodeResult[[]Product](c.CallTool(ctx, "your_go_to_items", map[string]any{
		"addressId": addressID, "offset": offset,
	}))
}

func (c *Client) GetCart(ctx context.Context) (Cart, error) {
	return decodeResult[Cart](c.CallTool(ctx, "get_cart", nil))
}

func (c *Client) UpdateCart(ctx context.Context, addressID string, items []CartItem) (Cart, error) {
	return decodeResult[Cart](c.CallTool(ctx, "update_cart", map[string]any{
		"selectedAddressId": addressID, "items": items,
	}))
}

func (c *Client) ClearCart(ctx context.Context) error {
	_, err := c.CallTool(ctx, "clear_cart", nil)
	return err
}

func (c *Client) GetOrders(ctx context.Context, count int, activeOnly bool) ([]Order, error) {
	return decodeResult[[]Order](c.CallTool(ctx, "get_orders", map[string]any{
		"count": count, "activeOnly": activeOnly,
	}))
}
