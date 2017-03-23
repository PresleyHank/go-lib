// queue test
package util_test

import (
    "testing"
    "runtime"

    //"util"  // our module
)


func xassert(cond bool, t *testing.T) {

    if cond { return }

    _, file, line, ok := runtime.Caller(1)
    if !ok {
        file = "???"
        line = 0
    }

    t.Fatalf("%s: %d: Assertion failed\n", file, line)
}


// Basic sanity tests
func Test7(t *testing.T) {

}
