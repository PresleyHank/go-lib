// ringbuf.go -- Ringbuffer for packet I/O
//
// (c) 2016 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2 
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

package ringbuf


import (
    "net"
)

const BUFSIZE   int = 2048

// Abstraction of a UDP buf
type PacketBuf struct {
    Data    []byte      // slice of buf below
    Dest    net.Addr    // destination to send this off to

    buf     [BUFSIZE]byte

    r      *Ring
}


type Ring struct {
    Size    int     // number of buffers
    q       chan *PacketBuf
}


// Create a new ring buffer to hold N packets
func NewRing(n int) *Ring {
    r := &Ring{Size: n}

    r.q = make(chan *PacketBuf, n)

    for ;n > 0; n -= 1 {
        u := &PacketBuf{r: r}
        u.Data = u.buf[:]
        r.q <- u
    }

    return r
}



// Blocking operation if ring is empty
func (r *Ring) Get() *PacketBuf {
    u := <- r.q
    u.r = r
    return u
}

func (u *PacketBuf) Free() {
    u.Data = u.buf[:]
    u.r.q <- u
}

func (u *PacketBuf) Reset() []byte {
    u.Data = u.buf[:]
    return u.Data
}

