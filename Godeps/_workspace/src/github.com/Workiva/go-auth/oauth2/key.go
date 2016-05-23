package oauth2

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

// jsonKeyMap stores the list of jsonKey's
type jsonKeyMap struct {
	Keys []jsonKey `json:"keys"`
}

// jsonKey stores the base64 encoded N and E for the rsa.PublicKey
type jsonKey struct {
	N string `json:"n"`
	E string `json:"e"`
}

// TokenService provides a service layer for interacting with RSA public keys
type TokenService interface {
	// GetOAuth20TokenKey makes a GET request to the provided URL and
	// unmarshals the returned RSA public key
	GetOAuth20TokenKey(string) (*rsa.PublicKey, error)
}

type tokenService struct {
	client *http.Client
}

// NewTokenService returns new TokenService instance
func NewTokenService(client *http.Client) TokenService {
	return tokenService{client}
}

// GetOAuth20TokenKey makes a GET request to the provided URL and unmarshals
// the returned RSA public key
func (t tokenService) GetOAuth20TokenKey(publicKeyURL string) (*rsa.PublicKey, error) {
	req, _ := http.NewRequest("GET", publicKeyURL, nil)
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Could not get public key")
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respJSON := buf.String()

	// in the Workiva 1.0 endpoint, the cert struct is the only element in an array
	if strings.HasSuffix(publicKeyURL, "/iam/oauth2/v1.0/certs") {
		var respPayload []jsonKeyMap
		err = json.Unmarshal([]byte(respJSON), &respPayload)
		if err != nil {
			return nil, fmt.Errorf("oauth2: failed to unmarshal issuer certificate in Workiva 1.0 mode; error was %s", err)
		}
		key := respPayload[0].Keys[0]
		return generateRSAKeyFromWorkiva10JSONKey(key)
	}

	var respPayload jsonKeyMap
	err = json.Unmarshal([]byte(respJSON), &respPayload)
	if err != nil {
		return nil, fmt.Errorf("oauth2: failed to unmarshal issuer certificate in standards mode; error was %s", err)
	}
	key := respPayload.Keys[0]
	return generateRSAKeyFromStandardJSONKey(key)
}

// generateRSAKeyFrom*JSONKey converts the provided jsonKey by decoding
// the base64 strings and coercing to big.Int (for N) and int (for E).

// Workiva's 1.0 public key endpoint has a few non-standard quirks:
//   - Each key part is encoded with URLEncoding (rather than StdEncoding)
//   - Each key part is byte-ordered using little-endian (rather than big-endian)
func generateRSAKeyFromWorkiva10JSONKey(key jsonKey) (*rsa.PublicKey, error) {
	// base64.URLEncoding.DecodeString requires strings which have length
	// divisible by 4
	nPad := padString(key.N)
	nBytes, err := base64.URLEncoding.DecodeString(nPad)
	if err != nil {
		return nil, err
	}
	ePad := padString(key.E)
	eBytes, err := base64.URLEncoding.DecodeString(ePad)
	if err != nil {
		return nil, err
	}

	n := &big.Int{}
	n.SetString(string(nBytes), 10)

	e, err := strconv.Atoi(string(eBytes))
	if err != nil {
		return nil, err
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// Go's RSA implementation permits only 4 bytes in the public exponent
const rsaMaxExponentBytes = 4

// Go's RSA implementation limits the exponent to an int32
const rsaMaxExponent = 1<<31 - 1

// Key parts n and e are big-endian
func generateRSAKeyFromStandardJSONKey(key jsonKey) (*rsa.PublicKey, error) {
	// base64.StdEncoding.DecodeString requires strings which have length
	// divisible by 4
	nPad := padString(key.N)
	nBytes, err := base64.StdEncoding.DecodeString(nPad)
	if err != nil {
		return nil, err
	}
	n := &big.Int{}
	n.SetBytes(nBytes)

	ePad := padString(key.E)
	eBytes, err := base64.StdEncoding.DecodeString(ePad)
	if err != nil {
		return nil, err
	}
	e := &big.Int{}
	e.SetBytes(eBytes)
	if len(e.Bytes()) > rsaMaxExponentBytes || e.Int64() > rsaMaxExponent {
		return nil, fmt.Errorf("oauth2: public exponent (E) %d was larger than the max exponent size %d", e, rsaMaxExponent)
	}
	// We are sure this is safe because of the size check above.
	eInt := int(e.Int64())

	return &rsa.PublicKey{N: n, E: eInt}, nil
}

// padString appends "="'s to the end of the given string. The returned string
// has length divisible by 4.
func padString(str string) string {
	modulo := len(str) % 4
	if modulo > 0 {
		for i := 0; i < 4-modulo; i++ {
			str = str + "="
		}
	}

	return str
}

type KeyDecodeError struct {
	key jsonKey
	err error
}

func (e KeyDecodeError) Error() string {
	return fmt.Sprintf("failed to decode retrieved key %v: %s", e.key, e.err)
}
