// Copyright 2016 Keybase Inc. All rights reserved.
// Use of this source code is governed by a BSD
// license that can be found in the LICENSE file.

// +build !dokan,!fuse

package test

const implementation = "libkbfs"

func createEngine() Engine {
	return &LibKBFS{}
}
