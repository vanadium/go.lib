// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simplemr_test

import (
	"fmt"

	"v.io/x/lib/simplemr"
)

func ExampleMR() {
	in, out := make(chan *simplemr.Record, 2), make(chan *simplemr.Record, 2)
	mr := &simplemr.MR{}
	identity := &simplemr.Identity{}
	go mr.Run(in, out, identity, identity)
	in <- &simplemr.Record{"1", []interface{}{"hello\n"}}
	in <- &simplemr.Record{"2", []interface{}{"world\n"}}
	close(in)
	k := <-out
	fmt.Printf("%s: %s", k.Key, k.Values[0].(string))
	k = <-out
	fmt.Printf("%s: %s", k.Key, k.Values[0].(string))
	if err := mr.Error(); err != nil {
		fmt.Printf("mr failed: %v", err)
	}
	// Output:
	// 1: hello
	// 2: world
}
