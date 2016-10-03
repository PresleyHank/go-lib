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

// Default pool size
const Poolsize = 64


type Bufpool struct {
    Size int
    q    chan interface{}
}


func NewBufpool(sz int) *Bufpool {
    if sz <= 0 { sz = Poolsize }

    b := &Bufpool{Size: sz}
    b.q = make(chan interface{}, sz)

    return b
}

func (b *Bufpool) Put(o interface{}) {
    b.q <- o
}

func (b *Bufpool) Get() interface{} {
    o := <- b.q
    return o
}

