package main

import "os"

// Exists check whether the file exists or not
func Exists(f string) bool {
	if _, err := os.Stat(f); os.IsNotExist(err) {
		return false
	}
	return true
}
