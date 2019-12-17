//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/context"
	"github.com/heketi/heketi/pkg/logging"
)

var (
	logger          = logging.NewLogger("[jwt]", logging.LEVEL_DEBUG)
	iatLeeway int64 = 120
)

func init() {
	if leeway, exists := os.LookupEnv("HEKETI_JWT_IAT_LEEWAY_SECONDS"); exists {
		value, err := strconv.ParseUint(leeway, 10, 64)
		if err != nil {
			logger.Info("HEKETI_JWT_IAT_LEEWAY_SECONDS not valid: %v, input string: %v, using default: %v", err, leeway, 120)
		} else {
			iatLeeway = int64(value)
		}
	}
}

// From https://github.com/dgrijalva/jwt-go/pull/139 it is understood
// that if the machine where jwt token is generated and/or the machine
// where jwt token is verified have any clock skew then there is a
// possibility of getting a "Token used before issued" error.
// Therefore we have implemented a derived claim type of HeketiJwtClaims and
// the validation function for "iat" compares time with provision for leeway.
// Also, now that we control the validation function, we make "exp" validation
// mandatory and get rid of required_claims array.
type HeketiJwtClaims struct {
	*jwt.StandardClaims
	Qsh string `json:"qsh,omitempty"`
}

func (c *HeketiJwtClaims) Valid() error {
	vErr := new(jwt.ValidationError)
	now := jwt.TimeFunc().Unix()

	if c.VerifyExpiresAt(now, true) == false {
		delta := time.Unix(now, 0).Sub(time.Unix(c.ExpiresAt, 0))
		vErr.Inner = fmt.Errorf("Token is expired by %v", delta)
		vErr.Errors |= jwt.ValidationErrorExpired
		logger.LogError("exp validation failed: %v", vErr.Error())
	}

	// "iat" check
	if now < c.IssuedAt-iatLeeway {
		vErr.Inner = fmt.Errorf("Token used before issued")
		vErr.Errors |= jwt.ValidationErrorIssuedAt
		logger.LogError("iat validation failed: %v, time now: %v, time issued: %v", vErr.Error(), time.Unix(now, 0), time.Unix(c.IssuedAt, 0))
	}

	// "nbf" is not a required claim
	if c.VerifyNotBefore(now, false) == false {
		vErr.Inner = fmt.Errorf("token is not valid yet")
		vErr.Errors |= jwt.ValidationErrorNotValidYet
	}

	if vErr.Errors > 0 {
		return vErr
	}

	return nil
}

type JwtAuth struct {
	adminKey []byte
	userKey  []byte
}

type Issuer struct {
	PrivateKey string `json:"key"`
}

type JwtAuthConfig struct {
	Admin Issuer `json:"admin"`
	User  Issuer `json:"user"`
}

func generate_qsh(r *http.Request) string {
	// Please see Heketi REST API for more information
	claim := r.Method + "&" + r.URL.Path
	hash := sha256.New()
	hash.Write([]byte(claim))
	return hex.EncodeToString(hash.Sum(nil))
}

func NewJwtAuth(config *JwtAuthConfig) *JwtAuth {

	if config.Admin.PrivateKey == "" ||
		config.User.PrivateKey == "" {
		return nil
	}

	j := &JwtAuth{}
	j.adminKey = []byte(config.Admin.PrivateKey)
	j.userKey = []byte(config.User.PrivateKey)

	return j
}

func (j *JwtAuth) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	// Access token from header
	rawtoken, err := jwtmiddleware.FromAuthHeader(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Determine if we have the token
	if rawtoken == "" {
		http.Error(w, "Required authorization token not found", http.StatusUnauthorized)
		return
	}

	// Parse token
	var claims *HeketiJwtClaims
	token, err := jwt.ParseWithClaims(rawtoken, &HeketiJwtClaims{}, func(token *jwt.Token) (interface{}, error) {

		// Verify Method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		claims = token.Claims.(*HeketiJwtClaims)

		// Get claims
		if "" != claims.Issuer {
			switch claims.Issuer {
			case "admin":
				return j.adminKey, nil
			case "user":
				return j.userKey, nil
			default:
				return nil, errors.New("Unknown user")
			}
		}

		return nil, errors.New("Token missing iss claim")
	})
	if err != nil {
		errmsg := fmt.Sprintf("Invalid JWT token: %s", err)
		// annoying that the types don't actually match
		if err.Error() == jwt.ErrSignatureInvalid.Error() {
			errmsg += " (client and server secrets may not match)"
		}
		if strings.Contains(err.Error(), "used before issued") {
			errmsg += " (client and server clocks may differ)"
		}
		http.Error(w, errmsg, http.StatusUnauthorized)
		return
	}

	if !token.Valid {
		http.Error(w, "Invalid JWT token", http.StatusUnauthorized)
		return
	}

	// Check qsh claim
	if claims.Qsh != generate_qsh(r) {
		http.Error(w, "Invalid qsh claim in token", http.StatusUnauthorized)
		return
	}

	// Store token in request for other middleware to access
	context.Set(r, "jwt", token)

	// Everything passes call next middleware
	next(w, r)
}
