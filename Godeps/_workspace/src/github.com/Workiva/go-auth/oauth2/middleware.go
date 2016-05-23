package oauth2

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

const (
	accessTokenCookieName = "access_token"
	authorizationHeader   = "Authorization"
	bearerPrefix          = "Bearer "
	AccountHeaderName     = "X-Workiva-Account"
	MembershipHeaderName  = "X-Workiva-Membership"
	UserHeaderName        = "X-Workiva-User"
	Secure                = MiddlewareSecure(true)
	Insecure              = MiddlewareSecure(false)
)

type JWTMiddlewareConfig struct {
	verifyToken        MiddlewareSecure
	Log                *log.Logger
	ValidateAccount    ValidateResourceFunc
	ValidateMembership ValidateResourceFunc
	ValidateClient     ValidateResourceFunc
	ValidateUser       ValidateResourceFunc
	ValidateScopes     ValidateScopesFunc
}

type ValidateResourceFunc func(resource string) error

type ValidateScopesFunc func(scopes []string) error

type MiddlewareSecure bool

// Middleware is a function compatible with go-rest Middleware and the standard
// HTTP library.
type Middleware func(w http.ResponseWriter, r *http.Request) bool

// NewJWTMiddleware creates a Middleware which validates JWT Tokens against the specified URLs.
// If the token is valid, its Account and Membership values are set on the request.
func NewJWTMiddleware(config JWTMiddlewareConfig, publicKeyUrls ...*url.URL) Middleware {
	if len(publicKeyUrls) == 0 {
		panic("publicKeyUrls must not be empty")
	}
	fillConfig(&config)
	config.verifyToken = Secure
	middlewareObj := jwtMiddleware{Config: config, Service: NewJWTService(publicKeyUrls...)}
	return middlewareObj.intercept
}

// NewInsecureJWTMiddleware creates a Middleware which does not actually require JWT Tokens.
// If a token is found, its Account and Membership values are set on the request.
func NewInsecureJWTMiddleware(config JWTMiddlewareConfig) Middleware {
	fillConfig(&config)
	config.verifyToken = Insecure
	middlewareObj := jwtMiddleware{Config: config, Service: NewJWTService()}
	return middlewareObj.intercept
}

// NewJWTMiddlewareViaService creates a Middleware which delegates JWT Token verification to the provided service.
// If `secure` is true, it calls `jwtService.ValidateToken()`.
// Otherwise, it calls `jwtService.GetTokenStringClaims()`.
func NewJWTMiddlewareViaService(config JWTMiddlewareConfig, jwtService JWTService, secure MiddlewareSecure) Middleware {
	fillConfig(&config)
	config.verifyToken = secure
	middlewareObj := jwtMiddleware{Config: config, Service: jwtService}
	return middlewareObj.intercept
}

func fillConfig(config *JWTMiddlewareConfig) {
	if config.Log == nil {
		blackholeLogger := log.New()
		blackholeLogger.Out = ioutil.Discard
		config.Log = blackholeLogger
	}
	if config.ValidateAccount == nil {
		config.ValidateAccount = alwaysValidResource
	}
	if config.ValidateMembership == nil {
		config.ValidateMembership = alwaysValidResource
	}
	if config.ValidateClient == nil {
		config.ValidateClient = alwaysValidResource
	}
	if config.ValidateUser == nil {
		config.ValidateUser = alwaysValidResource
	}
	if config.ValidateScopes == nil {
		config.ValidateScopes = alwaysValidScopes
	}
}

func alwaysValidResource(resource string) error {
	return nil
}

func alwaysValidScopes(scopes []string) error {
	return nil
}

type jwtMiddleware struct {
	Config  JWTMiddlewareConfig
	Service JWTService
}

// intercept authorizes requests based on the Authorization header or a cookie.
// If the token is valid, sets the Account and Membership headers in `r`.
func (m *jwtMiddleware) intercept(w http.ResponseWriter, r *http.Request) bool {
	tokenProperties, err := m.authorizeRequest(r, m.Service)
	if err != nil {
		m.Config.Log.WithFields(log.Fields{
			"error": err, "token": tokenProperties,
		}).Debug("Rejecting request due to bad JWT")

		bearer := fmt.Sprintf(`Bearer error="invalid_token", error_description="%s"`, err.Error())
		w.Header().Add("WWW-Authenticate", bearer)
		w.WriteHeader(http.StatusUnauthorized)
		return true
	}
	return false
}

// authorizeRequest authorizes requests based on the Authorization header or a cookie.
// Sets the Account and Membership headers if the token is acceptable.
// If m.Config.Secure is true, an error is returned if the token is invalid.
// In any case, the Account and Membership headers are unchanged
// if the token is not acceptable.
func (m *jwtMiddleware) authorizeRequest(r *http.Request, j JWTService) (*TokenProperties, error) {
	// If an error occurs while finding the token, `jwtString` will be empty
	// and it will be detected as an invalid token.
	jwtString, _ := FindToken(r)
	var properties TokenProperties

	if !m.Config.verifyToken {
		var err error
		properties, err = GetUnvalidatedTokenClaims(jwtString)
		if err != nil {
			m.Config.Log.WithFields(log.Fields{"jwtString": jwtString, "err": err}).Warn("Failed to GetUnvalidatedTokenClaims(); not operating securely so ignoring failure")
			return nil, nil
		}
	} else {
		var err error
		properties, err = j.ValidateToken(jwtString)
		if err != nil {
			return nil, err
		}

		expiration := time.Unix(properties.Expiration, 0)
		if time.Now().UTC().After(expiration) {
			return nil, fmt.Errorf("JWT has expired")
		}
		err = m.Config.ValidateAccount(strconv.FormatInt(properties.AccountID, 10))
		if err != nil {
			return nil, err
		}
		err = m.Config.ValidateMembership(strconv.FormatInt(properties.MembershipID, 10))
		if err != nil {
			return nil, err
		}
		err = m.Config.ValidateClient(properties.ClientID)
		if err != nil {
			return nil, err
		}
	}

	accountID := strconv.FormatInt(properties.AccountID, 10)
	membershipID := strconv.FormatInt(properties.MembershipID, 10)
	userID := strconv.FormatInt(properties.UserID, 10)

	r.Header.Set(AccountHeaderName, accountID)
	r.Header.Set(MembershipHeaderName, membershipID)
	r.Header.Set(UserHeaderName, userID)
	return &properties, nil
}

// FindToken searches the Authorization header and then the cookie for an OAuth2 token.
func FindToken(r *http.Request) (string, error) {
	// First attempt to pull the JWT from the `Authorization` header
	jwtString, err := getTokenFromHeader(r)
	if err != nil {
		// If that didn't work, attempt to pull the JWT from the cookie
		jwtString, err = getTokenFromCookie(r)
	}
	return jwtString, err
}

// getTokenFromHeader returns the JWT from the Authorization header or returns
// an error if the authorization header is missing.
var getTokenFromHeader = func(r *http.Request) (string, error) {
	auth := r.Header.Get(authorizationHeader)
	if !strings.HasPrefix(auth, bearerPrefix) {
		return "", fmt.Errorf("JWT not found on header")
	}
	return strings.TrimPrefix(auth, bearerPrefix), nil
}

// getTokenFromCookie returns the JWT from the access_token cookie or returns
// an error if the access_token cookie is missing.
var getTokenFromCookie = func(r *http.Request) (string, error) {
	cookie, err := r.Cookie(accessTokenCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}
