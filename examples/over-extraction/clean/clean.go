package orders

import "errors"

var ErrEmptyOrder = errors.New("order has no items")

type Order struct {
	Items []Item
	Total int
}

type Item struct {
	Price int
	Tags  []string
}

// totalPrice has real logic, so naming it is worthwhile.
func totalPrice(items []Item) int {
	total := 0
	for _, item := range items {
		total += item.Price
	}
	return total
}

func Validate(o Order) error {
	if len(o.Items) == 0 {
		return ErrEmptyOrder
	}
	if o.Total <= 0 {
		return errors.New("order total must be positive")
	}
	return nil
}

func ApplyDiscount(o Order) int {
	if o.Total < totalPrice(o.Items) {
		return o.Total / 2
	}
	return o.Total
}

func CountTagged(items []Item) int {
	count := 0
	for _, item := range items {
		if len(item.Tags) > 0 && item.Price > 0 {
			count++
		}
	}
	return count
}
