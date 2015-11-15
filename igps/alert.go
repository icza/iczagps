/*

Alert check implementation (scheduled cron job).

Checks if everything is ok with the Car and its
GPS device and sends alert emails if something is (or might be) wrong.

Also checks if the Car is reported moving when personal mobile is not or they are far away from each other
when car is moving.

*/

package igps

import (
	"appengine"
	"appengine/datastore"
	"appengine/mail"
	"fmt"
	"igps/ds"
	"igps/page/logic"
	"math"
	"net/http"
	"time"
)

func init() {
	http.HandleFunc("/cron/alert", alertHandler)
}

// alertHandler is the handler of the alert check cron job.
// Processes all Alerts.
func alertHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	// TODO consider caching the list of alerts.
	// But care must be taken: a non-ancestor query is not strongly consistent!
	// So it should be listing all with a keys-only query and load each with Get().

	// Note: this is not an ancestor query but it is not a problem (not a requirement to see changes immediately).
	q := datastore.NewQuery(ds.ENameAlert)

	var alerts []*ds.Alert
	var alertKeys []*datastore.Key
	var err error

	if alertKeys, err = q.GetAll(c, &alerts); err != nil {
		c.Errorf("Failed to get Alerts: %v", err)
		return
	}

	plural := ""
	if len(alerts) != 1 {
		plural = "s"
	}
	c.Infof("Loaded %d alert%s.", len(alerts), plural)

	for i, alert := range alerts {
		alert.KeyID = alertKeys[i].IntID()
		accKeyID := alertKeys[i].Parent().IntID()
		c.Infof("Processing #%d ...", i)
		c.Debugf("Alert id: %d, owner account id: %d", alert.KeyID, accKeyID)
		c.Debugf("Car dev id: %d, pers mob dev id: %d", alert.CarDevID, alert.PersMobDevID)

		checkAlert(c, alert, accKeyID)
	}
}

// checkAlert checks the specified alert.
func checkAlert(c appengine.Context, a *ds.Alert, accKeyID int64) {
	const alertDurationMin = 5
	const alertDuration = alertDurationMin * time.Minute

	// Get latest car GPS records
	carRecords, err := getDevRecords(c, a.CarDevID)
	if err != nil {
		return
	}

	// Check if car GPS records are received properly:
	if time.Since(carRecords[0].Created) > alertDuration {
		c.Warningf("No car GPS records found in the last %d minutes!", alertDurationMin)
		sendAlert(c, accKeyID, "Car GPS device gone dark!", carGoneDarkAlertMail)
		return
	}

	if a.PersMobDevID == 0 {
		c.Debugf("Car GPS records found in the last 5 minutes. No Personal Mobile device specified.")
		return
	}

	carMoved := devMoved(carRecords)
	if !carMoved {
		// Nothing more to do if car is not moving
		c.Infof("Car is not moving. Ok.")
		return
	}

	c.Infof("Car is moving!")

	// Get latest personal mobile GPS records
	persMobRecords, err := getDevRecords(c, a.PersMobDevID)
	if err != nil {
		return
	}

	// Check if personal mobile GPS records are received properly:
	if time.Since(persMobRecords[0].Created) > alertDuration {
		c.Warningf("No personal mobile GPS records found in the last %d minutes!", alertDurationMin)
		sendAlert(c, accKeyID, "Car is moving without you!", carMovingWithoutYouMail)
		return
	}

	persMobMoved := devMoved(persMobRecords)

	// Do not draw fast conclusion here if personal mobile is not moving,
	// it might be GPS tracking was just turned on and we don't have 2 track records yet
	// or they are not at great distance. But if we don't even have 1 track record,
	// that's hijacking-suspicious:

	if persMobMoved {
		c.Infof("Personal mobile is also moving!")
	} else {
		c.Infof("Personal mobile is NOT moving!")
	}

	var pg1 *ds.GPS
	for _, r := range persMobRecords {
		if r.Track() {
			pg1 = r
			break
		}
	}

	if pg1 == nil || time.Since(pg1.Created) > alertDuration {
		// Car is moving and we don't have recent track record from personal mobile!
		c.Warningf("No personal mobile GPS track record found in the last %d minutes!", alertDurationMin)
		sendAlert(c, accKeyID, "Car is moving without you!", carMovingWithoutYouMail)
		return
	}

	// Check distance:
	// We have a track record for both devices for sure (because both moved which also ensures having at least 2!).
	var cg1, cg2 *ds.GPS
	for _, r := range carRecords {
		if r.Track() {
			if cg1 == nil {
				cg1 = r
			} else if cg2 == nil {
				cg2 = r
				break
			}
		}
	}

	cg1.Dd = logic.Distance(cg2.GeoPoint.Lat, cg2.GeoPoint.Lng, cg1.GeoPoint.Lat, cg1.GeoPoint.Lng) // [m]
	cg1.Dt = cg1.Created.Sub(cg2.Created)                                                           // duration
	cv := float64(cg1.Dd) / cg1.Dt.Seconds()                                                        // Car speed [m/s]
	cpdt := math.Abs(cg1.Created.Sub(pg1.Created).Seconds())                                        // [s]
	dist := logic.Distance(cg1.GeoPoint.Lat, cg1.GeoPoint.Lng, pg1.GeoPoint.Lat, pg1.GeoPoint.Lng)

	c.Debugf("Car movement speed: %.1f km/h", cv*3.6)
	c.Debugf("Delta T between latest Car and PersMob GPS records: %d s", int64(cpdt))
	c.Debugf("Car - PersMob distance: %d m", dist)

	var alertMargin int64 = 500 // [m]
	// Increase alert margin based on the movement speed of the car and the delta time between
	// the last car and personal mobile GPS records.
	// Also if this delta time is greater, accuracy decreases/drops.
	// So also increase alert margin based on delta time: 6 meters for every second.
	// (It is an effect like increasing car speed by 6 m/s = 21.6 km/h.)
	// BE RESTRICTIVE: Only do this correction if personal mobile is also moving!
	// If not, do not let the car get far away (if for example pers mob is not moving,
	// the car could get kilometers away before alert would be sent).
	if persMobMoved {
		alertMargin += int64(cv*cpdt + cpdt*6)
	}
	c.Debugf("Using alert margin distance: %d m", alertMargin)

	if dist > alertMargin {
		c.Warningf("Personal mobile is not moving together with car!")
		sendAlert(c, accKeyID, "Car is moving without you!", carMovingWithoutYouMail)
		return
	}
	c.Infof("They are moving together. Ok.")
}

// getDeviceRecords returns the latest GPS records of the device with the specified id.
func getDevRecords(c appengine.Context, devKeyID int64) ([]*ds.GPS, error) {
	pageSize := 7
	q := datastore.NewQuery(ds.ENameGPS).Filter(ds.PNameDevKeyID+"=", devKeyID)
	q = q.Order("-" + ds.PNameCreated).Limit(pageSize)

	var rs = make([]*ds.GPS, 0, pageSize)
	if _, err := q.GetAll(c, &rs); err != nil {
		c.Errorf("Failed to get latest car GPS records for device id: %d: %v", devKeyID, err)
		return nil, err
	}

	if len(rs) == 0 {
		c.Errorf("No Device GPS records! Wrong device id? id: %d", devKeyID)
		return nil, fmt.Errorf("No Device GPS records! Wrong device id? id: %d", devKeyID)
	}

	return rs, nil
}

// devMoved tells if a device moved based on the passed latest GPS records.
// At least 2 track GPS records must be present to report moving (to return true).
func devMoved(rs []*ds.GPS) bool {
	// Min delta distance that is considered moving
	const minDeltaDist = 230 // [m]

	var first, last *ds.GPS // First and Last track records

	for _, r := range rs {
		if !r.Track() {
			continue
		}
		if first == nil {
			first = r
		}

		if last != nil {
			if logic.Distance(last.GeoPoint.Lat, last.GeoPoint.Lng, r.GeoPoint.Lat, r.GeoPoint.Lng) > minDeltaDist {
				return true
			}
		}

		last = r
	}

	// Device might be moving very slow, e.g. only 100 m per minute in which case subsequent
	// records will not report moving. Also compare the first to the last:
	if first != nil && last != nil {
		if logic.Distance(last.GeoPoint.Lat, last.GeoPoint.Lng, first.GeoPoint.Lat, first.GeoPoint.Lng) > minDeltaDist {
			return true
		}
	}

	return false
}

// sendAlert sends an alert email about the potential car hijacking.
func sendAlert(c appengine.Context, accKeyID int64, alertMsg, bodyTempl string) {
	// load account
	acc := new(ds.Account)
	key := datastore.NewKey(c, ds.ENameAccount, "", accKeyID, nil)
	if err := datastore.Get(c, key, acc); err != nil {
		c.Errorf("Failed to load account: %v", err)
		return
	}

	const adminEmail = "Andras Belicza <iczaaa@gmail.com>"
	msg := &mail.Message{
		Sender:  adminEmail,
		To:      []string{acc.Email},
		ReplyTo: adminEmail,
		Subject: "[IczaGPS] ALERT: " + alertMsg,
		Body:    fmt.Sprintf(bodyTempl, acc.Email),
	}
	if len(acc.ContactEmail) > 0 {
		msg.Cc = []string{acc.ContactEmail}
	}
	if err := mail.Send(c, msg); err == nil {
		c.Infof("Sent successful alert email: %s", alertMsg)
	} else {
		c.Errorf("Couldn't send alert email: %s, %v", alertMsg, err)
	}
}

const carGoneDarkAlertMail = `Hi %s,

WARNING: POTENTIAL CAR HIJACKING!

This is an alert email to let you know that your car GPS device has gone dark for more than 5 minutes now!

You can visit IczaGPS here:
https://iczagps.appspot.com

Best Regards,
Andras Belicza
`

const carMovingWithoutYouMail = `Hi %s,

WARNING: POTENTIAL CAR HIJACKING!

This is an alert email to let you know that your car GPS device is moving without your personal mobile!

You can visit IczaGPS here:
https://iczagps.appspot.com

Best Regards,
Andras Belicza
`
