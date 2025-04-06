package server

import "testing"

func TestParseRemoteAddr(t *testing.T) {
	for _, tc := range []struct {
		name     string
		addr     string
		expected string
	}{
		{
			name:     "parse IPv4 address without port",
			addr:     "192.168.0.1",
			expected: "192.168.0.1",
		},
		{
			name:     "parse IPv4 address with port",
			addr:     "192.168.0.1:8181",
			expected: "192.168.0.1",
		},
		{
			name:     "parse IPv6 address without port",
			addr:     "fd00:ec2::254",
			expected: "fd00:ec2::254",
		},
		{
			name:     "parse IPv6 address with port",
			addr:     "[fd00:ec2::254]:8181",
			expected: "fd00:ec2::254",
		},
		{
			name:     "parse invalid address",
			addr:     "abcd",
			expected: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRemoteAddr(tc.addr)
			if got != tc.expected {
				t.Errorf("parseRemoteAddr(%q) = %q; want %q", tc.addr, got, tc.expected)
			}
		})
	}
}
