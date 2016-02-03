// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simplemr

type Identity struct{}

func (i *Identity) Map(mr *MR, key string, val interface{}) error {
	mr.MapOut(key, val)
	return nil
}

func (i *Identity) Reduce(mr *MR, key string, values []interface{}) error {
	mr.ReduceOut(key, values...)
	return nil
}
