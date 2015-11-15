/*
Defines the Login log record type.
*/

package ds

import (
	"time"
)

// Name of the Datastore Login entity
const ENameLogin = "Lgn"

// Login type.
type Login struct {
	// Unique ID of the User.
	UserID string `datastore:"uid"`

	// Google Account email of the user
	Email string `datastore:"gae"`

	// Owner Account id, if it is a registered user.
	AccountID int64 `datastore:"aid"`

	// User Agent string.
	Agent string `datastore:"a"`

	// Client IP address
	IP string `datastore:"i"`

	// Location string of the client assembled from data taken from the HTTP request.
	//
	// It has the form of "country;region;city" where:
	//  -"country" is the ISO 3166-1 alpha-2 country code as reported by AppEngine in the X-AppEngine-Country HTTP header field
	//         (See: http://en.wikipedia.org/wiki/ISO_3166-1_alpha-2)
	//  -"region" is the ISO 3166-2 country specific region code as reported by AppEngine in the X-AppEngine-Region HTTP header field
	//         (See: http://en.wikipedia.org/wiki/ISO_3166-2)
	//  -"city" is the city as reported by AppEngine in the X-AppEngine-City HTTP header field
	//
	// More about request headers: https://cloud.google.com/appengine/docs/go/requests#Go_Request_headers
	Location string `datastore:"l"`

	// Timestamp
	Created time.Time `datastore:"t"`
}

// Check checks string fields and cuts them if they are longer than 500 bytes (Datastore limit).
func (l *Login) Check() {
	if len(l.Agent) > 500 {
		l.Agent = l.Agent[:500]
	}
	if len(l.IP) > 500 {
		l.IP = l.IP[:500]
	}
	if len(l.Location) > 500 {
		l.Location = l.Location[:500]
	}
}
