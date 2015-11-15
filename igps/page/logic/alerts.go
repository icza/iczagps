/*
Alerts page logic.
*/

package logic

import (
	"appengine/datastore"
	"html/template"
	"igps/cache"
	"igps/ds"
	"igps/page"
	"strconv"
	"time"
)

func init() {
	page.NamePageMap["Alerts"].Logic = alerts
}

// alerts is the logic implementation of the Alerts page.
func alerts(p *page.Params) {
	c := p.AppCtx

	// First get devices
	var devices []*ds.Device
	if devices, p.Err = cache.GetDevListForAccKey(c, p.Account.GetKey(c)); p.Err != nil {
		return
	}
	p.Custom["Devices"] = devices

	fv := p.Request.PostFormValue

	// Detect form submits:
	switch {
	case fv("submitAdd") != "":
		// Add New Alert form submitted!
		// Checks:
		switch {
		case !checkDeviceID(p, fv("carDeviceID"), true, devices):
		case !checkDeviceID(p, fv("persMobDeviceID"), false, devices):
		}
		if p.ErrorMsg == nil {
			// So far so good. Futher checks: car and personal mobile device must differ
			carDevID, _ := strconv.ParseInt(fv("carDeviceID"), 10, 64)
			var persMobDevID int64
			if fv("persMobDeviceID") != "" {
				persMobDevID, _ = strconv.ParseInt(fv("persMobDeviceID"), 10, 64)
				if carDevID == persMobDevID {
					p.ErrorMsg = template.HTML(`<span class="code">Car GPS Device</span> and <span class="code">Personal Mobile GPS Device</span> cannot be the same!`)
				}
			}
			if p.ErrorMsg == nil {
				// So far still good. Furter check: same alert cannot be saved twice
				q := datastore.NewQuery(ds.ENameAlert).Ancestor(p.Account.GetKey(c))
				var alerts []*ds.Alert
				if _, p.Err = q.GetAll(c, &alerts); p.Err != nil {
					return
				}
				for _, alert := range alerts {
					if alert.CarDevID == carDevID && alert.PersMobDevID == persMobDevID {
						p.ErrorMsg = template.HTML(`An Alert with the same <span class="code">Car GPS Device</span> and <span class="code">Personal Mobile GPS Device</span> already exists!`)
					}
				}
			}
			if p.ErrorMsg == nil {
				// All data OK, save new Alert
				alert := ds.Alert{carDevID, persMobDevID, time.Now(), 0, "", ""}
				if _, p.Err = datastore.Put(c, datastore.NewIncompleteKey(c, ds.ENameAlert, p.Account.GetKey(c)), &alert); p.Err != nil {
					return // Datastore error
				}
				p.InfoMsg = "New Alert saved successfully."
			}
		}
	case fv("submitDelete") != "":
		// Delete Alert form submitted!
		if alertID, err := strconv.ParseInt(string(fv("alertID")), 10, 64); err != nil {
			p.ErrorMsg = "Invalid Alert!"
		} else {
			alertKey := datastore.NewKey(c, ds.ENameAlert, "", alertID, p.Account.GetKey(c))
			// Check if
			var alert ds.Alert
			if err = datastore.Get(c, alertKey, &alert); err != nil {
				if err == datastore.ErrNoSuchEntity {
					p.ErrorMsg = "You do not have access to the specified Alert!"
				} else {
					// Real datastore error
					p.Err = err
					return
				}
			} else {
				// Proceed to delete
				if p.Err = datastore.Delete(c, alertKey); p.Err != nil {
					return // Datastore error
				}
				p.InfoMsg = "Alert deleted successfully."
			}
		}
	}

	q := datastore.NewQuery(ds.ENameAlert).Ancestor(p.Account.GetKey(c))

	var alerts []*ds.Alert
	var alertKeys []*datastore.Key
	if alertKeys, p.Err = q.GetAll(c, &alerts); p.Err != nil {
		return
	}
	for i, alert := range alerts {
		alert.KeyID = alertKeys[i].IntID()
		for _, d := range devices {
			switch d.KeyID {
			case alert.CarDevID:
				alert.CarDevName = d.Name
			case alert.PersMobDevID:
				alert.PersMobDevName = d.Name
			}
		}
	}

	p.Custom["Alerts"] = alerts
}

// checkDeviceID checks the specified device ID and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid).
func checkDeviceID(p *page.Params, devIDst string, isCar bool, devices []*ds.Device) (ok bool) {
	var fieldName string
	if isCar {
		fieldName = `<span class="code">Car GPS Device</span>`
	} else {
		fieldName = `<span class="code">Personal Mobile GPS Device</span>`
	}

	if devIDst == "" {
		if !isCar {
			// Personal Mobile device is optional
			return true
		}
		p.ErrorMsg = template.HTML(fieldName + " must be provided! Please select a Device from the list.")
		return false
	}

	var devID int64
	var err error

	if devID, err = strconv.ParseInt(devIDst, 10, 64); err != nil {
		p.ErrorMsg = template.HTML("Invalid " + fieldName + "! Please select a Device from the list.")
		return false
	}

	// Check if device is owned by the user:
	for _, d := range devices {
		if d.KeyID == devID {
			return true
		}
	}

	p.ErrorMsg = "You do not have access to the specified Device! Please select a Device from the list."
	return false
}
