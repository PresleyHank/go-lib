// debug_posix.go -- debugging hooks for POSIX OSes
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


// +build !android !windows !nacl
// +build darwin netbsd openbsd freebsd dragonflybsd

// Android package lives in android/pkg
package pkg // android/pkg

import (
    "os"
    "fmt"
)

// Return the calling uid as a pseudo package
func getself() *Pkg {
    pid := fmt.Sprintf("caller-pid-%v", os.Getpid())
    p := &Pkg{Name: pid, Uid: uint32(os.Getuid())}
    return p
}

