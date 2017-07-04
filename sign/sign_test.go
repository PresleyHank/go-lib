// sign_test.go -- Test harness for sign
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

package sign

import (
    "os"
    "fmt"
    "io/ioutil"
    "testing"
    "runtime"
    "crypto/rand"
    "crypto/subtle"

    // module under test
    //"github.com/sign"
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

    t.Logf("Tempdir is %s", tmp)
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


const badsk string = `
esk: q8AP3/6C5F0zB8CLiuJsidx2gJYmrnyOmuoazEbKL5Uh+Jn/Zgw85fTbYfhjcbt48CJejBzsgPYRYR7wWECFRA==
salt: uIdTQZotfnkaLkth9jsHvoQKMWdNZuE7dgVNADrRoeY=
algo: scrypt-sha256
verify: AOFLLC6h29+mvstWtMU1/zZFwHLBMMiI4mlW9DHpYdM=
Z: 65536
r: 8
p: 1
`


// #1. Create new key pair, and read them back.
func Test0(t *testing.T) {
    kp, err := NewKeypair()
    assert(err == nil, t, "NewKeyPair() fail")

    dn := tempdir(t)
    bn := fmt.Sprintf("%s/t0", dn)

    err = kp.Serialize(bn, "", "abc")
    assert(err == nil, t, "keyPair.Serialize() fail")

    pkf := fmt.Sprintf("%s.pub", bn)
    skf := fmt.Sprintf("%s.key", bn)

    // We must find these two files
    assert(fileExists(pkf), t, "missing pkf")
    assert(fileExists(skf), t, "missing skf")

    // send wrong file and see what happens
    pk, err := ReadPublicKey(skf)
    assert(err != nil, t, "bad PK ReadPK fail")

    pk, err = ReadPublicKey(pkf)
    assert(err == nil, t, "ReadPK() fail")

    // -ditto- for Sk
    sk, err := ReadPrivateKey(pkf, "")
    assert(err != nil, t, "bad SK ReadSK fail")

    sk, err = ReadPrivateKey(skf, "")
    assert(err != nil, t, "ReadSK() empty pw fail")

    sk, err  = ReadPrivateKey(skf, "abcdef")
    assert(err != nil, t, "ReadSK() wrong pw fail")

    badf := fmt.Sprintf("%s/badf.key", dn)
    err   = ioutil.WriteFile(badf, []byte(badsk), 0600)
    assert(err == nil, t, "write badsk")

    sk, err = ReadPrivateKey(badf, "abc")
    assert(err != nil, t, "badsk read fail")

    // Finally, with correct password it should work.
    sk, err  = ReadPrivateKey(skf, "abc")
    assert(err == nil, t, "ReadSK() correct pw fail")

    // And, deserialized keys should be identical
    assert(byteEq(pk.Pk, kp.Pub.Pk), t, "pkbytes unequal")
    assert(byteEq(sk.Sk, kp.Sec.Sk), t, "skbytes unequal")

    os.RemoveAll(dn)
}


// #2. Create new key pair, sign a rand buffer and verify
func Test1(t *testing.T) {
    kp, err := NewKeypair()
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

    // Corrupt the pkhash and see
    rand.Read(ss.Pkhash[:])
    assert(!ss.IsPKMatch(pk), t, "corrupt pk match fail")

    // Incorrect checksum == should fail verification
    ok, err := pk.VerifyMessage(ck[:16], ss)
    assert(err == nil, t, "bad ck verify err fail")
    assert(!ok, t, "bad ck verify fail")

    // proper checksum == should work
    ok, err = pk.VerifyMessage(ck[:], ss)
    assert(err == nil, t, "verify err")
    assert(ok, t, "verify fail")


    // Now sign a file
    dn := tempdir(t)
    bn := fmt.Sprintf("%s/k", dn)

    pkf := fmt.Sprintf("%s.pub", bn)
    skf := fmt.Sprintf("%s.key", bn)

    err = kp.Serialize(bn, "", "")
    assert(err == nil, t, "keyPair.Serialize() fail")

    // Now read the private key and sign
    sk, err = ReadPrivateKey(skf, "")
    assert(err == nil, t, "readSK fail")

    pk, err = ReadPublicKey(pkf)
    assert(err == nil, t, "ReadPK fail")

    var buf [8192]byte

    zf := fmt.Sprintf("%s/file.dat", dn)
    fd, err := os.OpenFile(zf, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    assert(err == nil, t, "file.dat creat file")

    for i := 0; i < 8; i++ {
        rand.Read(buf[:])
        n, err := fd.Write(buf[:])
        assert(err == nil, t, fmt.Sprintf("file.dat write fail: %s", err))
        assert(n == 8192, t, fmt.Sprintf("file.dat i/o fail: exp 8192 saw %v", n))
    }
    fd.Sync()
    fd.Close()

    sig, err := sk.SignFile(zf)
    assert(err == nil, t, "file.dat sign fail")
    assert(sig != nil, t, "file.dat sign nil")


    ok, err  = pk.VerifyFile(zf, sig)
    assert(err == nil, t, "file.dat verify fail")
    assert(ok,         t, "file.dat verify false")


    // Now, serialize the signature and read it back
    sf := fmt.Sprintf("%s/file.sig", dn)
    err = sig.SerializeFile(sf, "")
    assert(err == nil, t, "sig serialize fail")


    s2, err := ReadSignature(sf)
    assert(err == nil, t, "file.sig read fail")
    assert(s2  != nil, t, "file.sig sig nil")

    assert(byteEq(s2.Sig, sig.Sig), t, "sig compare fail")

    // If we give a wrong file, verify must fail
    st, err := os.Stat(zf)
    assert(err == nil, t, "file.dat stat fail")
    
    n := st.Size();
    assert(n == 8192 * 8, t, "file.dat size fail")

    os.Truncate(zf, n-1)

    st, err = os.Stat(zf)
    assert(err == nil, t, "file.dat stat2 fail")
    assert(st.Size() == (n-1), t, "truncate fail")

    // Now verify this corrupt file
    ok, err = pk.VerifyFile(zf, sig)
    assert(err == nil, t, "file.dat corrupt i/o fail")
    assert(!ok,        t, "file.dat corrupt verify false")

    os.RemoveAll(dn)
}


