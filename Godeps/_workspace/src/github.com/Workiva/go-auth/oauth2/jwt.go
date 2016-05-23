package oauth2

import (
	"crypto/rsa"
	_ "crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/dgrijalva/jwt-go"
)

// TokenProperties stores the JWT properties
type TokenProperties struct {
	MembershipID  int64              `json:"mem"`
	MembershipRID string             `json:"mrid"` // Resource id, different than int64 id, available token/v2.0 to token/v4.0
	AccountID     int64              `json:"acc"`
	AccountRID    string             `json:"arid"` // Resource id, different than int64 id, available token/v2.0 to token/v4.0
	UserID        int64              `json:"usr"`
	UserRID       string             `json:"urid"` // Resource id, different than int64 id, available token/v2.0 to token/v4.0
	ClientID      string             `json:"cid"`  // Deprecated, use token/v2.0 endpoint with "aud" claim instead
	Audience      string             `json:"aud"`
	Expiration    int64              `json:"exp"`
	Service       uint8              `json:"wsk"`
	Issuer        string             `json:"iss"`
	Scopes        []string           `json:"scp"` // Available starting token/v2.0
	Scope         string             `json:"scope"`
	Version       uint8              `json:"ver"`
	Grant         string             `json:"gnt"`
	Roles         []string           `json:"roles"` // Available starting token/v3.0
	Licenses      []string           `json:"licenses"`
	Context       *ContextProperties `json:"context"` // Available starting token/v5.0
}

type ContextProperties struct {
	Account          string   `json:"account"`
	AcountName       string   `json:"account_name"`
	CollectStats     bool     `json:"collect_stats"`
	CSRFToken        string   `json:"csrf_token"`
	Licenses         []string `json:"licenses"`
	Membership       string   `json:"membership"`
	MembershipStatus uint8    `json:"membership_status"` // Possible values: 0, 1, 2. 0 = None, 1 = 1, 2 = 2 or more
	Permissions      []string `json:"permissions"`
	Roles            []string `json:"roles"`
	User             string   `json:"user"`
	Username         string   `json:"username"`
}

// TokenService provides a service layer for validating and decoding JWT tokens
type JWTService interface {
	// ValidateToken will validate the jwtString on the jwtService using the
	// issuer of the token (assuming the issuer is white-listed and the JWT
	// service has access to the public key. If the token is valid, the
	// decoded token properties are returned.
	ValidateToken(jwtString string) (TokenProperties, error)
}

type jwtService struct {
	publicKeys   map[string]*pubKey
	validIssuers []string
	tokenService TokenService
	keyLock      sync.Mutex
}

type pubKey struct {
	keyURL *url.URL
	cached *rsa.PublicKey
}

// NewJWTService returns a new jwtService instance with NewTokenService()
// using http.DefaultClient.
func NewJWTService(publicKeyURLs ...*url.URL) JWTService {
	return makeService(http.DefaultClient, publicKeyURLs...)
}

// NewJWTServiceWithClient returns a new jwtService instance with
// NewTokenService() using the provided http.Client.
func NewJWTServiceWithClient(publicKeyURL *url.URL, client *http.Client) JWTService {
	return makeService(client, publicKeyURL)
}

func makeService(client *http.Client, publicKeyURLs ...*url.URL) JWTService {
	publicKeys := generateKeys(publicKeyURLs...)
	validIssuers := keysFromMap(publicKeys)
	return &jwtService{
		publicKeys:   publicKeys,
		validIssuers: validIssuers,
		tokenService: NewTokenService(client),
	}
}

func generateKeys(publicKeyURLs ...*url.URL) map[string]*pubKey {
	keyMap := map[string]*pubKey{}
	for _, url := range publicKeyURLs {
		keyMap[fmt.Sprintf("%s://%s", url.Scheme, url.Host)] = &pubKey{keyURL: url}
	}
	return keyMap
}

// ValidateToken will validate the JWTString on the jwtService using the
// public key retrieved from PublicKeyURL. If the token is valid, the
// decoded token properties are returned.
func (j *jwtService) ValidateToken(jwtString string) (TokenProperties, error) {
	var err error
	var publicKey *rsa.PublicKey
	getVerificationKey := func(t *jwt.Token) (interface{}, error) {

		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
		}

		return publicKey, nil
	}

	var unverifiedClaims TokenProperties
	unverifiedClaims, err = GetUnvalidatedTokenClaims(jwtString)
	if err != nil {
		return TokenProperties{}, err
	}

	// Validate and Parse the jwt token
	var token *jwt.Token
	for i := 0; i < 2; i++ {

		useCache := i < 1 // First attempt uses cache
		if publicKey, err = j.getPublicKey(unverifiedClaims.Issuer, useCache); err != nil {
			break
		}

		// Validate and Parse the jwt token
		if token, err = jwt.Parse(jwtString, getVerificationKey); err == nil {
			break
		}
	}
	if err != nil {
		return TokenProperties{}, err
	}

	if token.Valid {
		// We have performed validation, so we can safely accept these claims.
		return unverifiedClaims, nil
	}
	return TokenProperties{}, fmt.Errorf("Invalid json web token: %s", jwtString)
}

// GetUnvalidatedTokenClaims insecurely decodes the `claims` portion of a JWT string
// into a TokenProperties instance.
// No claim validation is performed, so the claims could be forged.
// Use ValidateToken() to validate and decode claims in a secure way.
func GetUnvalidatedTokenClaims(jwtString string) (TokenProperties, error) {
	var properties TokenProperties
	splitString := strings.Split(jwtString, ".")
	if len(splitString) != 3 {
		return properties, fmt.Errorf("Not a valid JWT token string")
	}
	claims := splitString[1]
	claimsMap, err := base64.URLEncoding.DecodeString(padString(claims))
	if err != nil {
		return properties, err
	}
	if err = json.Unmarshal(claimsMap, &properties); err != nil {
		return properties, fmt.Errorf("Failed unmarshal on JWT claims: %s", err)
	}

	if len(properties.Scopes) == 0 {
		properties.Scopes = strings.Split(properties.Scope, " ")
	}

	if properties.ClientID == "" {
		properties.ClientID = properties.Audience
	}

	return properties, nil
}

// getPublicKey returns the public key from either the cache or directly from
// the key provider (as determined by the issuer).
func (j *jwtService) getPublicKey(issuer string, useCache bool) (
	*rsa.PublicKey, error) {

	j.keyLock.Lock()
	defer j.keyLock.Unlock()
	pubKey, ok := j.publicKeys[issuer]
	if !ok {
		return nil, fmt.Errorf("Invalid issuer %s; expected one of %s", issuer, j.validIssuers)
	}

	if useCache && pubKey.cached != nil && pubKey.cached.E > 0 {
		return pubKey.cached, nil
	}
	publicKey, err := j.tokenService.GetOAuth20TokenKey(pubKey.keyURL.String())
	if err != nil {
		return nil, err
	}

	pubKey.cached = publicKey
	return publicKey, nil
}

func keysFromMap(m map[string]*pubKey) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
