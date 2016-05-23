package oauth2

import (
	"fmt"
	"log"
	"os"
	"strings"
)

/*

To get access to the configuration, pass the constant into the Get() function:

    clientID := config.Get(config.BigskyOAuthClientID)

*/

var initialized bool

// Environment Variables Names
const (
	Prod               = "PROD"
	BigskyPublicKeyURL = "BIGSKY_PUBLIC_KEY_URL"
	BigskyHost         = "BIGSKY_HOST"
	ClientID           = "CLIENT_ID"
)

var config struct {
	bigskyPublicKeyURL string
	bigskyHost         string
	prod               string
	clientID           string
}

// Get will return the value associated with the passed in configuration name.
// The loading of the values from the environment happens the first time Get is
// called.
//
// NOTE: if you pass an unrecognized name to this function, this function
// returns a blank string.
func Get(name string) string {
	load()

	switch name {
	case BigskyPublicKeyURL:
		return config.bigskyPublicKeyURL
	case BigskyHost:
		return config.bigskyHost
	case Prod:
		return config.prod
	case ClientID:
		return config.clientID
	}
	log.Printf("go-auth/oauth2: unrecognized configuration variable %q, returning blank string", name)
	return ""
}

// load will load all the configuration options from the environment variables.
func load() {
	if initialized {
		return
	}
	config.prod = getEnv(Prod, "", false)
	isProd := strings.ToLower(config.prod) == "true"
	config.bigskyPublicKeyURL = getEnv(BigskyPublicKeyURL, "", isProd)
	config.bigskyHost = getEnv(BigskyHost, "", isProd)
	config.clientID = getEnv(ClientID, "", false)
	initialized = true
}

// getEnv will get the enironment variable v. If it can't find it then, if p is
// false, it will return the provided default or, if p is true, it will panic.
func getEnv(v, d string, p bool) string {
	var x string
	if x = os.Getenv(v); x == "" {
		if !p {
			return d
		}
		panic(fmt.Sprintf("go-auth: required environment variable %q is not set", v))
	}
	log.Printf("%v = %v\n", v, x)
	return x
}

// IsProduction indicates if the server is running in production mode.
func IsProduction() bool {
	return strings.ToLower(Get(Prod)) == "true"
}
