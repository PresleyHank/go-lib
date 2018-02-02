// stdwrapper.go - wrapper around my logger to make it compatible
// with stdlib log.Logger.
//
// Changes Copyright 2012, Sudhi Herle <sudhi -at- herle.net>
// This code is licensed under the same terms as the golang core.

package logger

import (
    gl "log"
)

// Return an instance of self that satisfies stdlib logger
func (l *Logger) StdLogger() *gl.Logger {

    fl := gl.LUTC
    if 0 != (l.flag&Ldate)          { fl |= gl.Ldate }
    if 0 != (l.flag&Ltime)          { fl |= gl.Ltime }
    if 0 != (l.flag&Lmicroseconds)  { fl |= gl.Lmicroseconds }
    if 0 != (l.flag&Llongfile)      { fl |= gl.Llongfile }
    if 0 != (l.flag&Lshortfile)     { fl |= gl.Lshortfile }

    // here 'l' is the io.Writer
    l2 := gl.New(l, l.prefix, fl)
    return l2
}


// We only provide an ioWriter implementation for stdlogger
func (l *Logger) Write(b []byte) (int, error) {
    l.qwrite(string(b))
    return len(b), nil
}

