// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines an implementation of the IBE interfaces using the
// Boneh-Boyen scheme. The paper defining this algorithm (see comments for
// SetupBB1) uses multiplicative groups while the bn256 package used in the
// implementation here defines an additive group. The comments follow the
// notation in the paper while the code uses the bn256 library. For example,
// g^i corresponds to G1.ScalarBaseMult(i).

package ibe

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bn256"
)

var errBadCiphertext = errors.New("invalid ciphertext")

const marshaledG1Size = 64

// Setup creates an ibe.Master based on the BB1 scheme described in "Efficient
// Selective Identity-Based Encryption Without Random Oracles" by Dan Boneh and
// Xavier Boyen (http://crypto.stanford.edu/~dabo/papers/bbibe.pdf).
//
// Specifically, Section 4.3 of the paper is implemented.
func SetupBB1() (Master, error) {
	var (
		m     = &bb1master{params: new(bb1params)}
		pk    = m.params   // shorthand
		g0Hat = &(m.g0Hat) // shorthand
	)

	// Set generators
	pk.g.ScalarBaseMult(big.NewInt(1))
	pk.gHat.ScalarBaseMult(big.NewInt(1))

	// Pick a random alpha and set g1 & g1Hat
	alpha, err := random()
	if err != nil {
		return nil, err
	}
	pk.g1.ScalarBaseMult(alpha)
	pk.g1Hat.ScalarBaseMult(alpha)

	// Pick a random delta and set h and hHat
	delta, err := random()
	if err != nil {
		return nil, err
	}
	pk.h.ScalarBaseMult(delta)
	pk.hHat.ScalarBaseMult(delta)

	// Pick a random beta and set g0Hat.
	beta, err := random()
	if err != nil {
		return nil, err
	}
	alphabeta := new(big.Int).Mul(alpha, beta)
	g0Hat.ScalarBaseMult(alphabeta.Mod(alphabeta, bn256.Order)) // g0Hat = gHat^*(alpha*beta)

	pk.v = bn256.Pair(&pk.g, g0Hat)
	return m, nil
}

type bb1master struct {
	params *bb1params // Public params
	g0Hat  bn256.G2   // Master key
}

func (m *bb1master) Extract(id string) (PrivateKey, error) {
	r, err := random()
	if err != nil {
		return nil, err
	}

	var (
		ret = &bb1PrivateKey{params: m.params}
		// A bunch of shorthands
		d0    = new(bn256.G2)
		g1Hat = &(m.params.g1Hat)
		g0Hat = &(m.g0Hat)
		hHat  = &(m.params.hHat)
		i     = id2bignum(id)
	)
	// ret.d0 = g0Hat * (g1Hat^i * hHat)^r
	d0.ScalarMult(g1Hat, i)
	d0.Add(d0, hHat)
	d0.ScalarMult(d0, r)
	ret.d0.Add(d0, g0Hat)
	ret.d1.ScalarBaseMult(r)
	return ret, nil
}

func (m *bb1master) Params() Params { return m.params }

type bb1params struct {
	g, g1, h          bn256.G1
	gHat, g1Hat, hHat bn256.G2
	v                 *bn256.GT
}

func (e *bb1params) Encrypt(id string, m *Plaintext, C *Ciphertext) error {
	s, err := random()
	if err != nil {
		return err
	}

	var (
		vs    bn256.GT
		tmpG1 bn256.G1
		// Ciphertext C = (A, B, C1)
		A  = C[0:len(m)]
		B  = C[len(m) : len(m)+marshaledG1Size]
		C1 = C[len(m)+marshaledG1Size:]
	)
	vs.ScalarMult(e.v, s)
	pad := sha256.Sum256(vs.Marshal())
	// A = m ⊕ H(v^s)
	for i := range m {
		A[i] = m[i] ^ pad[i]
	}
	// B = g^s
	if err := marshalG1(B, tmpG1.ScalarBaseMult(s)); err != nil {
		return err
	}
	// C1 = (g1^H(id) h)^s
	tmpG1.ScalarMult(&e.g1, id2bignum(id))
	tmpG1.Add(&tmpG1, &e.h)
	tmpG1.ScalarMult(&tmpG1, s)
	if err := marshalG1(C1, &tmpG1); err != nil {
		return err
	}
	return nil
}

type bb1PrivateKey struct {
	params *bb1params // public parameters
	d0, d1 bn256.G2
}

func (k *bb1PrivateKey) Decrypt(C *Ciphertext, m *Plaintext) error {
	var (
		A     = C[0:len(m)]
		B, C1 bn256.G1
	)
	if _, ok := B.Unmarshal(C[len(m) : len(m)+marshaledG1Size]); !ok {
		return errBadCiphertext
	}
	if _, ok := C1.Unmarshal(C[len(m)+marshaledG1Size:]); !ok {
		return errBadCiphertext
	}
	// M = A ⊕ H(e(B, d0)/e(C1,d1))
	var (
		numerator   = bn256.Pair(&B, &k.d0)
		denominator = bn256.Pair(&C1, &k.d1)
		hash        = sha256.Sum256(numerator.Add(numerator, denominator.Neg(denominator)).Marshal())
	)
	for i := range m {
		m[i] = A[i] ^ hash[i]
	}
	return nil
}

// random returns a positive integer in the range [1, bn256.Order)
// (denoted by Zp in http://crypto.stanford.edu/~dabo/papers/bbibe.pdf).
//
// The paper refers to random numbers drawn from Zp*. From a theoretical
// perspective, the uniform distribution over Zp and Zp* start within a
// statistical distance of 1/p (where p=bn256.Order is a ~256bit prime).  Thus,
// drawing uniformly from Zp is no different from Zp*.
func random() (*big.Int, error) {
	for {
		k, err := rand.Int(rand.Reader, bn256.Order)
		if err != nil {
			return nil, err
		}
		if k.Sign() > 0 {
			return k, nil
		}
	}
}

func id2bignum(id string) *big.Int {
	h := sha256.Sum256([]byte(id))
	k := new(big.Int).SetBytes(h[:])
	return k.Mod(k, bn256.Order)
}

// marshalG1 writes the marshaled form of g into dst.
func marshalG1(dst []byte, g *bn256.G1) error {
	src := g.Marshal()
	if len(src) != len(dst) {
		return fmt.Errorf("bn256.G1.Marshal returned a %d byte slice, expected %d: the BB1 IBE implementation is likely broken", len(src), len(dst))
	}
	copy(dst, src)
	return nil
}
