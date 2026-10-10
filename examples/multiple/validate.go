package api

import "errors"

var errEmptyName = errors.New("empty name")

func Validate(name string) error {
	if name == "" {
		return errEmptyName
	}
	return nil
}
