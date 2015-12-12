// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lib

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"v.io/x/lib/gosh"
)

const helloWorld = "Hello, world!"

// HelloWorldMain is used to demonstrate usage of Shell.Main.
var HelloWorldMain = gosh.Register("HelloWorldMain", func() {
	fmt.Println(helloWorld)
})

func Get(addr string) {
	resp, err := http.Get("http://" + addr)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	fmt.Print(string(body))
}

// Copied from http://golang.org/src/net/http/server.go.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func Serve() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, helloWorld)
	})
	// Note: With http.ListenAndServe() there's no easy way to tell which port
	// number we were assigned, so instead we use net.Listen() followed by
	// http.Server.Serve().
	srv := &http.Server{Addr: "localhost:0"}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		panic(err)
	}
	gosh.SendVars(map[string]string{"Addr": ln.Addr().String()})
	go func() {
		time.Sleep(100 * time.Millisecond)
		gosh.SendReady()
	}()
	if err = srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)}); err != nil {
		panic(err)
	}
}
