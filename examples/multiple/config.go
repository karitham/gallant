package api

import (
	"os"
	"strconv"
)

type Config struct {
	Port int
}

func LoadConfig(path string) Config {
	// read the file
	data, _ := os.ReadFile(path)

	// parse the port
	port, _ := strconv.Atoi(string(data))

	// return the config
	return Config{Port: port}
}
