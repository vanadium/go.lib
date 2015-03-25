// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate

import (
	"net"
	"testing"
)

func TestIsGloballyRoutable(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", false},
		{"192.169.0.3", true},
		{"10.1.1.1", false},
		{"172.17.100.255", false},
		{"172.32.0.1", true},
		{"255.255.255.255", false},
		{"127.0.0.1", false},
		{"224.0.0.1", false},
		{"FF02::FB", false},
		{"fe80::be30:5bff:fed3:843f", false},
		{"2620:0:1000:8400:be30:5bff:fed3:843f", true},
	}
	for _, test := range tests {
		ip := net.ParseIP(test.ip)
		if got := IsGloballyRoutableIP(ip); got != test.want {
			t.Fatalf("%s: want %v got %v", test.ip, test.want, got)
		}
	}
}
