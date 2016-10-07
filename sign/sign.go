// sign.go -- Ed25519 keys and signature handling
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

// Package sign implements Ed25519 signing, verification on files.
// It builds upon golang.org/x/crypto/ed25519 by adding methods
// for serializing and deserializing Ed25519 private & public keys.
// In addition, it works with large files - by precalculating their
// SHA512 checksum in mmap'd mode and sending the 64 byte signature
// for Ed25519 signing.
package sign


import (
    "os"
    "fmt"
    "hash"
    "crypto"
    "syscall"
    "io/ioutil"
    "crypto/rand"
    "crypto/subtle"
    "crypto/sha256"
    "crypto/sha512"
    "encoding/base64"

    "gopkg.in/yaml.v2"
    "golang.org/x/crypto/scrypt"
    Ed "golang.org/x/crypto/ed25519"
    "golang.org/x/crypto/ssh/terminal"
)


// Private Ed25519 key
type PrivateKey struct {
    Sk      []byte

    // Cached copy of the public key
    // In reality, it is a pointer to Sk[32:]
    pk      []byte
}


// Public Ed25519 key
type PublicKey struct {
    Pk      []byte
}


// Ed25519 key pair
type Keypair struct {
    Sec     PrivateKey
    Pub     PublicKey
}


// An Ed25519 Signature
type Signature struct {
    Sig     []byte      // 32 byte digital signature
    Pkhash  []byte      // [0:16] SHA256 hash of public key needed for verification
}


// Algorithm used in the encrypted private key
const sk_algo  = "scrypt-sha256"
const sig_algo = "sha512-ed25519"

// Scrypt parameters
const _N = 1 << 17
const _r = 16
const _p = 1

// Encrypted Private key
type encPrivKey struct {
    // Encrypted Sk
    Esk     []byte

    // parameters for Sk serialization
    Salt    []byte

    // Algorithm used for checksum and KDF
    Algo    string

    // Checksum to verify passphrase before we xor it
    Verify  []byte

    // These are params for scrypt.Key()
    // CPU Cost parameter; must be a power of 2
    N       uint32
    // r * p should be less than 2^30
    r       uint32
    p       uint32
}


// Serialized representation of private key
type serializedPrivKey struct {
    Comment string      `yaml:"comment,omitempty"`
    Esk     string      `yaml:"esk"`
    Salt    string      `yaml:"salt,omitempty"`
    Algo    string      `yaml:"algo,omitempty"`
    Verify  string      `yaml:"verify,omitempty"`
    N       uint32      `yaml:"Z,flow,omitempty"`
    R       uint32      `yaml:"r,flow,omitempty"`
    P       uint32      `yaml:"p,flow,omitempty"`
}


// serialized representation of public key
type serializedPubKey struct {
    Comment string      `yaml:"comment,omitempty"`
    Pk      string      `yaml:"pk"`
}


// Serialized signature
type signature struct {
    Comment     string  `yaml:"comment,omitempty"`
    Pkhash      string  `yaml:"pkhash,omitempty"`
    Signature   string  `yaml:"signature"`
}




// Generate a new Ed25519 keypair
func NewKeypair() (*Keypair, error) {
    //kp := &Keypair{Sec: PrivateKey{N: 1 << 17, r: 64, p: 1}}
    kp := &Keypair{}
    sk := &kp.Sec
    pk := &kp.Pub

    // Fuck. Why can't they use simple types!
    p, s, err := Ed.GenerateKey(rand.Reader)
    if err != nil { return nil, fmt.Errorf("Can't generate Ed25519 keys: %s", err) }


    pk.Pk = []byte(p)
    sk.Sk = []byte(s)
    sk.pk = pk.Pk

    return kp, nil
}


// Serialize the keypair to two separate files. The basename of the
// file is 'bn'; the public key goes in $bn.pub and the private key
// goes in $bn.key.
// If password is non-empty, then the private key is encrypted
// before writing to disk.
func (kp *Keypair) Serialize(bn, comment string, pw string) error {

    sk := &kp.Sec
    pk := &kp.Pub

    skf := fmt.Sprintf("%s.key", bn)
    pkf := fmt.Sprintf("%s.pub", bn)

    err := pk.serialize(pkf, comment)
    if err != nil { return fmt.Errorf("Can't serialize to %s: %s", pkf, err) }

    err = sk.serialize(skf, comment, pw)
    if err != nil { return fmt.Errorf("Can't serialize to %s: %s", pkf, err) }

    return nil
}



// Read the private key in 'fn', optionally decrypting it using
// password 'pw' and create new instance of PrivateKey
func ReadPrivateKey(fn string, pw string) (*PrivateKey, error) {
    yml, err := ioutil.ReadFile(fn)
    if err != nil { return nil, err }

    var ssk serializedPrivKey

    err = yaml.Unmarshal(yml, &ssk)
    if err != nil { return nil, fmt.Errorf("can't parse YAML in %s: %s", fn, err) }

    esk := &encPrivKey{N: ssk.N, r: ssk.R, p: ssk.P, Algo: ssk.Algo}
    b64 := base64.StdEncoding.DecodeString

    esk.Esk,  err = b64(ssk.Esk)
    if        err != nil { return nil, fmt.Errorf("can't decode YAML:Esk in %s: %s", fn, err) }

    esk.Salt, err = b64(ssk.Salt)
    if        err != nil { return nil, fmt.Errorf("can't decode YAML:Salt in %s: %s", fn, err) }

    esk.Verify, err = b64(ssk.Verify)
    if          err != nil { return nil, fmt.Errorf("can't decode YAML:Verify in %s: %s", fn, err) }

    sk := &PrivateKey{}

    // We take short passwords and extend them
    pwb := sha512.Sum512([]byte(pw))

    xork, err := scrypt.Key(pwb[:], esk.Salt, int(esk.N), int(esk.r), int(esk.p), len(esk.Esk))
    if    err != nil { return nil, fmt.Errorf("can't derive key in %s: %s", fn, err) }

    hh := sha256.New()
    hh.Write(esk.Salt)
    hh.Write(xork)
    ck := hh.Sum(nil)

    if subtle.ConstantTimeCompare(esk.Verify, ck) != 1 {
        return nil, fmt.Errorf("incorrect password for %s", fn)
    }

    // Everything works. Now, decode the key
    sk.Sk = make([]byte, len(esk.Esk))
    for i:=0; i < len(esk.Esk); i++ {
        sk.Sk[i] = esk.Esk[i] ^ xork[i]
    }

    return sk, nil
}




// Serialize the private key to a file
// Format: YAML
// All []byte are in base64 (RawEncoding)
func (sk *PrivateKey) serialize(fn, comment string, pw string) error {

    b64 := base64.StdEncoding.EncodeToString
    esk := &encPrivKey{}
    ssk := &serializedPrivKey{Comment: comment}

    // Even with an empty password, we still encrypt and store.

    // expand the password into 64 bytes
    pwb := sha512.Sum512([]byte(pw))

    esk.N    = _N
    esk.r    = _r
    esk.p    = _p

    esk.Salt = make([]byte, 32)
    esk.Esk  = make([]byte, len(sk.Sk))

    _, err := rand.Read(esk.Salt)
    if err != nil { return fmt.Errorf("Can't read random salt: %s", err) }

    xork, err := scrypt.Key(pwb[:], esk.Salt, int(esk.N), int(esk.r), int(esk.p), len(sk.Sk))
    if err != nil { return fmt.Errorf("Can't derive scrypt key: %s", err) }

    hh := sha256.New()
    hh.Write(esk.Salt)
    hh.Write(xork)
    esk.Verify = hh.Sum(nil)

    esk.Algo = sk_algo     // global var

    // Finally setup the encrypted key
    for i:=0; i < len(sk.Sk); i++ {
        esk.Esk[i] = sk.Sk[i] ^ xork[i]
    }


    ssk.Esk    = b64(esk.Esk)
    ssk.Salt   = b64(esk.Salt)
    ssk.Verify = b64(esk.Verify)
    ssk.Algo   = esk.Algo
    ssk.N      = esk.N
    ssk.R      = esk.r
    ssk.P      = esk.p

    out, err := yaml.Marshal(ssk)
    if err   != nil { return fmt.Errorf("can't marahal to YAML: %s", err) }

    return writeFile(fn, out, 0600)
}




// Sign a prehashed Message; return the signature as opaque bytes
// Signature is an YAML file:
//    Comment: source file path
//    Signature: Ed25519 signature
func (sk *PrivateKey) SignMessage(ck []byte, comment string) (*Signature, error) {
    x := Ed.PrivateKey(sk.Sk)

    sig, err := x.Sign(rand.Reader, ck, crypto.Hash(0))
    if err   != nil {
        return nil, fmt.Errorf("can't sign %x: %s", ck, err)
    }

    pkh := sha256.Sum256(sk.pk)

    return &Signature{Sig: sig, Pkhash: pkh[:16]}, nil
}



// Read and sign a file
//
// We calculate the signature differently here: We first calculate
// the SHA-512 checksum of the file and its size. We sign the
// checksum.
func (sk *PrivateKey) SignFile(fn string) (*Signature, error) {

    ck, err := fileCksum(fn, sha512.New())
    if err  != nil { return nil, err }

    return sk.SignMessage(ck, fn)
}


// -- Signature Methods --


// Read serialized signature from file 'fn' and construct a
// Signature object
func ReadSignature(fn string) (*Signature, error) {
    yml, err := ioutil.ReadFile(fn)
    if err   != nil { return nil, err }

    return MakeSignature(yml)
}


// Parse serialized signature from bytes 'b' and construct a
// Signature object
func MakeSignature(b []byte) (*Signature, error) {
    var ss signature
    err    := yaml.Unmarshal(b, &ss)
    if err != nil { return nil, fmt.Errorf("can't parse YAML signature: %s", err) }

    b64    := base64.StdEncoding.DecodeString

    s, err := b64(ss.Signature)
    if err != nil {
        return nil, fmt.Errorf("can't decode Base64:Signature <%s>: %s", ss.Signature, err)
    }

    p, err := b64(ss.Pkhash)
    if err != nil {
        return nil, fmt.Errorf("can't decode Base64:Pkhash <%s>: %s", ss.Pkhash, err)
    }
    
    return &Signature{Sig: s, Pkhash: p}, nil
}



// Serialize a signature suitiable for storing in durable media
func (sig *Signature) Serialize(comment string) ([]byte, error) {

    sigs := base64.StdEncoding.EncodeToString(sig.Sig)
    pks  := base64.StdEncoding.EncodeToString(sig.Pkhash)
    ss   := &signature{Comment: comment, Pkhash: pks, Signature: sigs}

    out, err := yaml.Marshal(ss)
    if err != nil {
        return nil, fmt.Errorf("can't marshal signature of %x to YAML: %s", sig.Sig, err)
    }

    return out, nil
}


// Serialize signature to an output file 'f'
func (sig *Signature) SerializeFile(fn, comment string) error {
    b, err := sig.Serialize(comment)
    if err != nil { return err }

    return writeFile(fn, b, 0644)
}



// Return true if a given PK matches the PKhash we have
func (sig *Signature) IsPKMatch(pk *PublicKey) bool {
    h := sha256.Sum256(pk.Pk)
    return subtle.ConstantTimeCompare(h[:16], sig.Pkhash) == 1
}


//  --- Public Key Methods ---


// Read the public key from 'fn' and create new instance of
// PublicKey
func ReadPublicKey(fn string) (*PublicKey, error) {
    var err error
    var yml []byte

    if yml, err = ioutil.ReadFile(fn); err != nil { return nil, err }

    var spk serializedPubKey

    if err = yaml.Unmarshal(yml, &spk); err != nil {
        return nil, fmt.Errorf("can't parse YAML in %s: %s", fn, err)
    }

    pk  := &PublicKey{}
    b64 := base64.StdEncoding.DecodeString

    if pk.Pk, err = b64(spk.Pk); err != nil {
        return nil, fmt.Errorf("can't decode YAML:Pk in %s: %s", fn, err)
    }

    // Simple sanity checks
    if len(pk.Pk) == 0 {
        return nil, fmt.Errorf("public key data is empty?")
    }

    return pk, nil
}


// Serialize Public Keys
func (pk *PublicKey) serialize(fn, comment string) error {
    b64 := base64.StdEncoding.EncodeToString
    spk := &serializedPubKey{Comment: comment}

    spk.Pk     = b64(pk.Pk)

    out, err := yaml.Marshal(spk)
    if err != nil { return fmt.Errorf("Can't marahal to YAML: %s", err) }

    return writeFile(fn, out, 0644)
}




// Verify a signature 'sig' for file 'fn' against public key 'pk'
// Return True if signature matches, False otherwise
func (pk *PublicKey) VerifyFile(fn string, sig *Signature) (bool, error) {

    ck, err := fileCksum(fn, sha512.New())
    if err != nil { return false, err }

    return pk.VerifyMessage(ck, sig)
}


// Verify a signature 'sig' for a pre-calculated checksum 'ck' against public key 'pk'
// Return True if signature matches, False otherwise
func (pk *PublicKey) VerifyMessage(ck []byte, sig *Signature) (bool, error) {

    x := Ed.PublicKey(pk.Pk)
    return Ed.Verify(x, ck, sig.Sig), nil
}




// Utility function: Ask user for an interactive password
// If verify is true, confirm a second time.
// Mistakes during confirmation cause the process to restart upto a
// maximum of 2 times.
func Askpass(prompt string, verify bool) (string, error) {

    for i := 0; i < 2; i++ {
        fmt.Printf("%s: ", prompt)
        pw1, err := terminal.ReadPassword(int(syscall.Stdin))
        fmt.Printf("\n")
        if err != nil { return "", err }
        if !verify { return string(pw1), nil }

        fmt.Printf("%s again: ", prompt)
        pw2, err := terminal.ReadPassword(int(syscall.Stdin))
        fmt.Printf("\n")
        if err != nil { return "", err }

        a := string(pw1)
        b := string(pw2)
        if a == b { return a, nil }
    }

    return "", fmt.Errorf("Too many tries getting password")
}


// -- Internal Utility Functions --

// Unlink a file.
func unlink(f string) {
    st, err := os.Stat(f)
    if err == nil {
        if !st.Mode().IsRegular() { panic(fmt.Sprintf("%s can't be unlinked. Not a regular file?", f)) }

        os.Remove(f)
        return
    }

    // XXX Go has no easy way to do os.file.isexist() ala python; it has
    //     a confusing fuktard full of err checks all over the place.
    //     Here, err != nil. But, I have no way of knowing if it is
    //     becasue of ENOENT or some other error.
}


// Simple function to reliably write data to a file.
// Does MORE than ioutil.WriteFile() - in that it doesn't trash the
// existing file with an incomplete write.
func writeFile(fn string, b []byte, mode uint32) error {
    tmp := fmt.Sprintf("%s.tmp", fn)
    unlink(tmp)

    fd, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(mode))
    if err != nil { return fmt.Errorf("Can't create key file %s: %s", tmp, err) }

    _, err  = fd.Write(b)
    if err != nil {
        fd.Close()
        return fmt.Errorf("Can't write %v bytes to %s: %s", len(b), tmp, err)
    }

    fd.Close()  // we ignore close(2) errors; unrecoverable anyway.

    os.Rename(tmp, fn)
    return nil
}




// Generate file checksum out of hash function h
// Use mmap(2) to read the file in chunks.
// Here, we use a 2GB chunk size
func fileCksum(fn string, h hash.Hash) ([]byte, error) {
    fd, err := os.Open(fn)
    if err != nil { return nil, fmt.Errorf("can't open %s: %s", fn, err) }
    
    defer fd.Close()

    st, err := fd.Stat()
    if err != nil { return nil, fmt.Errorf("can't stat %s: %s", fn, err) }

    // Mmap'ing large files won't work. We need to do it in 1 or 2G
    // chunks.
    const chunk  int64 = 1 * 1024 * 1024 * 1024

    var off int64
    for sz := st.Size(); sz > 0; {
        var n int

        // XXX Ternary operator was so bloody unreadable? Fuktards.
        if sz > chunk {
            n = int(chunk)
        } else {
            n = int(sz)
        }

        mem, err := syscall.Mmap(int(fd.Fd()), off, n, syscall.PROT_READ, syscall.MAP_SHARED)
        if err != nil { return nil, fmt.Errorf("can't mmap %v bytes of %s at %v: %s", n, fn, off, err) }

        h.Write(mem)
        syscall.Munmap(mem)

        sz  -= int64(n)
        off += int64(n)
    }

    ck := h.Sum(nil)

    return ck, nil
}

// EOF
