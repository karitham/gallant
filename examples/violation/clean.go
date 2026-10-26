package orders

import (
	"encoding/json"
	"os"
)

type Order struct {
	ID     string  `json:"id"`
	Total  float64 `json:"total"`
	Status string  `json:"status"`
}

func LoadOrder(path string) (Order, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Order{}, err
	}

	var order Order
	if err := json.Unmarshal(raw, &order); err != nil {
		return Order{}, err
	}
	return order, nil
}

func TotalPaid(orders []Order) float64 {
	var total float64
	for _, order := range orders {
		if order.Status == "paid" {
			total += order.Total
		}
	}
	return total
}
