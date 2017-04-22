//
// Ratelimiting incoming connections - Small Library
//
// (c) 2013 Sudhi Herle <sudhi-dot-herle-at-gmail-com>
//
// License: GPLv2
//
// Notes:
//  - This is a very simple interface for rate limiting. It
//    implements a token bucket algorithm
//  - Based on Anti Huimaa's very clever token bucket algorithm:
//    http://stackoverflow.com/questions/667508/whats-a-good-rate-limiting-algorithm
//
// Usage:
//    rate = 1000
//    per  = 5
//    rl = ratelimit.New(rate, per) // ratelimit to 1000 every 5 seconds
//
//    ....
//    if rl.Limit() {
//       drop_connection(conn)
//    }
//
package ratelimit

import "time"

type Ratelimiter struct {

    rate  int        // conn/sec
    per   float64    // time interval (seconds)
    last  time.Time  // last time we were polled/asked

    allowance float64
}


// Create new limiter that limits to 'rate' every 'per' seconds
func New(rate, per int) (*Ratelimiter, error) {

    if rate <= 0 { rate = 0 }
    if per <= 0  { per  = 1 }

    r := Ratelimiter{rate:rate,
                    per: float64(per),
                    last:time.Now(),
                    allowance: float64(rate),
         }

    return &r, nil
}


// Return true if the current call exceeds the set rate, false
// otherwise
func (r* Ratelimiter) Limit() bool {

    // handle cases where rate in config file is unset - defaulting
    // to "0" (unlimited)
    if r.rate == 0 {
        return false
    }

    rate        := float64(r.rate)
    now         := time.Now()
    elapsed     := now.Sub(r.last)
    r.last       = now
    r.allowance += float64(elapsed) * (rate / r.per)


    // Clamp number of tokens in the bucket. Don't let it get
    // unboundedly large
    if r.allowance > rate {
        r.allowance = rate
    }

    if r.allowance < 1.0 {
        return true
    }

    r.allowance -= 1.0
    return false
}

