// mmap.go - Better interface to mmap(2) on go
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

package util


import (
    "os"
    "io"
    "fmt"
    "syscall"
)


// A mmap'd file reader that processes an already pen file in large chunks.
// The default chunk-size is 1GB (1024 x 1024 x 1024 bytes).
//
// This function can be used to efficiently hash very large files:
//
//    h := sha256.New()
//    err := MmapReader(fd, 0, 0, h)
func MmapReader(fd *os.File, off, sz int64, wr io.Writer) error {
    // Mmap'ing large files won't work. We need to do it in 1 or 2G
    // chunks.
    const chunk  int64 = 1 * 1024 * 1024 * 1024

    if sz == 0 {
        st, err := fd.Stat()
        if err != nil { return fmt.Errorf("can't stat: %s", err) }
        sz = st.Size()
    }

    if off >= sz { return fmt.Errorf("can't mmap outside file size (off %v filesize %v)", off, sz) }

    for sz > 0 {
        var n = int(sz)

        if sz > chunk { n = int(chunk) }

        mem, err := syscall.Mmap(int(fd.Fd()), off, n, syscall.PROT_READ, syscall.MAP_SHARED)
        if err   != nil { return fmt.Errorf("can't mmap %v bytes at %v: %s", n, off, err) }

        wr.Write(mem)
        syscall.Munmap(mem)

        off += int64(n)
        sz  -= int64(n)
    }

    return nil
}

// EOF
