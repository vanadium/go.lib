// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ibe

import (
	"bytes"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bn256"
)

var magicNumber = []byte{0x1b, 0xe0} // prefix that appears in the marshaled form: 2 bytes

type marshaledType byte

const (
	// types of encoded bytes, 1 byte
	typeBB1Params     marshaledType = 0
	typeBB1PrivateKey               = 1

	// Sizes excluding the magic number and type header.
	headerSize                 = 3
	marshaledBB1ParamsSize     = 2*marshaledG1Size + 2*marshaledG2Size + marshaledGTSize
	marshaledBB1PrivateKeySize = 2 * marshaledG2Size
)

func writeHeader(typ marshaledType) []byte {
	ret := make([]byte, headerSize)
	copy(ret, magicNumber)
	ret[len(magicNumber)] = byte(typ)
	return ret
}

// readHeader parses hdr and returns the message type and the remainder of the
// message, excluding the header.
func readHeader(hdr []byte) (marshaledType, []byte, error) {
	if len(hdr) < headerSize {
		return 0, nil, fmt.Errorf("header is too small")
	}
	if !bytes.Equal(hdr[0:len(magicNumber)], magicNumber) {
		return 0, nil, fmt.Errorf("invalid magic number")
	}
	return marshaledType(hdr[len(magicNumber)]), hdr[headerSize:], nil
}

// MarshalParams encodes p into a byte slice.
func MarshalParams(p Params) ([]byte, error) {
	switch p := p.(type) {
	case *bb1params:
		ret := make([]byte, 0, headerSize+marshaledBB1ParamsSize)
		// g and gHat are the generators, do not need to be marshaled.
		for _, field := range [][]byte{
			writeHeader(typeBB1Params),
			p.g1.Marshal(),
			p.h.Marshal(),
			p.g1Hat.Marshal(),
			p.hHat.Marshal(),
			p.v.Marshal(),
		} {
			ret = append(ret, field...)
		}
		return ret, nil
	default:
		return nil, fmt.Errorf("MarshalParams for %T for implemented yet", p)
	}
}

// UnmarshalParams parses an encoded Params object.
func UnmarshalParams(data []byte) (Params, error) {
	var typ marshaledType
	var err error
	if typ, data, err = readHeader(data); err != nil {
		return nil, err
	}
	advance := func(n int) []byte {
		ret := data[0:n]
		data = data[n:]
		return ret
	}
	switch typ {
	case typeBB1Params:
		if len(data) != marshaledBB1ParamsSize {
			return nil, fmt.Errorf("invalid size")
		}
		p := &bb1params{v: new(bn256.GT)}
		one := big.NewInt(1)
		p.g.ScalarBaseMult(one)
		p.gHat.ScalarBaseMult(one)
		if _, ok := p.g1.Unmarshal(advance(marshaledG1Size)); !ok {
			return nil, fmt.Errorf("failed to unmarshal g1")
		}
		if _, ok := p.h.Unmarshal(advance(marshaledG1Size)); !ok {
			return nil, fmt.Errorf("failed to unmarshal h")
		}
		if _, ok := p.g1Hat.Unmarshal(advance(marshaledG2Size)); !ok {
			return nil, fmt.Errorf("failed to unmarshal g1Hat")
		}
		if _, ok := p.hHat.Unmarshal(advance(marshaledG2Size)); !ok {
			return nil, fmt.Errorf("failed to unmarshal hHat")
		}
		if _, ok := p.v.Unmarshal(advance(marshaledGTSize)); !ok {
			return nil, fmt.Errorf("failed to unmarshal v")
		}
		return p, nil
	default:
		return nil, fmt.Errorf("unrecognized Params type (%d)", typ)
	}
}

// MarshalPrivateKey encodes the private component of k into a byte slice.
func MarshalPrivateKey(k PrivateKey) ([]byte, error) {
	switch k := k.(type) {
	case *bb1PrivateKey:
		ret := make([]byte, 0, headerSize+marshaledBB1PrivateKeySize)
		for _, field := range [][]byte{
			writeHeader(typeBB1PrivateKey),
			k.d0.Marshal(),
			k.d1.Marshal(),
		} {
			ret = append(ret, field...)
		}
		return ret, nil
	default:
		return nil, fmt.Errorf("MarshalPrivateKey for %T for implemented yet", k)
	}
}

// UnmarshalPrivateKey parses an encoded PrivateKey object.
func UnmarshalPrivateKey(params Params, data []byte) (PrivateKey, error) {
	var typ marshaledType
	var err error
	if typ, data, err = readHeader(data); err != nil {
		return nil, err
	}
	advance := func(n int) []byte {
		ret := data[0:n]
		data = data[n:]
		return ret
	}
	switch typ {
	case typeBB1PrivateKey:
		if len(data) != marshaledBB1PrivateKeySize {
			return nil, fmt.Errorf("invalid size")
		}
		k := new(bb1PrivateKey)
		if _, ok := k.d0.Unmarshal(advance(marshaledG2Size)); !ok {
			return nil, fmt.Errorf("failed to unmarshal d0")
		}
		if _, ok := k.d1.Unmarshal(advance(marshaledG2Size)); !ok {
			return nil, fmt.Errorf("failed to unmarshal d1")
		}
		if p, ok := params.(*bb1params); !ok {
			return nil, fmt.Errorf("params type %T incompatible with %T", params, k)
		} else {
			k.params = new(bb1params)
			*(k.params) = *p
		}
		return k, nil
	default:
		return nil, fmt.Errorf("unrecognized private key type (%d)", typ)
	}
}
