/*
Settings page logic.
*/

package logic

import (
	"appengine/datastore"
	"html/template"
	"igps/cache"
	"igps/ds"
	"igps/page"
	"strconv"
	"strings"
	"time"
)

// Valid map preview image formats.
var imgFormats = []string{"jpg", "png"}

func init() {
	page.NamePageMap["Settings"].Logic = settings
}

// settings is the logic implementation of the Settings page.
func settings(p *page.Params) {
	p.Custom["ImgFormats"] = imgFormats

	fv := p.Request.PostFormValue

	if fv("submitSettings") == "" {
		// No form submitted. Initial values:
		p.Custom["GoogleAccount"] = p.Account.Email
		p.Custom["ContactEmail"] = p.Account.ContactEmail
		p.Custom["LocationName"] = p.Account.LocationName
		if p.Account.LogsPageSize > 0 {
			p.Custom["LogsPageSize"] = p.Account.LogsPageSize
		}
		p.Custom["MapPrevSize"] = p.Account.MapPrevSize
		p.Custom["MobMapPrevSize"] = p.Account.MobMapPrevSize
		p.Custom["MobMapImgFormat"] = p.Account.MobMapImgFormat
		if p.Account.MobPageWidth > 0 {
		p.Custom["MobPageWidth"] = p.Account.MobPageWidth
		}
		return
	}

	p.Custom["GoogleAccount"] = fv("googleAccount")
	p.Custom["ContactEmail"] = fv("contactEmail")
	p.Custom["LocationName"] = fv("locationName")
	p.Custom["LogsPageSize"] = fv("logsPageSize")
	p.Custom["MapPrevSize"] = fv("mapPrevSize")
	p.Custom["MobMapPrevSize"] = fv("mobMapPrevSize")
	p.Custom["MobMapImgFormat"] = fv("mobMapImgFormat")
	p.Custom["MobPageWidth"] = fv("mobPageWidth")

	// Checks:
	switch {
	case !checkGoogleAccounts(p, fv("googleAccount")):
	case !checkContactEmail(p, fv("contactEmail")):
	case !checkLocationName(p, fv("locationName")):
	case !checkLogsPageSize(p, fv("logsPageSize")):
	case !checkMapPrevSize(p, "Map preview size", fv("mapPrevSize")):
	case !checkMapPrevSize(p, "Mobile Map preview size", fv("mobMapPrevSize")):
	case !checkMobMapImgFormat(p, fv("mobMapImgFormat")):
	case !checkMobPageWidth(p, fv("mobPageWidth")):
	}

	if p.ErrorMsg != nil {
		return
	}

	// All data OK, save Account

	c := p.AppCtx

	// Create a "copy" of the account, only set it if saving succeeds.
	// Have to set ALL fields (else their values would be lost when (re)saved)!
	var logsPageSize, mobPageWidth int
	if fv("logsPageSize") != "" {
		logsPageSize, _ = strconv.Atoi(fv("logsPageSize"))
	}
	if fv("mobPageWidth") != "" {
		mobPageWidth, _ = strconv.Atoi(fv("mobPageWidth"))
	}
	acc := ds.Account{
		Email: p.User.Email, Lemail: strings.ToLower(p.User.Email), UserID: p.User.ID,
		ContactEmail: fv("contactEmail"), LocationName: fv("locationName"), LogsPageSize: logsPageSize,
		MapPrevSize: fv("mapPrevSize"), MobMapPrevSize: fv("mobMapPrevSize"),
		MobMapImgFormat: fv("mobMapImgFormat"), MobPageWidth: mobPageWidth,
		Created: p.Account.Created, KeyID: p.Account.KeyID,
	}

	key := datastore.NewKey(c, ds.ENameAccount, "", p.Account.KeyID, nil)

	if _, p.Err = datastore.Put(c, key, &acc); p.Err == nil {
		p.InfoMsg = "Settings saved successfully."
		p.Account = &acc
		// Update cache with the new Account
		cache.CacheAccount(c, p.Account)
	}
}

// checkLocationName checks the specified Location name and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid or empty).
func checkLocationName(p *page.Params, locnam string) (ok bool) {
	if locnam == "" {
		return true
	}

	_, err := time.LoadLocation(locnam)
	if err != nil {
		p.ErrorMsg = template.HTML(`Invalid <span class="code">Location name</span>!`)
		return false
	}

	return true
}

// checkLogsPageSize checks the specified Logs Page Size string and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid or empty).
func checkLogsPageSize(p *page.Params, lps string) (ok bool) {
	if lps == "" {
		return true
	}

	basemsg := `Invalid <span class="code">Logs Table Page Size</span>!`

	num, err := strconv.Atoi(lps)
	if err != nil {
		p.ErrorMsg = SExecTempl(basemsg+` Invalid number: <span class="highlight">{{.}}</span>`, lps)
		return false
	}
	if num < 5 || num > 30 {
		p.ErrorMsg = SExecTempl(basemsg+` Value is outside of valid range (5..30): <span class="highlight">{{.}}</span>`, num)
		return false
	}

	return true
}

// checkMapPrevSize checks the specified Map preview size string and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid or empty).
func checkMapPrevSize(p *page.Params, settingName, mps string) (ok bool) {
	if mps == "" {
		return true
	}

	basemsg := `Invalid <span class="code">` + settingName + `</span>!`

	parts := strings.Split(mps, "x")
	if len(parts) != 2 {
		p.ErrorMsg = template.HTML(basemsg)
		return false
	}

	for i, part := range parts {
		name := []string{"width", "height"}[i]
		num, err := strconv.Atoi(part)
		if err != nil {
			p.ErrorMsg = SExecTempl(basemsg+` Invalid number for {{index . 0}}: <span class="highlight">{{index . 1}}</span>`, []interface{}{name, part})
			return false
		}
		if num < 100 || num > 640 { // 640 is an API limit for free accounts
			p.ErrorMsg = SExecTempl(basemsg+` {{index . 0}} is outside of valid range (100..640): <span class="highlight">{{index . 1}}</span>`, []interface{}{name, part})
			return false
		}
	}

	return true
}

// checkMobMapImgFormat checks the specified Mobile Map image format and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid or empty).
func checkMobMapImgFormat(p *page.Params, imgFormat string) (ok bool) {
	if imgFormat == "" {
		return true
	}

	for _, v := range imgFormats {
		if v == imgFormat {
			return true
		}
	}

	p.ErrorMsg = template.HTML(`Invalid <span class="code">Mobile Map image format</span>!`)

	return false
}

// checkMobPageWidth checks the specified Mobile Page width string and sets an appropriate error message
// if there's something wrong with it.
// Returns true if is acceptable (valid or empty).
func checkMobPageWidth(p *page.Params, mpw string) (ok bool) {
	if mpw == "" {
		return true
	}

	basemsg := `Invalid <span class="code">Mobile Page width</span>!`

	num, err := strconv.Atoi(mpw)
	if err != nil {
		p.ErrorMsg = SExecTempl(basemsg+` Invalid number: <span class="highlight">{{.}}</span>`, mpw)
		return false
	}
	if num < 400 || num > 10000 {
		p.ErrorMsg = SExecTempl(basemsg+` Value is outside of valid range (400..10000): <span class="highlight">{{.}}</span>`, num)
		return false
	}

	return true
}
