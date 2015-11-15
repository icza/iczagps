/*
Defines the GPS record type.
*/

package ds

import (
	"appengine"
	"fmt"
	"time"
)

// Name of the Datastore GPS entity
const ENameGPS = "G"

// Event type associated with GPS records.
type Event int64

const (
	EvtStart Event = -1 // Tracker start
	EvtStop  Event = -2 // Tracker stop
)

func (e Event) String() string {
	switch e {
	case EvtStart:
		return "Start"
	case EvtStop:
		return "Stop"
	default:
		return "Track"
	}
}

// A GPS record: device ID, Area code with event integrated, location (GeoPoint) and timestamp.
// There will be many, many GPS records; this entity must be as compact as possible.
type GPS struct {
	// ID field of the Device's key (not the RandID).
	DevKeyID int64 `datastore:"d"`

	// Area codes for location searches OR an event indicator.
	// If Area code value is negative, it is an event indicator like Start or Stop.
	// Non-negative values are Area codes.
	// See the igps/page/logic/AreaCodeForGeoPt() function for details.
	AreaCodes []int64 `datastore:"a"`

	// Geopoint, GPS latitude and longitude coordinates
	GeoPoint appengine.GeoPoint `datastore:"g,noindex"`

	// Timestamp
	Created time.Time `datastore:"t"`

	// ------------------------------------------------------------------------------
	// Derived/computed fields

	// When displayed in a table, delta distance from the previous (in time) record, in meters.
	Dd int64 `datastore:"-"`

	// When displayed in a table, delta time from the previous (in time) record.
	Dt time.Duration `datastore:"-"`
	
	// Label of the record as it appears on map previews, it's one character in ranges 1..9 or A..Z.
	Label rune `datastore:"-"`
}

// Evt returns the Event of the GPS record.
// Event is not stored as a separate property, it is determined by the Area code.
func (g *GPS) Evt() Event {
	if len(g.AreaCodes) == 0 {
		return 0 // Will be considered as Track
	}
	return Event(g.AreaCodes[0])
}

// Track tells if the record is a track event (has GeoPoint).
func (g *GPS) Track() bool {
	return len(g.AreaCodes) == 0 || g.AreaCodes[0] >= 0
}

// Ago returns the elapsed time since the creation of the record, truncated to seconds.
func (g *GPS) Ago() time.Duration {
	return time.Since(g.Created) / time.Second * time.Second
}

// Metrics tells if there are metrics present (calculated with the previous (in time) record).
func (g *GPS) Metrics() bool {
	return g.Dd >= 0
}

// DtString returns the delta time, truncated to seconds.
func (g *GPS) DtString() int64 {
	return int64(g.Dt / time.Second)
}

// V returns the movment speed specified by Dd and Dt, in km/h.
func (g *GPS) V() string {
	return fmt.Sprintf("%.1f", float64(g.Dd)/g.Dt.Hours()/1000.0)
}

// StaticMapURL returns a URL that should be set to an <img> element
// which loads a static Google maps image showing the location of this GPS record.
func (g *GPS) StaticMapURL() string {
	return fmt.Sprintf("https://maps.googleapis.com/maps/api/staticmap?markers=%f,%f&zoom=15&size=500x500&key=AIzaSyCEU_tZ1n0-mMg4woGKIfPqdbi0leSKvjg", g.GeoPoint.Lat, g.GeoPoint.Lng)
}

// EmbeddedMapURL returns a URL that should be set to an <iframe> element
// which loads an embedded Google maps showing the location of this GPS record.
func (g *GPS) EmbeddedMapURL() string {
	return fmt.Sprintf("https://www.google.com/maps/embed/v1/place?q=%f%,%f&key=AIzaSyCEU_tZ1n0-mMg4woGKIfPqdbi0leSKvjg", g.GeoPoint.Lat, g.GeoPoint.Lng)
}
