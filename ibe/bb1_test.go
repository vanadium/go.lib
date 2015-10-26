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

func TestHashVal(t *testing.T) {
	var (
		prefix0 = [1]byte{0x00}
		prefix1 = [1]byte{0x01}
		msg0    = []byte("message 0")
		msg1    = []byte("message 1")
	)

	// Hashes of distinct messages (with the same prefix) should be different
	if bytes.Equal(hashval(prefix0, msg0)[:], hashval(prefix0, msg1)[:]) {
		t.Errorf("Hashing two distinct values produced same output")
	}
	// Hashes of identical messages with different prefixes should be different
	if bytes.Equal(hashval(prefix0, msg0)[:], hashval(prefix1, msg1)[:]) {
		t.Errorf("Hashing two messages with different prefixes produced same output")
	}
}

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
	m := []byte("AThirtyTwoBytePieceOfTextThisIs!")
	overhead := master.Params().CiphertextOverhead()
	C := make([]byte, len(m)+overhead)
	C2 := make([]byte, len(m)+overhead)

	if err := master.Params().Encrypt(alice, m, C); err != nil {
		t.Fatal(err)
	}
	if err := master.Params().Encrypt(alice, m, C2); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(C, C2) {
		t.Errorf("Repeated encryptions of the identical plaintext should not produce identical ciphertext")
	}

	// Decrypt
	decrypt := func(sk PrivateKey) ([]byte, error) {
		ret := make([]byte, len(C)-overhead)
		if err := sk.Decrypt(C, ret); err != nil {
			return nil, err
		}
		return ret, nil
	}
	if decrypted, err := decrypt(aliceSK); err != nil || !bytes.Equal(decrypted, m) {
		t.Errorf("Got (%v, %v), want (%v, nil)", decrypted, err, m[:])
	}
	if decrypted, err := decrypt(aliceSK2); err != nil || !bytes.Equal(decrypted, m) {
		t.Errorf("Got (%v, %v), want (%v, nil)", decrypted, err, m[:])
	}
	if _, err := decrypt(bobSK); err == nil {
		t.Errorf("Decrypted message with a different PrivateKey")
	}
}

// Applying the Fujisaki-Okamoto transformation to the BB1 IBE
// scheme yields a CCA2-secure encryption scheme. Since CCA2-security
// implies non-malleability, we verify that a tampered ciphertext
// does not properly decrypt in this test case.
func TestBB1NonMalleability(t *testing.T) {
	master, err := SetupBB1()
	if err != nil {
		t.Fatal(err)
	}
	const alice = "alice"
	aliceSK, err := master.Extract(alice)
	if err != nil {
		t.Fatal(err)
	}

	m := []byte("01234567899876543210123456789012")
	overhead := master.Params().CiphertextOverhead()
	C := make([]byte, len(m)+overhead)

	if err := master.Params().Encrypt(alice, m, C); err != nil {
		t.Fatal(err)
	}

	out := make([]byte, len(C)-overhead)
	// Test that an untampered C can be decrypted successfully.
	if err := aliceSK.Decrypt(C, out); err != nil || !bytes.Equal(out, m) {
		t.Fatal(err)
	}
	// Test that a tampered C cannot be decrypted successfully.
	C[0] = C[0] ^ byte(1)
	if err := aliceSK.Decrypt(C, out); err == nil {
		t.Fatalf("successfully decrypted a tampered ciphetext: %v", err)
	}
}

var (
	bb1           Master
	bb1SK         PrivateKey
	benchmarkmlen = 64
	benchmarkm    = make([]byte, benchmarkmlen)
	bb1C          []byte
)

func TestBB1Marshaling(t *testing.T) {
	bb1P := bb1.Params()
	pbytes, err := MarshalParams(bb1P)
	if err != nil {
		t.Fatal(err)
	}
	skbytes, err := MarshalPrivateKey(bb1SK)
	if err != nil {
		t.Fatal(err)
	}
	m := []byte("01234567899876543210123456789012")
	overhead := bb1P.CiphertextOverhead()
	var (
		C1 = make([]byte, len(m)+overhead)
		C2 = make([]byte, len(m)+overhead)
		m1 = make([]byte, len(m))
		m2 = make([]byte, len(m))
	)
	// Encrypt with the original params, decrypt with the unmarshaled key.
	if err := bb1P.Encrypt("alice", m, C1); err != nil {
		t.Error(err)
	} else if sk, err := UnmarshalPrivateKey(bb1P, skbytes); err != nil {
		t.Error(err)
	} else if err := sk.Decrypt(C1, m2); err != nil {
		t.Error(err)
	} else if !bytes.Equal(m, m2) {
		t.Errorf("Got %q, want %q", m, m2)
	}
	// Encrypt with the unmarshaled params, decrypt with the original key.
	if p, err := UnmarshalParams(pbytes); err != nil {
		t.Error(err)
	} else if err := p.Encrypt("alice", m, C2); err != nil {
		t.Error(err)
	} else if err := bb1SK.Decrypt(C2, m1); err != nil {
		t.Error(err)
	} else if !bytes.Equal(m, m1) {
		t.Errorf("Got %q, want %q", m, m1)
	}

	// Truncation errors
	if _, err := UnmarshalParams(pbytes[:len(pbytes)-1]); err == nil {
		t.Errorf("UnmarshalParams succeeded on truncated input")
	}
	if _, err := UnmarshalPrivateKey(bb1P, skbytes[:len(skbytes)-1]); err == nil {
		t.Errorf("UnmarshalPrivateKey succeeded on truncated input")
	}
	// Extension errors
	if _, err := UnmarshalParams(append(pbytes, 0)); err == nil {
		t.Errorf("UnmarshalParams succeeded on extended input")
	}
	if _, err := UnmarshalPrivateKey(bb1P, append(skbytes, 0)); err == nil {
		t.Errorf("UnmarshalPrivateKey succeeded on extended input")
	}
	// Zero length (no valid header either)
	if _, err := UnmarshalParams(nil); err == nil {
		t.Errorf("UnmarshalParams succeeded on nil input")
	}
	if _, err := UnmarshalParams([]byte{}); err == nil {
		t.Errorf("UnmarshalParams succeeded on zero length input")
	}
	if _, err := UnmarshalPrivateKey(bb1P, nil); err == nil {
		t.Errorf("UnmarshalPrivateKey succeeded on nil input")
	}
	if _, err := UnmarshalPrivateKey(bb1P, []byte{}); err == nil {
		t.Errorf("UnmarshalPrivateKey succeeded on zero length input")
	}
}

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
	overhead := bb1.Params().CiphertextOverhead()
	bb1C = make([]byte, benchmarkmlen+overhead)
	if err := bb1.Params().Encrypt("alice", benchmarkm, bb1C); err != nil {
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
	C := make([]byte, benchmarkmlen+p.CiphertextOverhead())
	for i := 0; i < b.N; i++ {
		if err := p.Encrypt("alice", benchmarkm, C); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecryptBB(b *testing.B) {
	m := make([]byte, benchmarkmlen)
	for i := 0; i < b.N; i++ {
		if err := bb1SK.Decrypt(bb1C, m); err != nil {
			b.Fatal(err)
		}
	}
}
