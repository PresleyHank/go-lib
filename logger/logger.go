// Copyright 2009 The Go Authors. All rights reserved.
//
// Changes Copyright 2012, Sudhi Herle <sudhi -at- herle.net>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


// Package Logger is an enhanced derivative of the Golang 'log'
// package.
//
// The list of enhancements are:
//
//  - All I/O is done in an asynchronous go-routine; thus, the caller
//    does not incur any overhead beyond the formatting of the
//    strings.
//
//  - Log levels define a heirarchy (from most-verbose to
//    least-verbose):
//      LOG_DEBUG
//      LOG_INFO
//      LOG_WARNING
//      LOG_ERR
//      LOG_CRIT
//      LOG_EMERG
//  
//  - An instance of a logger is configured with a given log level;
//    and it only prints log messages "above" the configured level.
//    e.g., if a logger is configured with level of INFO, then it will
//    print all log messages with INFO and higher priority;
//    in particular, it won't print DEBUG messages.
//
//  - A single program can have multiple loggers; each with a
//    different priority.
//
//  - The logger method Backtrace() will print a stack backtrace to
//    the configured output stream. Log levels are NOT
//    considered when backtraces are printed.
//
//  - The Panic() and Fatal() logger methods implicitly print the
//    stack backtrace (upto 5 levels).
//
//  - DEBUG, ERR, CRIT log outputs (via Debug(), Err() and Crit()
//    methods) also print the source file location from whence they
//    were invoked.
//
//  - New package functions to create a syslog(1) or a file logger
//    instance.
//
//  - Callers can create a new logger instance if they have an
//    io.writer instance of their own - in case the existing output
//    streams (File and Syslog) are insufficient.
//
//  - Any logger instance can create child-loggers with a different
//    priority and prefix (but same destination); this is useful in large
//    programs with different modules.
//
//  - Compressed log rotation based on daily ToD (configurable ToD) -- only
//    available for file-backed destinations.
package logger

import (
    "io"
    "os"
    "fmt"
    stdlog "log"
    "sync"
    "time"
    "errors"
    "strings"
    "runtime"
    "log/syslog"
    "sync/atomic"
    "crypto/rand"
    "compress/gzip"
    "encoding/binary"
)

// These flags define which text to prefix to each log entry generated by the Logger.
const (
    // Bits or'ed together to control what's printed. There is no control over the
    // order they appear (the order listed here) or the format they present (as
    // described in the comments).  A colon appears after these items:
    //  2009/01/23 01:23:23.123123 /a/b/c/d.go:23: message
    Ldate         = 1 << iota     // the date: 2009/01/23
    Ltime                         // the time: 01:23:23
    Lmicroseconds                 // microsecond resolution: 01:23:23.123123.  assumes Ltime.
    Llongfile                     // full file name and line number: /a/b/c/d.go:23
    Lshortfile                    // final file name element and line number: d.go:23. overrides Llongfile

    // Internal flags
    lSyslog                       // set to indicate that output destination is syslog
    lPrefix                       // set if prefix is non-zero
    lClose                        // close the file when done
    lSublog                       // Set if this is a sub-logger
    lRotate                       // Rotate the logs

    LstdFlags     = Ldate | Ltime // initial values for the standard logger
)

// Log priority. These form a heirarchy:
//
//   LOG_DEBUG
//   LOG_INFO
//   LOG_WARNING
//   LOG_ERR
//   LOG_CRIT
//   LOG_EMERG
//
// An instance of a logger is configured with a given log level;
// and it only prints log messages "above" the configured level.
type Priority int


// Maximum number of daily logs we will store
const MAX_LOGFILES = 7

// Log Priorities
const (
    LOG_NONE Priority = iota
    LOG_DEBUG
    LOG_INFO
    LOG_WARNING
    LOG_ERR
    LOG_CRIT
    LOG_EMERG
)

// Map string names to actual priority levels. Useful for taking log
// levels defined in config files and turning them into usable
// priorities.
var PrioName = map[string]Priority {
    "LOG_DEBUG": LOG_DEBUG,
    "LOG_INFO":  LOG_INFO,
    "LOG_WARNING": LOG_WARNING,
    "LOG_WARN": LOG_WARNING,
    "LOG_ERR": LOG_ERR,
    "LOG_ERROR": LOG_ERR,
    "LOG_CRIT": LOG_CRIT,
    "LOG_EMERG": LOG_EMERG,

    "DEBUG": LOG_DEBUG,
    "INFO":  LOG_INFO,
    "WARNING": LOG_WARNING,
    "WARN": LOG_WARNING,
    "ERR": LOG_ERR,
    "ERROR": LOG_ERR,
    "CRIT": LOG_CRIT,
    "CRITICAL": LOG_CRIT,
    "EMERG": LOG_EMERG,
    "EMERGENCY": LOG_EMERG,
}


// Map log priorities to their string names
var PrioString = map[Priority]string {
    LOG_DEBUG: "DEBUG",
    LOG_INFO:  "INFO",
    LOG_WARNING: "WARNING",
    LOG_ERR: "ERROR",
    LOG_CRIT: "CRITICAL",
    LOG_EMERG: "EMERGENCY",
}

// Since we now have sub-loggers, we need a way to keep the output
// channel and its close status together. This struct keeps the
// abstraction together. There is only ever _one_ instance of this
// struct in a top-level logger.
type outch struct {
    sync.Mutex
    closed uint32           // atomically set/read
    logch  chan string      // buffered channel
}

// A Logger represents an active logging object that generates lines of
// output to an io.Writer.  Each logging operation makes a single call to
// the Writer's Write method.  A Logger can be used simultaneously from
// multiple goroutines; it guarantees serialized access to the Writer.
type Logger struct {
    mu     sync.Mutex       // ensures atomic changes to properties
    prio   Priority         // Logging priority
    prefix string           // prefix to write at beginning of each line
    flag   int              // properties
    out    io.Writer        // destination for output
    name   string           // file name for file backed logs

    rot_tm  time.Time       // UTC time when file should be rotated
    rot_n   int             // number of days of logs to keep

    ch     *outch           // output chan
    wait   chan bool        // wait chan for closing the log

    gl     *stdlog.Logger   // cached pointer to stdlogger if any; created by StdLogger()
}



// make a async goroutine for doing actual I/O 
func newLogger(ll *Logger) (*Logger, error) {

    oo     := &outch{logch: make(chan string, 64)}
    ll.ch   = oo
    ll.wait = make(chan bool)

    if len(ll.prefix) > 0 {
        ll.flag  |= lPrefix
        ll.prefix = fmt.Sprintf("%s: ", ll.prefix)
    }

    go ll.qrunner()

    return ll, nil
}

func (l *Logger) closeCh() (r uint32) {
    l.ch.Lock()
    if r = atomic.SwapUint32(&l.ch.closed, 1); r == 0 {
        close(l.ch.logch)
    }
    l.ch.Unlock()

    return r
}


// Close the logger and wait for I/O to complete
func (l *Logger) Close() {
    if 0 != (l.flag & lSublog) { return }

    if z := l.closeCh(); z == 0 {
        //fmt.Printf("## Closing Logger; closed=%v\n", l.ch.closed)
        _, _ = <- l.wait
    }
}


// Creates a new Logger instance. The 'out' variable sets the
// destination to which log data will be written.
// The prefix appears at the beginning of each generated log line.
// The flag argument defines the logging properties.
func New(out io.Writer, prio Priority, prefix string, flag int) (*Logger, error) {
    return newLogger(&Logger{out: out, prio: prio, prefix: prefix, flag: flag})
}


// Create a new Sub-Logger with a different prefix and priority.
// This is useful when different components in a large program want
// their own log-prefix (for easier debugging)
func (l *Logger) New(prefix string, prio Priority) *Logger {

    if prio == LOG_NONE { prio = l.prio }

    nl := &Logger{out: l.out, prio: prio, flag: l.flag, ch: l.ch}

    if len(prefix) > 0 {
        if (l.flag & lPrefix) != 0 {
            n := len(l.prefix)
            oldpref  := l.prefix[:n-2]
            nl.prefix = fmt.Sprintf("%s-%s: ", oldpref, prefix)
        } else {
            nl.prefix = fmt.Sprintf("%s: ", prefix)
            nl.flag  |= lPrefix
        }
    }

    nl.flag |= lSublog
    return nl
}


// Open a new file logger to write logs to 'file'.
// This function erases the previous file contents. This is the only
// constructor that allows you to subsequently configure a log-rotator.
func NewFilelog(file string, prio Priority, prefix string, flag int) (*Logger, error) {
    flag &= ^(lSyslog|lPrefix|lClose)

    // We use O_RDWR because we will likely rotate the file and it
    // will help us to seek(0) and read the logs for purposes of
    // compressing it.
    logfd, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_SYNC, 0600)
    if err != nil {
        s := fmt.Sprintf("Can't open log file '%s': %s", file, err)
        return nil, errors.New(s)
    }

    l := &Logger{out: logfd, prio: prio, prefix: prefix, flag: flag|lClose, name: file}
    return newLogger(l)
}


// Open a new syslog logger.
// XXX What happens on Win32?
func NewSyslog(prio Priority, prefix string, flag int) (*Logger, error) {
    flag &= ^(lSyslog|lPrefix|lClose)

    wr, err := syslog.New(syslog.LOG_NOTICE|syslog.LOG_USER, "")
    if err != nil {
        s := fmt.Sprintf("Can't open SYSLOG connection: %s", err)
        return nil, errors.New(s)
    }

    return newLogger(&Logger{out: wr, prio: prio, prefix: prefix, flag: flag|lSyslog})
}


// Create a new file logger or syslog logger
func NewLogger(name string, prio Priority, prefix string, flag int) (*Logger, error) {

    flag &= ^(lSyslog|lPrefix|lClose)
    switch strings.ToUpper(name) {
        case "SYSLOG":
            return NewSyslog(prio, prefix, flag)

        case "STDOUT":
            return New(os.Stdout, prio, prefix, flag)

        case "STDERR":
            return New(os.Stderr, prio, prefix, flag)

        default:
            return NewFilelog(name, prio, "", flag)
    }
}


// Enable log rotation to happen every day at 'hh:mm:ss' (24-hour
// representation); keep upto 'max' previous logs. Rotated logs are
// gzip-compressed.
func (l *Logger) EnableRotation(hh, mm, ss int, max int) error {
    l.mu.Lock()
    defer l.mu.Unlock()

    if (l.flag & lClose) == 0 { return fmt.Errorf("logger is not file backed") }

    if hh < 0 || hh > 23 || mm < 0 || mm > 59 || ss < 0 || ss > 59 {
        return fmt.Errorf("invalid rotation config %d:%d.%d", hh, mm, ss)
    }


    n := time.Now().UTC()

    // This is the time for next file-rotation
    x := time.Date(n.Year(), n.Month(), n.Day(), hh, mm, ss, 0, n.Location())

    // For debugging log-rotate logic
    //x  = n.Add(2 * time.Minute)

    // If we somehow ended up in "yesterday", then set the reminder
    // for the "next day"
    if x.Before(n) {
        x = x.Add(24 * time.Hour)
    }

    if max <= 0 { max = MAX_LOGFILES }

    /*
    l.directWrite(0, LOG_INFO,
                  fmt.Sprintf("logger: enabling daily log-rotation (keep %d days); first rotate at %s",
                                 max, x.Format(time.RFC822Z)))
     */
     l.Info("logger: enabling daily log-rotation (keep %d days); first rotate at %s",
             max, x.Format(time.RFC822Z))

    l.rot_tm = x
    l.rot_n  = max
    l.flag |= lRotate

    return nil
}



// Cheap integer to fixed-width decimal ASCII.  Give a negative width to avoid zero-padding.
// Knows the buffer has capacity.
func itoa(i int, wid int) string {
    var u uint = uint(i)
    if u == 0 && wid <= 1 {
        return "0"
    }

    // Assemble decimal in reverse order.
    var b [32]byte
    bp := len(b)
    for ; u > 0 || wid > 0; u /= 10 {
        bp--
        wid--
        b[bp] = byte(u%10) + '0'
    }

    return string(b[bp:])
}

func (l *Logger) formatHeader(t time.Time) string {
    var s string

    //*buf = append(*buf, l.prefix...)
    if l.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
        if l.flag&Ldate != 0 {
            year, month, day := t.Date()
            s += itoa(year, 4)
            s += "/"
            s += itoa(int(month), 2)
            s += "/"
            s += itoa(day, 2)
        }
        if l.flag&(Ltime|Lmicroseconds) != 0 {
            hour, min, sec := t.Clock()

            s += " "
            s += itoa(hour, 2)
            s += ":"
            s += itoa(min, 2)
            s += ":"
            s += itoa(sec, 2)
            if l.flag&Lmicroseconds != 0 {
                s += "."
                s += itoa(t.Nanosecond()/1e3, 6)
            }
        }
    }
    return s
}

// Output formats the output for a logging event.  The string s contains
// the text to print after the prefix specified by the flags of the
// Logger.  A newline is appended if the last character of s is not
// already a newline.  Calldepth is used to recover the PC and is
// provided for generality, although at the moment on all pre-defined
// paths it will be 2.
func (l *Logger) ofmt(calldepth int, prio Priority, s string) string {
    if len(s) == 0 { return s }

    var buf string

    // Put the timestamp and priority only if we are NOT syslog
    if (l.flag & lSyslog) == 0 {
        now := time.Now().UTC()
        buf  = fmt.Sprintf("<%d>:%s ", prio, l.formatHeader(now))
    }

    if (l.flag & lPrefix) != 0 { buf += l.prefix }

    if calldepth > 0 {
        var file string
        var line int
        var finfo string
        if l.flag&(Lshortfile|Llongfile) != 0 {
            var ok bool
            _, file, line, ok = runtime.Caller(calldepth)
            if !ok {
                file = "???"
                line = 0
            }
            if l.flag&Lshortfile != 0 {
                short := file
                for i := len(file) - 1; i > 0; i-- {
                    if file[i] == '/' {
                        short = file[i+1:]
                        break
                    }
                }
                file = short
            }
            finfo = fmt.Sprintf("(%s:%d) ", file, line)
        }

        if len(finfo) > 0 {
            buf += finfo
        }
    }


    //buf = append(buf, fmt.Sprintf(":<%d>: ", prio)...)
    buf += s
    if s[len(s)-1] != '\n' {
        buf += "\n"
    }

    return buf
}


// Enqueue a write to be flushed by qrunner()
func (l *Logger) qwrite(s string) {
    // NB: close(ch) happens under the lock. Therefore, any writes
    // to ch must also be under the same lock. Thus, z == 0 tells
    // us that the channel is alive and kicking, and therefore, we can shove
    // some items to it.
    l.ch.Lock()
    if z := atomic.LoadUint32(&l.ch.closed); z == 0 {
        l.ch.logch <- s
    }
    l.ch.Unlock()
}


// Enqueue a log-write to happen asynchronously
func (l *Logger)  Output(calldepth int, prio Priority, s string) {
    if calldepth > 0 { calldepth += 1 }
    t := l.ofmt(calldepth, prio, s)

    l.qwrite(t)
}


// Write to the underlying FD directly; INTERNAL USE ONLY
func (l *Logger) directWrite(calldepth int, prio Priority, s string) {
    if calldepth > 0 { calldepth += 1 }
    t := l.ofmt(calldepth, prio, s)
    l.out.Write([]byte(t))
}


// Dump stack backtrace for 'depth' levels
// Backtrace is of the form "file:line [func name]"
// NB: The absolute pathname of the file is used in the backtrace -
//     regardless of the logger flags requesting shortfile.
func (l* Logger) Backtrace(depth int) {
    var pc []uintptr = make([]uintptr, 64)
    var v  []string

    // runtime.Callers() requires a pre-created array.
    n := runtime.Callers(3, pc)

    if depth == 0 || depth > n {
        depth = n
    } else if n > depth {
        n = depth
    }

    for i := 0; i < n; i++ {
        var s string = "*unknown*"
        p := pc[i]
        f := runtime.FuncForPC(p)

        if f != nil {
            nm := f.Name()
            file, line := f.FileLine(p)
            s = fmt.Sprintf("%s:%d [%s]", file, line, nm)
        }
        v = append(v, s)
    }
    v = append(v, "\n")

    str := "Backtrace:\n    " + strings.Join(v, "\n    ")
    if z := atomic.LoadUint32(&l.ch.closed); z == 0 {
        l.ch.logch <- str
    }
}


// Predicate that returns true if we can log at level prio
func (l* Logger) Loggable(prio Priority) bool {
    return l.prio >= LOG_NONE && prio  >= l.prio
}


// Printf calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Printf(format string, v ...interface{}) {
    l.Output(0, LOG_INFO, fmt.Sprintf(format, v...))
}

// Print calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Print.
func (l *Logger) Print(v ...interface{}) {
    l.Output(0, LOG_INFO, fmt.Sprint(v...))
}


// Fatalf is equivalent to l.Printf() followed by a call to os.Exit(1).
func (l *Logger) Fatal(format string, v ...interface{}) {
    l.Output(2, LOG_EMERG, fmt.Sprintf(format, v...))
    l.Backtrace(0)
    os.Exit(1)
}


// Panicf is equivalent to l.Printf() followed by a call to panic().
func (l *Logger) Panic(format string, v ...interface{}) {
    s := fmt.Sprintf(format, v...)
    l.Output(2, LOG_EMERG, s)
    l.Backtrace(5)
    panic(s)
}



// Crit prints logs at level CRIT
func (l *Logger) Crit(format string, v ...interface{}) {
    if l.Loggable(LOG_CRIT) {
        s := fmt.Sprintf(format, v...)
        l.Output(2, LOG_CRIT, s)
    }
}


// Err prints logs at level ERR
func (l *Logger) Error(format string, v ...interface{}) {
    if l.Loggable(LOG_ERR) {
        s := fmt.Sprintf(format, v...)
        l.Output(2, LOG_ERR, s)
    }
}

// Warn prints logs at level WARNING
func (l *Logger) Warn(format string, v ...interface{}) {
    if l.Loggable(LOG_WARNING) {
        s := fmt.Sprintf(format, v...)
        l.Output(0, LOG_WARNING, s)
    }
}


// Info prints logs at level INFO
func (l *Logger) Info(format string, v ...interface{}) {
    if l.Loggable(LOG_INFO) {
        s := fmt.Sprintf(format, v...)
        l.Output(0, LOG_INFO, s)
    }
}


// Debug prints logs at level INFO
func (l *Logger) Debug(format string, v ...interface{}) {
    if l.Loggable(LOG_DEBUG) {
        s := fmt.Sprintf(format, v...)
        l.Output(2, LOG_DEBUG, s)
    }
}


// Manipulate properties of loggers


// Return priority of this logger
func (l* Logger) Prio() Priority {
    l.mu.Lock()
    defer l.mu.Unlock()
    return l.prio
}

// Set priority
func (l* Logger) SetPrio(prio Priority) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.prio = prio
}

// Flags returns the output flags for the logger.
func (l *Logger) Flags() int {
    l.mu.Lock()
    defer l.mu.Unlock()
    return l.flag
}

// SetFlags sets the output flags for the logger.
func (l *Logger) SetFlags(flag int) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.flag = flag
}

// Prefix returns the output prefix for the logger.
func (l *Logger) Prefix() string {
    l.mu.Lock()
    defer l.mu.Unlock()
    return l.prefix
}

// SetPrefix sets the output prefix for the logger.
func (l *Logger) SetPrefix(prefix string) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.prefix = prefix
}


// -- Internal functions --



// Go routine to do async log writes
func (l *Logger) qrunner() {

    for s := range l.ch.logch {
        if 0 != (l.flag & lRotate) {
            n := time.Now().UTC()
            d := l.rot_tm.Sub(n)
            //l.directWrite(0, LOG_DEBUG, fmt.Sprintf("rot: now=%s, delta=%s", n, d))
            if d < 0 {
                l.rotateLog()

                // Set next rotation for +24 hours
                l.rot_tm = n.Add(24 * time.Hour)
                //l.rot_tm = n.Add(2 * time.Minute)

                l.directWrite(0, LOG_INFO,
                              fmt.Sprintf("Log rotation complete. Next rotate at %s..",
                              l.rot_tm.Format(time.RFC822Z)))
            }
        }

        l.out.Write([]byte(s))
    }

    if (l.flag & lClose) != 0 {
       if fd, ok := l.out.(io.WriteCloser); ok {
           fd.Close()
       }
   }

    close(l.wait)
}


// Rotate current file out
func (l *Logger) rotateLog() {
    //fmt.Printf("Logger: Compressing & Rotating file %s ..\n", l.name)

    fd, ok := l.out.(*os.File)
    if !ok { panic("logger: rotatelog wants a file - but seems to be corrupted") }

    fd.Sync()
    fd.Seek(0, 0)

    // First rotate the older files
    rotatefile(l.name, l.rot_n)

    var gfd *gzip.Writer
    var wfd *os.File
    var err error

    // Now, compress the current file and store it
    gz       := fmt.Sprintf("%s.0.gz", l.name)
    gztmp    := fmt.Sprintf("%s.%v", l.name, rand64())
    wfd, err  = os.OpenFile(gztmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)

    if err != nil {
        fmt.Fprintf(os.Stderr, "Can't create %s for log rotation: %s", gztmp, err)
        goto fail
    }


    gfd, err = gzip.NewWriterLevel(wfd, 9)
    if err != nil {
        fmt.Fprintf(os.Stderr, "can't initialize gzip %s for log rotation: %s", gztmp, err)
        goto fail1
    }

    _, err = io.Copy(gfd, fd)
    if err != nil {
        fmt.Fprintf(os.Stderr, "can't write gzip %s for log rotation: %s", gztmp, err)
        goto fail1
    }

    err = gfd.Close()
    if err != nil {
        fmt.Fprintf(os.Stderr, "can't write gzip %s for log rotation: %s", gztmp, err)
        goto fail1
    }

    wfd.Close()
    os.Rename(gztmp, gz)

    //fmt.Printf("(re)opening old logfile %s..\n", l.name)
    fd.Truncate(0)
    fd.Seek(0, 0)

    return


fail1:
    wfd.Close()
    os.Remove(gztmp)

    // XXX This is a horrible sequence of things that follows
fail:
    fd.Close()
    l.out = os.Stderr
    fmt.Fprintf(os.Stderr, "Switching logging to Stderr...")
    l.flag &= ^lClose
    return
}


// Rotate files of the form fn.NN where 0 <= NN < max
// Delete the oldest file (NN == max-1)
func rotatefile(fn string, max int) {

    old := fmt.Sprintf("%s.%d.gz", fn, max-1)
    os.Remove(old)

    // Now, we iterate from max-1 to 0
    for i := max-1; i > 0; i -= 1 {
        older := old
        old    = fmt.Sprintf("%s.%d.gz", fn, i-1)
        if exists(old) {
            os.Rename(old, older)
        }
    }
}


// Predicate - returns true if file 'fn' exists; false otherwise
func exists(fn string) bool {
    _, err := os.Stat(fn)
    if os.IsNotExist(err) { return false }

    // XXX Should we check for IsRegular() ?
    return true
}


// 64 bit random integer
func rand64() uint64 {
    var b [8]byte
    rand.Read(b[:])
    return binary.BigEndian.Uint64(b[:])
}

