package config

import (
	"bytes"
	"strings"
	"sync"

	"veyron.io/veyron/veyron2/verror"
	"veyron.io/veyron/veyron2/vom"
)

var ErrKeyNotFound = verror.NotFoundf("config key not found")

// TODO(caprita): Move the interface to veyron2 and integrate with
// veyron/services/config.

// Config defines a simple key-value configuration.  Keys and values are
// strings, and a key can have exactly one value.  The client is responsible for
// encoding structured values, or multiple values, in the provided string.
//
// Config data can come from several sources:
// - passed from parent process to child process through pipe;
// - using environment variables or flags;
// - via the neighborhood-based config service;
// - by RPCs using the Config idl;
// - manually, by calling the Set method.
//
// This interface makes no assumptions about the source of the configuration,
// but provides a unified API for accessing it.
type Config interface {
	// Set sets the value for the key.  If the key already exists in the
	// config, its value is overwritten.
	Set(key, value string)
	// Get returns the value for the key.  If the key doesn't exist in the
	// config, Get returns an error.
	Get(key string) (string, error)
	// Serialize serializes the config to a string.
	Serialize() (string, error)
	// MergeFrom deserializes config information from a string created using
	// Serialize(), and merges this information into the config, updating
	// values for keys that already exist and creating new key-value pairs
	// for keys that don't.
	MergeFrom(string) error
}

type cfg struct {
	sync.RWMutex
	m map[string]string
}

// New creates a new empty config.
func New() Config {
	return &cfg{m: make(map[string]string)}
}

func (c cfg) Set(key, value string) {
	c.Lock()
	defer c.Unlock()
	c.m[key] = value
}

func (c cfg) Get(key string) (string, error) {
	c.RLock()
	defer c.RUnlock()
	v, ok := c.m[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return v, nil
}

func (c cfg) Serialize() (string, error) {
	var buf bytes.Buffer
	c.RLock()
	defer c.RUnlock()
	if err := vom.NewEncoder(&buf).Encode(c.m); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (c cfg) MergeFrom(serialized string) error {
	var newM map[string]string
	if err := vom.NewDecoder(strings.NewReader(serialized)).Decode(&newM); err != nil {
		return err
	}
	c.Lock()
	defer c.Unlock()
	for k, v := range newM {
		c.m[k] = v
	}
	return nil
}
