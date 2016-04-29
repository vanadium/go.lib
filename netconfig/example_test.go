// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netconfig_test

import (
	"fmt"
	"log"

	"v.io/x/lib/netconfig"
)

func ExampleNetConfigWatcher() {
	for {
		ch, err := netconfig.NotifyChange()
		if err != nil {
			log.Fatalf("oops: %s", err)
		}
		fmt.Println("Do something to your network.")
		<-ch
		fmt.Println("Network configuration changed.")
	}
}
