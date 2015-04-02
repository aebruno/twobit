// Copyright 2015 Andrew E. Bruno. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package twobit

import (
    "fmt"
    "io"
    "bytes"
    "encoding/binary"
)

type header struct {
    sig         uint32
    version     uint32
    count       uint32
    reserved    uint32
    byteOrder   binary.ByteOrder
}

type TwoBit struct {
    reader       io.ReadSeeker
    hdr          header
    index        map[string]int
}

func (tb *TwoBit) packedSize(dnaSize int) (int) {
    return (dnaSize + 3) >> 2
}

func (tb *TwoBit) parseIndex() (error) {
    tb.index = make(map[string]int)

    for i := 0; i < tb.Count(); i++ {
        var size uint8
        err := binary.Read(tb.reader, tb.hdr.byteOrder, &size)
        if err != nil {
            return fmt.Errorf("Failed to read file index: %s", err)
        }

        name := make([]byte, size)
        err = binary.Read(tb.reader, tb.hdr.byteOrder, &name)
        if err != nil {
            return fmt.Errorf("Failed to read file index: %s", err)
        }

        var offset uint32
        err = binary.Read(tb.reader, tb.hdr.byteOrder, &offset)
        if err != nil {
            return fmt.Errorf("Failed to read file index: %s", err)
        }

        tb.index[string(name)] = int(offset)
    }

    return nil
}

func (tb *TwoBit) parseHeader() (error) {
    b := make([]byte, 16)
    _, err := io.ReadFull(tb.reader, b)
    if err != nil {
        return err
    }

    tb.hdr.sig = binary.BigEndian.Uint32(b[0:4])
    tb.hdr.byteOrder = binary.BigEndian

    if tb.hdr.sig != 0x1A412743 {
        tb.hdr.sig = binary.LittleEndian.Uint32(b[0:4])
        tb.hdr.byteOrder = binary.LittleEndian
        if tb.hdr.sig != 0x1A412743 {
            return fmt.Errorf("Invalid sig. Not a 2bit file?")
        }
    }

    tb.hdr.version = tb.hdr.byteOrder.Uint32(b[4:8])
    if tb.hdr.version != uint32(0) {
        return fmt.Errorf("Unsupported version %d", tb.hdr.version)
    }
    tb.hdr.count = tb.hdr.byteOrder.Uint32(b[8:12])
    tb.hdr.reserved = tb.hdr.byteOrder.Uint32(b[12:16])
    if tb.hdr.reserved != uint32(0) {
        return fmt.Errorf("Reserved != 0. got %d", tb.hdr.reserved)
    }

    return nil
}

func (tb *TwoBit) readBlockCoords() (map[int]int, error) {
    var count uint32
    err := binary.Read(tb.reader, tb.hdr.byteOrder, &count)
    if err != nil {
        return nil, fmt.Errorf("Failed to read blockCount: %s", err)
    }

    starts := make([]uint32, count)
    for i := range(starts) {
        err = binary.Read(tb.reader, tb.hdr.byteOrder, &starts[i])
        if err != nil {
            return nil, fmt.Errorf("Failed to block start: %s", err)
        }
    }

    sizes := make([]uint32, count)
    for i := range(sizes) {
        err = binary.Read(tb.reader, tb.hdr.byteOrder, &sizes[i])
        if err != nil {
            return nil, fmt.Errorf("Failed to block size: %s", err)
        }
    }

    blocks := make(map[int]int)

    for i := range(starts) {
        blocks[int(starts[i])] = int(sizes[i])
    }

    return blocks, nil
}

func (tb *TwoBit) Read(name string, start, end int) (string, error) {
    offset, ok := tb.index[name]
    if !ok {
        return "", fmt.Errorf("Invalid sequence name: %s", name)
    }

    tb.reader.Seek(int64(offset), 0)

    var dnaSize uint32
    err := binary.Read(tb.reader, tb.hdr.byteOrder, &dnaSize)
    if err != nil {
        return "", fmt.Errorf("Failed to read dnaSize: %s", err)
    }

    nBlocks, err := tb.readBlockCoords()
    if err != nil {
        return "", fmt.Errorf("Failed to read nBlocks: %s", err)
    }

    mBlocks, err := tb.readBlockCoords()
    if err != nil {
        return "", fmt.Errorf("Failed to read mBlocks: %s", err)
    }

    var reserved uint32
    err = binary.Read(tb.reader, tb.hdr.byteOrder, &reserved)
    if err != nil {
        return "", fmt.Errorf("Failed to read reserved: %s", err)
    }

    if reserved != uint32(0) {
        return "", fmt.Errorf("Invalid reserved")
    }

    bases := int(dnaSize)
    size := tb.packedSize(bases)

    // TODO: handle -1 ?
    if start < 0 {
        start = 0
    }

    //TODO: should we error out here?
    if end > bases {
        end = bases
    }

    // TODO: handle -1 ?
    if end == 0 || end < 0 {
        end = bases
    }

    if end <= start {
        return "", fmt.Errorf("Invalid range: %d-%d", start, end)
    }

    bases = end-start
    size = tb.packedSize(bases)
    if start > 0 {
        shift := tb.packedSize(start)
        if start % 4 != 0 {
            shift--
            size++
        }

        tb.reader.Seek(int64(shift), 1)
    }

    var dna bytes.Buffer
    for i := 0; i < size; i++ {
        var base byte
        err = binary.Read(tb.reader, tb.hdr.byteOrder, &base)
        if err != nil {
            return "", fmt.Errorf("Failed to read base: %s", err)
        }

        buf := make([]byte, 4)
        for j := 3; j >= 0; j-- {
            buf[j] = NT_BYTES[base & 0x3]
            base >>= 2
        }

        if i == 0 {
            dna.Write(buf[(start%4):])
            continue
        }

        dna.Write(buf)
    }

    seq := dna.Bytes()[0:bases]

    for bi, cnt := range nBlocks {
        if (bi+cnt) < start || bi > end {
            continue
        }
        idx := bi-start
        if idx < 0 {
            cnt += idx
            idx = 0
        }
        for i := 0; i < cnt; i++ {
            seq[idx] = BASE_N
            idx++
            if idx >= len(seq) {
                break
            }
        }
    }

    for bi, cnt := range mBlocks {
        if (bi+cnt) < start || bi > end {
            continue
        }
        idx := bi-start
        if idx < 0 {
            cnt += idx
            idx = 0
        }
        for i := 0; i < cnt; i++ {
            // Faster lower case.. see: https://groups.google.com/forum/#!topic/golang-nuts/Il2DX4xpW3w
            seq[idx] = seq[idx] + 32 // ('a' - 'A')
            idx++
            if idx >= len(seq) {
                break
            }
        }
    }

    return string(seq), nil
}

func NewReader(r io.ReadSeeker) (*TwoBit, error) {
    tb := new(TwoBit)
    tb.reader = r
    err := tb.parseHeader()
    if err != nil {
        return nil, err
    }

    err = tb.parseIndex()
    if err != nil {
        return nil, err
    }

    return tb, nil
}

func (tb *TwoBit) Count() (int) {
    return int(tb.hdr.count)
}

func (tb *TwoBit) Version() (int) {
    return int(tb.hdr.version)
}
