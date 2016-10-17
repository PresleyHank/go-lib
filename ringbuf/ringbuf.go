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


// Package ringbuf implements a blocking packet-buffer backed by a channel.
//
// This allows a ringbuf instance to be naturally race-free and
// thread-safe. Each packet-buffer is pre-allocated and stored in a
// buffered channel. The default size of the packet-buf is BUFSIZE
// bytes.
package ringbuf


import (
    "net"
)

const BUFSIZE   int = 2048

// Abstraction of a UDP buf
type PacketBuf struct {
    Data    []byte      // slice of buf below
    Dest    net.Addr    // destination to send this off to


    r      *Ring        // backpointer to the ring it belongs to
    buf     []byte      // original buffer - 'Data' is a slice into this
}


type Ring struct {
    Size    int     // number of buffers
    bufsize int     // size of an individual buffer
    q       chan *PacketBuf
}


// Create a new ring buffer to hold 'nbufs' packets where each
// packet-buffer is 'bufsize' in size. if 'bufsize' is zero, it
// defaults to BUFSIZE bytes.
func NewRing(nbufs, bufsize int) *Ring {
    r := &Ring{Size: nbufs}

    if bufsize <= 0 { bufsize = BUFSIZE }

    r.q = make(chan *PacketBuf, nbufs)

    for ;n > 0; n -= 1 {
        u := &PacketBuf{r: r}
        u.buf  = make([]byte, bufsize)
        u.Data = u.buf[:]
        r.q <- u
    }

    return r
}



// Get a new packet-buffer from the ring.
// Blocking operation if ring is empty
func (r *Ring) Get() *PacketBuf {
    u := <- r.q
    u.r = r
    return u
}


// Free the packet-buffer 'u' back to its owning pool
func (u *PacketBuf) Free() {
    u.Data = u.buf[:]
    u.r.q <- u
}


// Reset the packet buffer to its original start/end
func (u *PacketBuf) Reset() []byte {
    u.Data = u.buf[:]
    return u.Data
}

