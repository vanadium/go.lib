// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"

	"v.io/x/lib/netconfig"
	"v.io/x/lib/netconfig/osnetconfig"
)

func main() {
	sigch := make(chan os.Signal)
	signal.Notify(sigch, os.Interrupt, os.Kill)
	notifier := osnetconfig.NewNotifier(0)
	netconfig.SetOSNotifier(notifier)
	routes := notifier.GetIPRoutes(false)
	fmt.Printf("%d routes\n", len(routes))
	sort.SliceStable(routes, func(i, j int) bool {
		return routes[i].IfcIndex < routes[j].IfcIndex
	})
	for _, r := range routes {
		ps := ""
		if r.PreferredSource != nil {
			ps = r.PreferredSource.String() + " "
		}
		fmt.Printf("%v: %s %svia %s\n", r.IfcIndex, r.Net.String(), ps, r.Gateway.String())
	}
	for {
		ch, err := netconfig.NotifyChange()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start net config notifier: %v", err)
			os.Exit(1)
		}
		select {
		case <-ch:
		case sig := <-sigch:
			fmt.Fprintf(os.Stderr, "signal: %v\n", sig)
			os.Exit(1)
		}
		routes := netconfig.GetIPRoutes(true)
		for _, r := range routes {
			ps := ""
			if r.PreferredSource != nil {
				ps = r.PreferredSource.String() + " "
			}
			fmt.Printf("%v: %s %svia %s\n", r.IfcIndex, r.Net.String(), ps, r.Gateway.String())
		}
	}
	netconfig.Shutdown()
}
