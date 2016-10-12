// mtrand.go -- Mersenne Twister with a known seed
// 
// (c) 2015, 2016 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2 
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.
//
// Notes:
// ======
// o 32-bit Mersenne-Twister MT19937
// o Not safe for calling from multiple goroutines
// o Ref: https://en.wikipedia.org/wiki/Mersenne_Twister
//

package mtrand  // github.com/opencoff/go-lib/mtrand

import (
    "time"
)


type MT struct {

    mt  [624]uint32
    i   int
}


func New(seed uint32) *MT {
    m  := &MT{}
    mt := m.mt[:]

    if seed == 0 { seed = uint32(time.Now().UnixNano()) }

    mt[0] = seed
    for i := 1; i < 624; i++ {
        y := mt[i-1]
        mt[i] = 1812433253 * (y ^ (y >> 30)) + uint32(i)
    }

    return m
}


func (m *MT) twist() int {
    mt := m.mt[:]
    for i := 0; i < 624; i++ {
        // Get the most significant bit and add it to the less significant
        // bits of the next number
        y := (mt[i] & 0x80000000) | (mt[(i + 1) % 624] & 0x7fffffff)
        mt[i] = mt[(i + 397) % 624] ^ (y >> 1)

        if y % 2 != 0 { mt[i] ^= 0x9908b0df }
    }
    m.i = 0
    return m.i
}


func (m *MT) Next() uint32 {
    mt := m.mt[:]
    i  := m.i

    if i >= 624 { i = m.twist() }

    y := mt[i]
    y ^= (y >> 11)
	y ^= ((y << 7)  & 2636928640)
	y ^= ((y << 15) & 4022730752)
    y ^= (y >> 18)

    m.i = i+1
    return y
}

// vim: ft=go:sw=4:ts=4:tw=78:expandtab:
