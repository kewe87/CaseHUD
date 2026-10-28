package update

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// The release signing key (same as docs/release-signing.pub.pem; a test keeps
// both equal). Only signatures by its private key are accepted.
//
//go:embed release-signing.pub.pem
var publicKeyPEM []byte

// signContext is prepended to SHA256SUMS.txt before signing, so a signature
// made for something else never verifies as a release.
const signContext = "casehud-release-v1\n"

// KeyFingerprint is the SHA-256 of the key's SubjectPublicKeyInfo (DER).
const KeyFingerprint = "cfb6863f23c892a53c93786dd1ee78478618aed680c5603fbc3955c1dde4abaf"

func releaseKey() (*ecdsa.PublicKey, error) {
	b, _ := pem.Decode(publicKeyPEM)
	if b == nil {
		return nil, errors.New("release key: no PEM block")
	}
	if sum := sha256.Sum256(b.Bytes); hex.EncodeToString(sum[:]) != KeyFingerprint {
		return nil, errors.New("release key: fingerprint mismatch")
	}
	k, err := x509.ParsePKIXPublicKey(b.Bytes)
	if err != nil {
		return nil, err
	}
	ek, ok := k.(*ecdsa.PublicKey)
	if !ok || ek.Curve.Params().Name != "P-256" {
		return nil, errors.New("release key: not ECDSA P-256")
	}
	return ek, nil
}

// VerifySums checks sig (base64 of an ASN.1 DER ECDSA signature, as written
// by scripts/sign-release.ps1) over SHA256SUMS.txt.
func VerifySums(sums, sig []byte) error {
	key, err := releaseKey()
	if err != nil {
		return err
	}
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil {
		return fmt.Errorf("signature: %w", err)
	}
	h := sha256.New()
	h.Write([]byte(signContext))
	h.Write(sums)
	if !ecdsa.VerifyASN1(key, h.Sum(nil), der) {
		return errors.New("signature does not match the release key")
	}
	return nil
}

// SumFor returns the SHA-256 that SHA256SUMS.txt lists for name
// ("<hex>  <name>" or "<hex> *<name>" per line). Exactly one line must name it.
func SumFor(sums []byte, name string) ([32]byte, error) {
	var out [32]byte
	found := 0
	for _, line := range bytes.Split(sums, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		hexPart, rest, ok := bytes.Cut(line, []byte(" "))
		if !ok {
			continue
		}
		file := strings.TrimPrefix(strings.TrimPrefix(string(rest), " "), "*")
		if file != name {
			continue
		}
		b, err := hex.DecodeString(string(hexPart))
		if err != nil || len(b) != 32 {
			return out, fmt.Errorf("SHA256SUMS: bad digest for %s", name)
		}
		copy(out[:], b)
		found++
	}
	switch found {
	case 0:
		return out, fmt.Errorf("SHA256SUMS lists no %s", name)
	case 1:
		return out, nil
	default:
		return out, fmt.Errorf("SHA256SUMS lists %s more than once", name)
	}
}
