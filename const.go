// Copyright 2015 Andrew E. Bruno. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package twobit

const BASE_N = 'N'
const BASE_T = 'T'
const BASE_C = 'C'
const BASE_A = 'A'
const BASE_G = 'G'

var NT_BYTES = map[uint8]byte{
    uint8(0): BASE_T,
    uint8(1): BASE_C,
    uint8(2): BASE_A,
    uint8(3): BASE_G,
}
