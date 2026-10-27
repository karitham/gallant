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

func isEmpty(items []Item) bool { return len(items) == 0 }
func isPositive(n int) bool     { return n > 0 }
func hasTags(item Item) bool    { return len(item.Tags) > 0 }
func isFree(item Item) bool     { return item.Price == 0 }
func isDiscounted(o Order) bool { return o.Total < sum(o.Items) }

func sum(items []Item) int {
	total := 0
	for _, item := range items {
		total += item.Price
	}
	return total
}

func Validate(o Order) error {
	if isEmpty(o.Items) {
		return ErrEmptyOrder
	}
	if !isPositive(o.Total) {
		return errors.New("order total must be positive")
	}
	return nil
}

func ApplyDiscount(o Order) int {
	if isDiscounted(o) {
		return o.Total / 2
	}
	return o.Total
}

func CountTagged(items []Item) int {
	count := 0
	for _, item := range items {
		if hasTags(item) && !isFree(item) {
			count++
		}
	}
	return count
}
