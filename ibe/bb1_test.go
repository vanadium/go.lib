// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ibe

import (
	"bytes"
	"crypto/rand"
	"reflect"
	"testing"
)

func TestBB1Correctness(t *testing.T) {
	master, err := SetupBB1()
	if err != nil {
		t.Fatal(err)
	}
	const (
		alice = "alice"
		bob   = "bob"
	)
	// Extract
	aliceSK, err := master.Extract(alice)
	if err != nil {
		t.Fatal(err)
	}
	aliceSK2, err := master.Extract(alice)
	if err != nil {
		t.Fatal(err)
	} else if reflect.DeepEqual(aliceSK, aliceSK2) {
		t.Fatal("Two Extract operations yielded the same PrivateKey!")
	}
	bobSK, err := master.Extract(bob)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt
	var (
		m     Plaintext
		C, C2 Ciphertext
	)
	if n := copy(m[:], []byte("AThirtyTwoBytePieceOfTextThisIs!")); n != len(m) {
		t.Fatalf("Test string must be %d bytes, not %d", len(m), n)
	}
	if err := master.Params().Encrypt(alice, &m, &C); err != nil {
		t.Fatal(err)
	}
	if err := master.Params().Encrypt(alice, &m, &C2); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(C[:], C2[:]) {
		t.Errorf("Repeated encryptions of the identical plaintext should not produce identical ciphertext")
	}

	// Decrypt
	decrypt := func(sk PrivateKey) (*Plaintext, error) {
		var ret Plaintext
		if err := sk.Decrypt(&C, &ret); err != nil {
			return nil, err
		}
		return &ret, nil
	}
	if decrypted, err := decrypt(aliceSK); err != nil || !bytes.Equal(decrypted[:], m[:]) {
		t.Errorf("Got (%v, %v), want (%v, nil)", decrypted, err, m[:])
	}
	if decrypted, err := decrypt(aliceSK2); err != nil || !bytes.Equal(decrypted[:], m[:]) {
		t.Errorf("Got (%v, %v), want (%v, nil)", decrypted, err, m[:])
	}
	if decrypted, _ := decrypt(bobSK); bytes.Equal(decrypted[:], m[:]) {
		t.Errorf("Decrypted message with a different PrivateKey")
	}
}

var (
	bb1        Master
	bb1SK      PrivateKey
	benchmarkm Plaintext
	bb1C       Ciphertext
)

func init() {
	var err error
	if bb1, err = SetupBB1(); err != nil {
		panic(err)
	}
	if bb1SK, err = bb1.Extract("alice"); err != nil {
		panic(err)
	}
	if _, err := rand.Read(benchmarkm[:]); err != nil {
		panic(err)
	}
	if err := bb1.Params().Encrypt("alice", &benchmarkm, &bb1C); err != nil {
		panic(err)
	}
}

func BenchmarkExtractBB1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := bb1.Extract("alice"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncryptBB(b *testing.B) {
	p := bb1.Params()
	var C Ciphertext
	for i := 0; i < b.N; i++ {
		if err := p.Encrypt("alice", &benchmarkm, &C); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecryptBB(b *testing.B) {
	var m Plaintext
	for i := 0; i < b.N; i++ {
		if err := bb1SK.Decrypt(&bb1C, &m); err != nil {
			b.Fatal(err)
		}
	}
}
