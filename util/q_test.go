// queue test
package util_test

import (
    "testing"
    "runtime"

    "util"  // our module
)


func assert(cond bool, t *testing.T) {

    if cond { return }

    _, file, line, ok := runtime.Caller(1)
    if !ok {
        file = "???"
        line = 0
    }

    t.Fatalf("%s: %d: Assertion failed\n", file, line)
}


// Basic sanity tests
func Test0(t *testing.T) {

    var v bool

    q := util.NewQ(4)

    assert(q.IsEmpty(), t)
    assert(!q.IsFull(), t)

    v = q.Enq(10); assert(v, t)

    v = q.Enq(20); assert(v, t)

    v = q.Enq(30); assert(v, t)

    assert(q.IsFull(), t)

    assert(q.Size() == 3, t)

    // Now, the q will be full
    v = q.Enq(40); assert(!v, t)

    // Pull items off the queue

    assert(!q.IsEmpty(), t)

    z, v  := q.Deq(); assert(v, t)
    x, ok := z.(int); assert(ok, t)
    assert(x == 10, t)

    z, v   = q.Deq(); assert(v, t)
    x, ok  = z.(int); assert(ok, t)
    assert(x == 20, t)

    z, v   = q.Deq(); assert(v, t)
    x, ok  = z.(int); assert(ok, t)
    assert(x == 30, t)

    assert(q.IsEmpty(), t)

    z, v   = q.Deq(); assert(!v, t)
}


// Test wrap around
func Test1(t *testing.T) {

    var v bool

    q := util.NewQ(4)


    v = q.Enq(10); assert(v, t)
    v = q.Enq(20); assert(v, t)
    v = q.Enq(30); assert(v, t)

    assert(q.IsFull(), t)
    assert(q.Size() == 3, t)

    z, v  := q.Deq(); assert(v, t)
    x, ok := z.(int); assert(ok, t)
    assert(x == 10, t)

    z, v   = q.Deq(); assert(v, t)
    x, ok  = z.(int); assert(ok, t)
    assert(x == 20, t)

    // This will wrap around
    v = q.Enq(40); assert(v, t)
    v = q.Enq(50); assert(v, t)

    //t.Logf("Q: %s\n", q)
    assert(q.Size() == 3, t)
}
