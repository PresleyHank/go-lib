=================
README for go-lib
=================

This is a small collection of Go libraries that I modified for my
own use. Some I wrote from scratch. I've retained the original
license for modules that I modified.

The code I created is licensed under the terms of GNU General Public
License v2.0.


What Libraries are available?
=============================
- android/pkg   -- Android packages.xml and packages.list parsing

- cdb -- my modifications to a popular CDB library. I added a SHA256
  sum to the end of the generated file to verify its integrity. This
  SHA256 sum is verified when a CDB is opened.

- fasthash - Zi Long Tan's Super fast hash

- logger - Heavily modified version of Golang's ``log`` module:

    * Log writes are asynchronous; i.e., every log output functions
      queues up its output to a channel that is drained
      asynchronously.

    * Support for sub-loggers with a different prefix and Logging priority but
      same output channel.

    * Log to STDOUT, STDERR, SYSLOG or a plain file.

    * Daily Log rotation based on time-of-day. THe logs are by
      default compressed. This is only for loggers which write to
      their own file.

    * Debug logs provide file:line information.

- options- A command line option parsing library that uses the help string to
  construct a parser for its options. Modified from Simon Menke's
  <simon.menke@gmail.com> original version. I've retained Simon's
  original licensing terms.

- ringbuf - Blocking rinbuffer of ``interfaces{}``; useful building
  block for a memory pool of buffers.

- sem - Semaphore implementation that doesn't use too much memory.

- sign - Ed25519 based Signature scheme for files; this is a generic
  library that provides functionality for creating and serializing
  Ed25519 keypairs and signatures. A companion command line tool
  called sigtool_  uses this library to provide a replacement for
  OpenBSD's signify(1).

- util - General purpose utility functions that I couldn't fit
  anywhere else.


.. _sigtool: http://github.com/opencoff/sigtool
