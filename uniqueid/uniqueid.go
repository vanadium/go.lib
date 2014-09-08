// Package uniqueid helps generate identifiers that are likely to be
// globally unique.  We want to be able to generate many IDs quickly,
// so we make a time/space tradeoff.  We reuse the same random data
// many times with a counter appended.  Note: these IDs are NOT useful
// as a security mechanism as they will be predictable.
package uniqueid

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
)

type ID [16]byte

var random = RandomGenerator{}

// A RandomGenerator can generate random IDs.
// The zero value of RandomGenerator is ready to use.
type RandomGenerator struct {
	mu     sync.Mutex
	id     ID
	count  uint16
	resets int
}

// NewID produces a new probably unique identifier.
func (g *RandomGenerator) NewID() (ID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.count == 0 {
		// Either the generator is uninitialized or the counter
		// has wrapped.  We need a new random prefix.
		if _, err := rand.Read(g.id[:14]); err != nil {
			return ID{}, err
		}
		g.resets++
	}
	binary.BigEndian.PutUint16(g.id[14:], g.count)
	g.count++
	return g.id, nil
}

// Random produces a new probably unique identifier using the RandomGenerator.
func Random() (ID, error) {
	return random.NewID()
}
