package swiggy

// These structs decode the fields console.store uses; unknown fields are
// ignored. They intentionally mirror catalog shapes so the catalog/swiggy
// adapter (a later slice) maps them with minimal translation.

type Address struct {
	ID    string  `json:"id"`
	Label string  `json:"annotation"`
	City  string  `json:"city"`
	Line  string  `json:"locality"`
	Full  string  `json:"address"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
}

type Restaurant struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	City   string  `json:"city"`
	ETA    string  `json:"eta"`
	Rating float64 `json:"rating"`
	Desc   string  `json:"description"`
}

type MenuItem struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Price  int     `json:"price"`
	Veg    bool    `json:"isVeg"`
	Desc   string  `json:"description"`
	Rating float64 `json:"rating"`
}

type Menu struct {
	RestaurantID string     `json:"restaurantId"`
	Items        []MenuItem `json:"items"`
}

type CartItem struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

type Cart struct {
	CartID string `json:"cartId"`
	Total  int    `json:"total"`
	Items  []struct {
		ItemID   string `json:"itemId"`
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		Price    int    `json:"price"`
	} `json:"items"`
}

type Coupon struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Amount      int    `json:"amount"`
}

type Order struct {
	ID         string `json:"orderId"`
	Status     string `json:"status"`
	Restaurant string `json:"restaurantName"`
	Total      int    `json:"total"`
	PlacedAt   string `json:"placedAt"`
}

type Tracking struct {
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
	ETA     string `json:"eta"`
}

type Product struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}
