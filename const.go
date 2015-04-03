// Copyright 2015 Andrew E. Bruno. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package twobit

const BASE_N = 'N'
const BASE_T = 'T'
const BASE_C = 'C'
const BASE_A = 'A'
const BASE_G = 'G'

var BYTES2NT = map[uint8]byte{
    uint8(0): BASE_T,
    uint8(1): BASE_C,
    uint8(2): BASE_A,
    uint8(3): BASE_G,
}

var NT2BYTES = map[byte]uint8{
    BASE_N: uint8(0),
    BASE_T: uint8(0),
    BASE_C: uint8(1),
    BASE_A: uint8(2),
    BASE_G: uint8(3),
    BASE_N+32: uint8(0),
    BASE_T+32: uint8(0),
    BASE_C+32: uint8(1),
    BASE_A+32: uint8(2),
    BASE_G+32: uint8(3),
}
