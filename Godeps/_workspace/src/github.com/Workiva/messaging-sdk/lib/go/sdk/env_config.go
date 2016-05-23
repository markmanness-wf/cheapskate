package sdk

import (
	"os"
	"strings"
)

// MsgURL returns the URL(s) for the gnatsd host(s) as specfied by the
// environment variable MSG_URL. This must be of the form
// nats://user:pass@host:port or tls://user:pass@host:port (if encrypted).
// This may also be a comma or space separated list of hosts, e.g.
// "tls://localhost:4222,tls://localhost:4223".
// Note: clusters are assumed to have the same tls credentials (or lack
// thereof).
func MsgURL() []string {
	return strings.FieldsFunc(os.Getenv("MSG_URL"), commaOrSpace)
}

// MsgCert returns the filepath for the NATS client certificate as specified
// by the environment variable MSG_CERT, e.g. "/etc/ssl/harbour/client.pem".
func MsgCert() string {
	return os.Getenv("MSG_CERT")
}

// MsgCACert returns the filepath for the NATS client certificated authority
// certificate as specified by the environment variable MSG_CA_CERT, e.g.
// "/etc/ssl/harbour/ca.crt".
func MsgCACert() string {
	return os.Getenv("MSG_CA_CERT")
}

// IamHost returns the iam auth host by checking the then the environment
// variables 'IAM_HOST' and 'OAUTH2_HOST'. This panics if nothing is set.
func IamHost() string {
	iamHost := os.Getenv("IAM_HOST")
	if iamHost != "" {
		return iamHost
	}

	iamHost = os.Getenv("OAUTH2_HOST")
	if iamHost != "" {
		return iamHost
	}

	panic("Must declare 'IAM_HOST' environment variable, e.g. https://wk-dev.wdesk.org")
}

// IamAllowedHosts returns a list of values from the environmental variable
// 'IAM_ALLOWED_HOST' separated by '|'. If not set, returns an empty list.
func IamAllowedHosts() []string {
	hosts := []string{}
	allowedHosts := os.Getenv("IAM_ALLOWED_HOSTS")
	if allowedHosts != "" {
		hosts = strings.Split(allowedHosts, "|")
	}
	return hosts
}

// IamClientID returns the iam client id by checking the environment variables
// 'IAM_CLIENT_ID' and 'OAUTH2_CLIENT_ID'. This panics if nothing is set.
func IamClientId() string {
	iamClientID := os.Getenv("IAM_CLIENT_ID")
	if iamClientID != "" {
		return iamClientID
	}

	iamClientID = os.Getenv("OAUTH2_CLIENT_ID")
	if iamClientID != "" {
		return iamClientID
	}

	panic("'Must declare 'IAM_CLIENT_ID' environment variable, e.g. datatables")
}

// IamPrivateKey returns the file name of the iam private key by checking the
// the environment variables 'IAM_PRIVATE_KEY' and 'OAUTH2_PRIVATE_KEY'. This
// panics if nothing is set.
func IamPrivateKey() string {
	iamPrivateKey := os.Getenv("IAM_PRIVATE_KEY")
	if iamPrivateKey != "" {
		return iamPrivateKey
	}

	iamPrivateKey = os.Getenv("OAUTH2_PRIVATE_KEY")
	if iamPrivateKey != "" {
		return iamPrivateKey
	}

	panic("Must declare 'IAM_PRIVATE_KEY' environment variable which points to a private RSA key")
}

// IamUnsafe returns a bool indicating if in IAM unsafe mode. This is keyed off
// `IAM_UNSAFE` environment variable. The following three values will TURN OFF
// SIGNATURE VERIFICATION: `true`, `yes`, `1`. This enviroment variable should
// ONLY be set for testing and MUST NOT BE SET IN PRODUCTION!
func IamUnsafe() bool {
	switch strings.ToLower(os.Getenv("IAM_UNSAFE")) {
	case "true", "yes", "1":
		return true
	}
	return false
}

func commaOrSpace(r rune) bool {
	return r == ',' || r == ' '
}
