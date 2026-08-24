package main

import "testing"

func TestValidateAddressAllowsOnlyLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:19081", "localhost:19123", "[::1]:19999"} {
		if err := validateAddress(address); err != nil {
			t.Errorf("应允许 %s: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:19081", "192.168.1.2:19081", "127.0.0.1:0", "localhost:8080:extra"} {
		if err := validateAddress(address); err == nil {
			t.Errorf("应拒绝 %s", address)
		}
	}
}
