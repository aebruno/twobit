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
    sequence     []byte
}

// TwoBit stores the file index and header information of the 2bit file
type twoBit struct {
    reader       io.ReadSeeker
    hdr          header
    index        map[string]int
    records      map[string]*seqRecord
}

type Reader twoBit
type Writer twoBit

// Return the size in packed bytes of a dna sequence. 4 bases per byte
func packedSize(dnaSize int) (int) {
    return (dnaSize + 3) >> 2
}

// Parse the file index of a 2bit file
func (r *Reader) parseIndex() (error) {
    r.index = make(map[string]int)

    for i := 0; i < r.Count(); i++ {
        var size uint8
        err := binary.Read(r.reader, r.hdr.byteOrder, &size)
        if err != nil {
            return fmt.Errorf("Failed to read file index: %s", err)
        }

        name := make([]byte, size)
        err = binary.Read(r.reader, r.hdr.byteOrder, &name)
        if err != nil {
            return fmt.Errorf("Failed to read file index: %s", err)
        }

        var offset uint32
        err = binary.Read(r.reader, r.hdr.byteOrder, &offset)
        if err != nil {
            return fmt.Errorf("Failed to read file index: %s", err)
        }

        r.index[string(name)] = int(offset)
    }

    return nil
}

// Parse the header of a 2bit file
func (r *Reader) parseHeader() (error) {
    b := make([]byte, 16)
    _, err := io.ReadFull(r.reader, b)
    if err != nil {
        return err
    }

    r.hdr.sig = binary.BigEndian.Uint32(b[0:4])
    r.hdr.byteOrder = binary.BigEndian

    if r.hdr.sig != SIG {
        r.hdr.sig = binary.LittleEndian.Uint32(b[0:4])
        r.hdr.byteOrder = binary.LittleEndian
        if r.hdr.sig != SIG {
            return fmt.Errorf("Invalid sig. Not a 2bit file?")
        }
    }

    r.hdr.version = r.hdr.byteOrder.Uint32(b[4:8])
    if r.hdr.version != uint32(0) {
        return fmt.Errorf("Unsupported version %d", r.hdr.version)
    }
    r.hdr.count = r.hdr.byteOrder.Uint32(b[8:12])
    r.hdr.reserved = r.hdr.byteOrder.Uint32(b[12:16])
    if r.hdr.reserved != uint32(0) {
        return fmt.Errorf("Reserved != 0. got %d", r.hdr.reserved)
    }

    return nil
}

// Parse the nBlock and mBlock coordinates
func (r *Reader) parseBlockCoords() (map[int]int, error) {
    var count uint32
    err := binary.Read(r.reader, r.hdr.byteOrder, &count)
    if err != nil {
        return nil, fmt.Errorf("Failed to read blockCount: %s", err)
    }

    starts := make([]uint32, count)
    for i := range(starts) {
        err = binary.Read(r.reader, r.hdr.byteOrder, &starts[i])
        if err != nil {
            return nil, fmt.Errorf("Failed to block start: %s", err)
        }
    }

    sizes := make([]uint32, count)
    for i := range(sizes) {
        err = binary.Read(r.reader, r.hdr.byteOrder, &sizes[i])
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
func (r *Reader) parseRecord(name string, coords bool) (*seqRecord, error) {
    rec := new(seqRecord)

    offset, ok := r.index[name]
    if !ok {
        return nil, fmt.Errorf("Invalid sequence name: %s", name)
    }

    r.reader.Seek(int64(offset), 0)

    err := binary.Read(r.reader, r.hdr.byteOrder, &rec.dnaSize)
    if err != nil {
        return nil, fmt.Errorf("Failed to read dnaSize: %s", err)
    }

    if coords {
        rec.nBlocks, err = r.parseBlockCoords()
        if err != nil {
            return nil, fmt.Errorf("Failed to read nBlocks: %s", err)
        }

        rec.mBlocks, err = r.parseBlockCoords()
        if err != nil {
            return nil, fmt.Errorf("Failed to read mBlocks: %s", err)
        }

        err = binary.Read(r.reader, r.hdr.byteOrder, &rec.reserved)
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
func (r *Reader) NBlocks(name string) (map[int]int, error) {
    rec, err := r.parseRecord(name, true)
    if err != nil {
        return nil, err
    }

    return rec.nBlocks, nil
}

// Read entire sequence.
func (r *Reader) Read(name string) (string, error) {
    return r.ReadRange(name, 0, 0)
}

// Read sequence from start to end.
func (r *Reader) ReadRange(name string, start, end int) (string, error) {
    rec, err := r.parseRecord(name, true)
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

        r.reader.Seek(int64(shift), 1)
    }

    var dna bytes.Buffer
    for i := 0; i < size; i++ {
        var base byte
        err = binary.Read(r.reader, r.hdr.byteOrder, &base)
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
func NewReader(r io.ReadSeeker) (*Reader, error) {
    tb := new(Reader)
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
func (r *Reader) Length(name string) (int, error) {
    rec, err := r.parseRecord(name, false)
    if err != nil {
        return -1, err
    }

    return int(rec.dnaSize), nil
}

// Returns the length for sequence with name but does not count Ns
func (r *Reader) LengthNoN(name string) (int, error) {
    rec, err := r.parseRecord(name, true)
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
func (r *Reader) Names() ([]string) {
    names := make([]string, len(r.index))

    i := 0
    for n := range r.index {
        names[i] = n
        i++
    }

    return names
}

// Returns the count of sequences in the 2bit file
func (r *Reader) Count() (int) {
    return int(r.hdr.count)
}

// Returns the version of the 2bit file
func (r *Reader) Version() (int) {
    return int(r.hdr.version)
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

// New Writer
func NewWriter() (*Writer) {
    tb := new(Writer)
    tb.records = make(map[string]*seqRecord)

    return tb
}

func mapBlocks(seq string, check func(r rune) bool) map[int]int {
    blocks := make(map[int]int)

    n      := len(seq)
    start  := 0
    match  := false
    isLast := false
    for i := 0; i < n; i++ {
        match = check(rune(seq[i]))
        if match {
            if !isLast {
                start = i
            }
        } else {
            if isLast {
                blocks[start] = i - start
            }
        }
        isLast = match
    }

    if isLast {
        blocks[start] = n - start
    }

    return blocks
}

// Add sequence
func (w *Writer) Add(name, seq string) (error) {
    rec := new(seqRecord)
    rec.dnaSize = uint32(len(seq))
    rec.nBlocks = mapBlocks(seq, func(r rune) bool {
        return r == 'N' || r == 'n'
    })
    rec.mBlocks = mapBlocks(seq, func(r rune) bool {
        return r >= 'a' && r <= 'z'
    })


    pack, err := Pack(seq)
    if err != nil {
        return err
    }

    rec.sequence = pack

    w.records[name] = rec

    return nil
}

// Write sequences in 2bit format to out
func (w *Writer) WriteTo(out io.Writer) (error) {
    header := make([]byte, 16)
    binary.LittleEndian.PutUint32(header[0:4], SIG)
    binary.LittleEndian.PutUint32(header[4:8], uint32(0))
    binary.LittleEndian.PutUint32(header[8:12], uint32(len(w.records)))
    binary.LittleEndian.PutUint32(header[8:16], uint32(0))
    _, err := out.Write(header)
    if err != nil {
        return err
    }

    return nil
}
