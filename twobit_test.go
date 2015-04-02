// Copyright 2015 Andrew E. Bruno. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package twobit

import (
    "testing"
    "os"
)

func TestHeader(t *testing.T) {
    f, err := os.Open("examples/simple.2bit")
    if err != nil {
        t.Errorf("Failed to open example file")
    }

    tb, err := NewReader(f)
    if err != nil {
        t.Errorf("%s", err)
    }

    if tb.Count() != 1 {
        t.Errorf("Invalid sequence count: %d != %d", tb.Count(), 1)
    }

    names := map[string]bool{
        "ex1"   : false,
    }

    for name := range tb.index {
        if _, ok := names[name]; !ok {
            t.Errorf("Invalid sequence name: %s", name)
        }
        names[name] = true
    }
    for name, seen := range names {
        if !seen {
            t.Errorf("Sequence name not found in file index: %s", name)
        }
    }
}

func TestRead(t *testing.T) {
    f, err := os.Open("examples/simple.2bit")
    if err != nil {
        t.Errorf("Failed to open example file")
    }

    tb, err := NewReader(f)
    if err != nil {
        t.Errorf("%s", err)
    }

    _, err = tb.Read("not-found", 0, 0)
    if err == nil {
        t.Errorf("Found non-existent name")
    }

    regions := map[string][]int {
        "ACTgcctttnnnNantnaCgc": []int{0, 0},
        "ACTgc"                : []int{0, 5},
             "ctttnn"          : []int{5, 11},
                       "tnaCgc": []int{15, 21},
                           "gc": []int{19, 21},
                            "c": []int{20, 21},
    }

    for good, coords := range regions {
        seq, err := tb.Read("ex1", coords[0], coords[1])
        if err != nil {
            t.Errorf("Failed to read sequence: %s", err)
        }

        if seq != good {
            t.Errorf("Invalid sequence: %s != %s", seq, good)
        }
    }
}
