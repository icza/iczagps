/*
Register page logic.
*/

package logic

import (
	"appengine"
	"appengine/datastore"
	"appengine/mail"
	"fmt"
	"igps/cache"
	"igps/ds"
	"igps/page"
	"strings"
	"time"
)

func init() {
	page.NamePageMap["Register"].Logic = register
}

// register is the logic implementation of the Register page.
func register(p *page.Params) {
	// Register page is special. Unlike with other pages we do have to check User and Account here
	// Because even though Register page works with User, logged in user is not required for this page!

	// User might have logged out on a separate tab
	if p.User == nil {
		return
	}
	// User might have registered on a different tab
	if p.Account != nil {
		return
	}

	fv := p.Request.PostFormValue

	if fv("submitRegister") == "" {
		// No form submitted. Initial values:
		p.Custom["GoogleAccount"] = p.User.Email
		return
	}

	p.Custom["GoogleAccount"] = fv("googleAccount")
	p.Custom["ContactEmail"] = fv("contactEmail")
	p.Custom["AcceptTermsAndPolicy"] = fv("acceptTermsAndPolicy")

	// Checks:
	switch {
	case !checkGoogleAccounts(p, fv("googleAccount")):
	case !checkContactEmail(p, fv("contactEmail")):
	case !checkAcceptTermsAndPolicy(p, fv("acceptTermsAndPolicy")):
	}

	// UNTIL PROJECT GOES PUBLIC, DISABLE REGISTRATION:
	if !appengine.IsDevAppServer() && p.ErrorMsg == nil {
		p.ErrorMsg = "REGISTRATION IS CURRENTLY DISABLED! Contact the administrator if you would like to register!"
		return
	}
	// END OF: UNTIL PROJECT GOES PUBLIC, DISABLE REGISTRATION

	if p.ErrorMsg != nil {
		return
	}

	// All data OK, save new Account
	c := p.AppCtx
	acc := ds.Account{Email: p.User.Email, Lemail: strings.ToLower(p.User.Email), UserID: p.User.ID, ContactEmail: fv("contactEmail"), Created: time.Now()}
	var key *datastore.Key
	if key, p.Err = datastore.Put(c, datastore.NewIncompleteKey(c, ds.ENameAccount, nil), &acc); p.Err == nil {
		p.Custom["Created"] = true
		acc.KeyID = key.IntID()
		p.Account = &acc
		// Put new Account into the cache
		cache.CacheAccount(c, p.Account)
	}

	// Send registration email (Account info email)
	const adminEmail = "Andras Belicza <iczaaa@gmail.com>"
	msg := &mail.Message{
		Sender:  adminEmail,
		To:      []string{acc.Email},
		Bcc:     []string{adminEmail},
		ReplyTo: adminEmail,
		Subject: "[IczaGPS] Account Info",
		Body:    fmt.Sprintf(accountInfoMail, acc.Email),
	}
	if len(acc.ContactEmail) > 0 {
		msg.Cc = []string{acc.ContactEmail}
	}
	if err := mail.Send(c, msg); err == nil {
		c.Infof("Sent successful registration email.")
	} else {
		c.Errorf("Couldn't send email: %v", err)
	}
}

// checkAcceptTermsAndPolicy checks the specified accept terms and policty response and sets an appropriate erorr message
// if there's something wrong with it.
// Returns true if it is acceptable (checked).
func checkAcceptTermsAndPolicy(p *page.Params, atap string) (ok bool) {
	if atap == "" {
		p.ErrorMsg = "You must accept the Terms and Policy!"
		return false
	}
	return true
}

const accountInfoMail = `Hi %s,

This is a confirmation email that you have successfully created an IczaGPS account.

You can visit IczaGPS here:
https://iczagps.appspot.com

Should you have any questions, hit "Reply" to this email.

Best Regards,
Andras Belicza
`
