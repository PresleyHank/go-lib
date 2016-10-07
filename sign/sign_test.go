// sign_test.go -- Test harness for android/pkg
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

package sign_test

import (
    "os"
    "fmt"
    "testing"
    "runtime"
    "crypto/rand"
    "crypto/subtle"

    // module under test
    "sign"
)


func assert(cond bool, t *testing.T, msg string) {

    if cond { return }

    _, file, line, ok := runtime.Caller(1)
    if !ok {
        file = "???"
        line = 0
    }

    t.Fatalf("%s: %d: Assertion failed: %q\n", file, line, msg)
}

// Return true if two byte arrays are equal
func byteEq(x, y []byte) bool {
    return subtle.ConstantTimeCompare(x, y) == 1
}

// Return a temp dir in a temp-dir
func tempdir(t *testing.T) string {
    var b [10]byte

    //dn := os.TempDir()
    dn := "/tmp"
    rand.Read(b[:])

    tmp := fmt.Sprintf("%s/%x", dn, b[:])
    err := os.MkdirAll(tmp, 0755)
    assert(err == nil, t, fmt.Sprintf("mkdir -p %s: %s", tmp, err))

    return tmp
}

// Return true if file exists, false otherwise
func fileExists(fn string) bool {
    st, err := os.Stat(fn)
    if err != nil {
        if os.IsNotExist(err) { return false }
        return false
    }

    if st.Mode().IsRegular() { return true }
    return false
}


// #1. Create new key pair, and read them back.
func Test0(t *testing.T) {
    kp, err := sign.NewKeypair()
    assert(err == nil, t, "NewKeyPair() fail")

    dn := tempdir(t)
    bn := fmt.Sprintf("%s/t0", dn)

    t.Logf("Tempdir is %s", dn)

    err = kp.Serialize(bn, "", "abc")
    assert(err == nil, t, "keyPair.Serialize() fail")

    pkf := fmt.Sprintf("%s.pub", bn)
    skf := fmt.Sprintf("%s.key", bn)

    // We must find these two files
    assert(fileExists(pkf), t, "missing pkf")
    assert(fileExists(skf), t, "missing skf")

    // send wrong file and see what happens
    pk, err := sign.ReadPublicKey(skf)
    assert(err != nil, t, "bad PK ReadPK fail")

    pk, err = sign.ReadPublicKey(pkf)
    assert(err == nil, t, "ReadPK() fail")

    // -ditto- for Sk
    sk, err := sign.ReadPrivateKey(pkf, "")
    assert(err != nil, t, "bad SK ReadSK fail")

    sk, err = sign.ReadPrivateKey(skf, "")
    assert(err != nil, t, "ReadSK() empty pw fail")

    sk, err  = sign.ReadPrivateKey(skf, "abcdef")
    assert(err != nil, t, "ReadSK() wrong pw fail")

    // Finally, with correct password it should work.
    sk, err  = sign.ReadPrivateKey(skf, "abc")
    assert(err == nil, t, "ReadSK() correct pw fail")

    // And, deserialized keys should be identical
    assert(byteEq(pk.Pk, kp.Pub.Pk), t, "pkbytes unequal")
    assert(byteEq(sk.Sk, kp.Sec.Sk), t, "skbytes unequal")

    os.RemoveAll(dn)
}


// #2. Create new key pair, sign a rand buffer and verify
func Test1(t *testing.T) {
    kp, err := sign.NewKeypair()
    assert(err == nil, t, "NewKeyPair() fail")

    var ck [64]byte        // simulates sha512 sum

    rand.Read(ck[:])

    pk := &kp.Pub
    sk := &kp.Sec

    ss, err := sk.SignMessage(ck[:], "")
    assert(err == nil, t, "sk.sign fail")
    assert(ss  != nil, t, "sig is null")

    // verify sig
    assert(ss.IsPKMatch(pk), t, "pk match fail")

    // Incorrect checksum == should fail verification
    ok, err := pk.VerifyMessage(ck[:16], ss)
    assert(err == nil, t, "bad ck verify err fail")
    assert(!ok, t, "bad ck verify fail")

    // proper checksum == should work
    ok, err = pk.VerifyMessage(ck[:], ss)
    assert(err == nil, t, "verify err")
    assert(ok, t, "verify fail")

}


// #3. Create new key pair, sign a rand buffer, serialize it and
// read it back
func Test2(t *testing.T) {
}
