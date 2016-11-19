// bufpool.go -- Buffer pool (blocking) abstraction
//
// (c) 2015, 2016 -- Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

package util

// A fixed-size buffer-pool backed by a channel. Callers are
// expected to free the buffer back to its originating pool.
type Bufpool struct {
    Size int
    q    chan interface{}
}

// Default pool size
const Poolsize = 64

// Create a new Bufpool. The caller is responsible for filling this
// pool with initial data.
func NewBufpool(sz int) *Bufpool {
    if sz <= 0 { sz = Poolsize }

    b  := &Bufpool{Size: sz}
    b.q = make(chan interface{}, sz)

    return b
}


// Put an item into the bufpool. This should not ever block; it
// indicates pool integrity failure (duplicates or erroneous Puts).
func (b *Bufpool) Put(o interface{}) {
    b.q <- o
}

// Get the next available item from the pool; block the caller if
// none are available.
func (b *Bufpool) Get() interface{} {
    o := <- b.q
    return o
}

// EOF
