// readline.go -- generic readline routine
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
//

package fileio

import (
    "io"
    "bufio"
    "strings"
)


// Read a line of input from the reader
func Readline(r *bufio.Reader) (string, int) {
    b, err := r.ReadString('\n')
    x := len(b)
    if x == 0 {
        if err == io.EOF { return "", -1 }
        return "", 0
    }

    if b[x-1] == '\n' {
        b  = b[:x-1]
        x -= 1
    }

    if x == 0 { return "", 0 }

    b = strings.TrimSpace(b)
    x = len(b)
    if x == 0 || b[0] == '#' { return "", 0 }

    return b, x
}


// Read fd and return a chan which yields lines
func Genlines(fd io.ReadCloser) chan string {
    ch := make(chan string, 2)

    go func(ch chan string, fd io.ReadCloser) {
        rd := bufio.NewReader(fd)
        for b, s := Readline(rd); s != -1; b, s = Readline(rd) {
            if s == 0  { continue }

            ch <- b
        }
        close(ch)
        fd.Close()
    }(ch, fd)

    return ch
}

// vim: ft=go:sw=4:ts=4:expandtab:tw=78:
