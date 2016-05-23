package sdk

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/Workiva/frugal/lib/go"
	goauth "github.com/Workiva/go-auth/oauth2"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
)

const (
	grantTypeJWTBearer = "urn:ietf:params:oauth:grant-type:jwt-bearer"
	grantTypeImplicit  = "implicit"

	authHeaderName   = "Authorization"
	authHeaderPrefix = "Bearer "

	membershipHeaderName = "membership"
	accountHeaderName    = "account"
	userHeaderName       = "user"
)

// fakeJWTHeader is the base64 encoded unsecure header for JWT tokens.
// See https://tools.ietf.org/html/rfc7519#section-6
const fakeJWTHeader = "eyJhbGciOiJub25lIn0"

// jwtServ is the global JWTService used for all IamContext validations
var jwtServ goauth.JWTService
var once sync.Once

// getJWTService allows returning a mockJWTService for unit testing
var getJWTService = func() goauth.JWTService {
	// Lazily set set the global JWTService
	once.Do(setJWT)
	return jwtServ
}

// getJWTServiceUrls builds a list of urls based upon the hosts from
// IamHost and optionally a valid list of hosts from IamAllowedHosts
// with the public certificate path appended.
func getJWTServiceUrls() []*url.URL {
	certPath := "/iam/oauth2/v2.0/certs"
	iamUrl, err := url.Parse(IamHost() + certPath)
	if err != nil {
		panic(err)
	}
	urls := []*url.URL{iamUrl}
	allowedUrls := IamAllowedHosts()
	for _, iamUrl := range allowedUrls {
		iamUrl, err := url.Parse(iamUrl + certPath)
		if err != nil {
			log.Warnf("Allowed Host value invalid URL. %s", err)
		} else {
			urls = append(urls, iamUrl)
		}
	}
	return urls
}

// setJWT uses the values from getJWTServiceUrls to make a JWT service
// The service get's their public certs making them valid Issuers.
func setJWT() {
	jwtServ = goauth.NewJWTService(getJWTServiceUrls()...)
}

// SetIamToken sets the auth header on an FContext with the given JWT token
func SetIamToken(ctx *frugal.FContext, token string) {
	ctx.AddRequestHeader(authHeaderName, authHeaderPrefix+token)
}

// SetServiceMembership sets membership, user, and account headers on the
// FContext. When hydrated as a IamContext, the membership, user, and account
// on the IamContext will be populated by the membership, user, and account
// headers on the FContext.Note: This API is only relevant for FContexts
// containing 2-legged (i.e. service) JWT tokens since receivers will ignore
// these headers for 3-legged tokens.
func SetIamMembershipHeaders(ctx *frugal.FContext, membership, user, account string) {
	ctx.AddRequestHeader(membershipHeaderName, membership)
	ctx.AddRequestHeader(userHeaderName, user)
	ctx.AddRequestHeader(accountHeaderName, account)
}

// SetServiceMembership sets the account header on the
// FContext. When hydrated as a IamContext, the membership, user, and account
// on the IamContext will be populated by the membership, user, and account
// headers on the FContext.Note: This API is only relevant for FContexts
// containing 2-legged (i.e. service) JWT tokens since receivers will ignore
// these headers for 3-legged tokens.
func SetIamAccountHeader(ctx *frugal.FContext, account string) {
	ctx.AddRequestHeader(accountHeaderName, account)
}

// SetServiceMembership sets the membership header on the
// FContext. When hydrated as a IamContext, the membership, user, and account
// on the IamContext will be populated by the membership, user, and account
// headers on the FContext.Note: This API is only relevant for FContexts
// containing 2-legged (i.e. service) JWT tokens since receivers will ignore
// these headers for 3-legged tokens.
func SetIamMembershipHeader(ctx *frugal.FContext, membership string) {
	ctx.AddRequestHeader(membershipHeaderName, membership)
}

// SetServiceMembership sets the user header on the
// FContext. When hydrated as a IamContext, the membership, user, and account
// on the IamContext will be populated by the membership, user, and account
// headers on the FContext.Note: This API is only relevant for FContexts
// containing 2-legged (i.e. service) JWT tokens since receivers will ignore
// these headers for 3-legged tokens.
func SetIamUserHeader(ctx *frugal.FContext, user string) {
	ctx.AddRequestHeader(userHeaderName, user)
}

// IamTokenFetcher defines an interface for fetching iam tokens
type IamTokenFetcher interface {
	// GetIamToken fetches a JWT
	GetIamToken() (string, error)
}

// NewIamJwtTokenFetcher produces an IamTokenFetcher for fetching JWTs
// using environmental config variables. If no tokenURLVersion is given,
// a default of "v2.0" is used.
func NewIamJwtTokenFetcher(tokenURLVersion string, scopes ...string) (IamTokenFetcher, error) {
	if tokenURLVersion == "" {
		tokenURLVersion = "v2.0"
	}

	privateKey, err := ioutil.ReadFile(IamPrivateKey())
	if err != nil {
		return nil, err
	}

	config := &jwt.Config{
		Email:      IamClientId(),
		PrivateKey: privateKey,
		Scopes:     scopes,
		TokenURL:   IamHost() + "/iam/oauth2/" + tokenURLVersion + "/token",
	}

	return &iamJwtTokenFetcher{
		tokenSource: config.TokenSource(context.TODO()),
	}, nil
}

type iamJwtTokenFetcher struct {
	tokenSource oauth2.TokenSource
}

// GetIamToken fetches a JWT
func (i *iamJwtTokenFetcher) GetIamToken() (string, error) {
	// The oauth2 library handles refreshing the token
	token, err := i.tokenSource.Token()
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

type mockTokenFetcher struct {
	token goauth.TokenProperties
}

// NewMockTokenFetcher retuns an IamTokenFetcher which returns unsigned tokens
// with the claims specified on the given TokenProperties.
func NewMockTokenFetcher(token goauth.TokenProperties) IamTokenFetcher {
	return &mockTokenFetcher{token}
}

// GetIamToken fetches an unsigned JWT
func (m *mockTokenFetcher) GetIamToken() (string, error) {
	token := m.token
	if token.Version == 0 {
		token.Version = 2
	}
	if token.Grant == "" {
		token.Grant = grantTypeImplicit
	}
	if token.Expiration == 0 {
		t := time.Now().Add(1 * time.Hour)
		token.Expiration = t.Unix()
	}
	claimsJSON, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s.", fakeJWTHeader, base64.URLEncoding.EncodeToString(claimsJSON)), nil
}

// IamContext defines an interface for validating tokens
type IamContext interface {
	// FContext returns the FContext struct wrapped by this
	FContext() *frugal.FContext
	// HasAnyScopes checks if the token contains any of the required scopes
	HasAnyScopes(requiredScopes ...string) bool
	// HasAllScopes checks if the token contains all of the required scopes
	HasAllScopes(requiredScopes ...string) bool
	// TokenProperties returns a struct containing the properties of the token
	TokenProperties() goauth.TokenProperties
	// MembershipID returns the membership ID from the token
	MembershipID() string
	// UserID returns the user ID from the token
	UserID() string
	// AccountID returns the account ID from the token
	AccountID() string
	// ClientID returns the client ID from the token
	ClientID() string
	// Roles returns the roles from the token
	Roles() []string
	// Licenses returns the licenses from the token
	Licenses() []string
	// Scopes returns the scopes from the token
	Scopes() []string
}

// NewIamJwtContext produces an IamContext from a context if the token
// on the context can be validated.
func NewIamJwtContext(ctx *frugal.FContext) (IamContext, error) {
	token, ok := ctx.RequestHeader(authHeaderName)
	if !ok {
		return nil, errors.New("messaging_sdk: '" + authHeaderName + "' header not present")
	}
	token = strings.Replace(token, authHeaderPrefix, "", -1)

	var properties goauth.TokenProperties
	var err error
	if IamUnsafe() {
		log.Warn("Skipping token validation for FContext")
		properties, err = goauth.GetUnvalidatedTokenClaims(token)
	} else {
		properties, err = getJWTService().ValidateToken(token)
	}
	if err != nil {
		return nil, err
	}

	switch properties.Grant {
	case grantTypeJWTBearer, grantTypeImplicit:
		// 2-legged and 3-legged tokens, respectively
	default:
		return nil, fmt.Errorf("messaging_sdk: Invalid grant type: %s", properties.Grant)
	}

	// Set membership, user, account for service token using FContext headers
	if properties.Service == 1 {
		properties.MembershipRID, _ = ctx.RequestHeader(membershipHeaderName)
		properties.UserRID, _ = ctx.RequestHeader(userHeaderName)
		properties.AccountRID, _ = ctx.RequestHeader(accountHeaderName)
	}
	return &iamJwtContext{fContext: ctx, properties: properties}, nil
}

type iamJwtContext struct {
	fContext   *frugal.FContext
	properties goauth.TokenProperties
}

// FContext returns the FContext struct wrapped by this
func (i *iamJwtContext) FContext() *frugal.FContext {
	return i.fContext
}

// TokenProperties returns a struct containing the properties of the token
func (i *iamJwtContext) TokenProperties() goauth.TokenProperties {
	return i.properties
}

// MembershipID returns the membership ID from the token
func (i *iamJwtContext) MembershipID() string {
	return i.properties.MembershipRID
}

// UserID returns the user ID from the token
func (i *iamJwtContext) UserID() string {
	return i.properties.UserRID
}

// AccountID returns the account ID from the token
func (i *iamJwtContext) AccountID() string {
	return i.properties.AccountRID
}

// ClientID returns the client ID from the token
func (i *iamJwtContext) ClientID() string {
	if i.properties.Version < 2 {
		return i.properties.ClientID
	}
	return i.properties.Audience
}

// Roles returns the roles from the token
func (i *iamJwtContext) Roles() []string {
	return i.properties.Roles
}

// Licenses returns the licenses from the token
func (i *iamJwtContext) Licenses() []string {
	return i.properties.Licenses
}

// Scopes returns the scopes from the token
func (i *iamJwtContext) Scopes() []string {
	return i.properties.Scopes
}

// HasAnyScopes checks if the token contains any of the required scopes
func (i *iamJwtContext) HasAnyScopes(requiredScopes ...string) bool {
	for _, reqScope := range requiredScopes {
		if contains(reqScope, i.properties.Scopes) {
			return true
		}
	}
	return false
}

// HasAllScopes checks if the token contains all of the required scopes
func (i *iamJwtContext) HasAllScopes(requiredScopes ...string) bool {
	for _, reqScope := range requiredScopes {
		if !contains(reqScope, i.properties.Scopes) {
			return false
		}
	}
	return true
}

// contains is a helper function for scope checking
func contains(value string, values []string) bool {
	for _, v := range values {
		if value == v {
			return true
		}
	}
	return false
}
