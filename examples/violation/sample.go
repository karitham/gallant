package orders

import (
	"encoding/json"
	"os"
)

var data []byte

func handle(data []byte) error {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	process(result)
	return nil
}

func process(thing map[string]any) {
	for _, item := range thing {
		doStuff(item)
	}
}

func doStuff(x any) {
	if x == nil {
		return
	}
}

func load() error {
	file, err := os.Open("orders.json")
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	data = make([]byte, info.Size())
	_, err = file.Read(data)
	return err
}
