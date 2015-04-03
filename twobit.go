// Copyright 2015 Andrew E. Bruno. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

// Package twobit implements the 2bit compact randomly-accessible file format
// for storing DNA sequence data.
package twobit

import (
    "fmt"
    "io"
    "bytes"
    "encoding/binary"
)

// 2bit header
type header struct {
    sig         uint32
    version     uint32
    count       uint32
    reserved    uint32
    byteOrder   binary.ByteOrder
}

// seqRecord stores sequence record from the file index
type seqRecord struct {
    dnaSize      uint32
    nBlocks      map[int]int
    mBlocks      map[int]int
    reserved     uint32
}

// TwoBit stores the file index and header information of the 2bit file
type TwoBit struct {
    reader       io.ReadSeeker
    hdr          header
    index        map[string]int
}

// Return the size in packed bytes of a dna sequence. 4 bases per byte
func packedSize(dnaSize int) (int) {
    return (dnaSize + 3) >> 2
}

// Parse the file index of a 2bit file
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

// Parse the header of a 2bit file
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

// Parse the nBlock and mBlock coordinates
func (tb *TwoBit) parseBlockCoords() (map[int]int, error) {
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

// Parse the sequence record information
func (tb *TwoBit) parseRecord(name string, coords bool) (*seqRecord, error) {
    rec := new(seqRecord)

    offset, ok := tb.index[name]
    if !ok {
        return nil, fmt.Errorf("Invalid sequence name: %s", name)
    }

    tb.reader.Seek(int64(offset), 0)

    err := binary.Read(tb.reader, tb.hdr.byteOrder, &rec.dnaSize)
    if err != nil {
        return nil, fmt.Errorf("Failed to read dnaSize: %s", err)
    }

    if coords {
        rec.nBlocks, err = tb.parseBlockCoords()
        if err != nil {
            return nil, fmt.Errorf("Failed to read nBlocks: %s", err)
        }

        rec.mBlocks, err = tb.parseBlockCoords()
        if err != nil {
            return nil, fmt.Errorf("Failed to read mBlocks: %s", err)
        }

        err = binary.Read(tb.reader, tb.hdr.byteOrder, &rec.reserved)
        if err != nil {
            return nil, fmt.Errorf("Failed to read reserved: %s", err)
        }

        if rec.reserved != uint32(0) {
            return nil, fmt.Errorf("Invalid reserved")
        }
    }

    return rec, nil
}

// Return blocks of Ns in sequence with name
func (tb *TwoBit) NBlocks(name string) (map[int]int, error) {
    rec, err := tb.parseRecord(name, true)
    if err != nil {
        return nil, err
    }

    return rec.nBlocks, nil
}

// Read entire sequence.
func (tb *TwoBit) Read(name string) (string, error) {
    return tb.ReadRange(name, 0, 0)
}

// Read sequence from start to end.
func (tb *TwoBit) ReadRange(name string, start, end int) (string, error) {
    rec, err := tb.parseRecord(name, true)
    if err != nil {
        return "", err
    }

    bases := int(rec.dnaSize)

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
    size := packedSize(bases)
    if start > 0 {
        shift := packedSize(start)
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
            buf[j] = BYTES2NT[base & 0x3]
            base >>= 2
        }

        if i == 0 {
            dna.Write(buf[(start%4):])
            continue
        }

        dna.Write(buf)
    }

    seq := dna.Bytes()[0:bases]

    for bi, cnt := range rec.nBlocks {
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

    for bi, cnt := range rec.mBlocks {
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

// NewReader returns a new TwoBit file reader which reads from r
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

// Returns the length for sequence with name
func (tb *TwoBit) Length(name string) (int, error) {
    rec, err := tb.parseRecord(name, false)
    if err != nil {
        return -1, err
    }

    return int(rec.dnaSize), nil
}

// Returns the length for sequence with name but does not count Ns
func (tb *TwoBit) LengthNoN(name string) (int, error) {
    rec, err := tb.parseRecord(name, true)
    if err != nil {
        return -1, err
    }

    n := 0
    for _, cnt := range rec.nBlocks {
        n += cnt
    }

    return int(rec.dnaSize)-n, nil
}

// Returns the names of sequences in the 2bit file
func (tb *TwoBit) Names() ([]string) {
    names := make([]string, len(tb.index))

    i := 0
    for n := range tb.index {
        names[i] = n
        i++
    }

    return names
}

// Returns the count of sequences in the 2bit file
func (tb *TwoBit) Count() (int) {
    return int(tb.hdr.count)
}

// Returns the version of the 2bit file
func (tb *TwoBit) Version() (int) {
    return int(tb.hdr.version)
}

// Unpack array of bytes to DNA string of length sz
func Unpack(raw []byte, sz int) (string) {
    var dna bytes.Buffer
    for _, base := range raw {
        buf := make([]byte, 4)
        for j := 3; j >= 0; j-- {
            buf[j] = BYTES2NT[base & 0x3]
            base >>= 2
        }

        dna.Write(buf)
    }

    return string(dna.Bytes()[0:sz])
}

// Packs DNA sequence string into an array of bytes. 4 bases per byte.
func Pack(s string) ([]byte, error) {
    sz := len(s)
    out := make([]byte, packedSize(sz))

    idx := 0
    for i := range out {
        var b uint8
        for j := 0; j < 4; j++ {
            val := NT2BYTES['T']
            if idx < sz {
                v, ok := NT2BYTES[s[idx]]
                if !ok {
                    return nil, fmt.Errorf("Unsupported base: %c", s[idx])
                }
                val = v
            }
            b <<= 2
            b += val
            idx++
        }
        out[i] = b
    }

    return out, nil
}
