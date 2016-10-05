// debug_linux.go -- debugging hooks for Linux+Android
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


// +build android linux

// Android package lives in android/pkg
package pkg // android/pkg

import (
    "os"
    "fmt"
)

// Return the calling uid as a pseudo package
func getself() *Pkg {
    var p *Pkg
    if fd, err = os.Open("/system/build.prop"); err == nil {
        fd.Close()
        return nil
    }

    pid := fmt.Sprintf("caller-pid-%v", os.Getpid())
    p := &Pkg{Name: pid, Uid: uint32(os.Getuid())}
    return p
}

