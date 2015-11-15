/*
Common/shared utilities used by page logics.
*/

package logic

import (
	"bytes"
	"html/template"
	"igps/page"
	"net/mail"
)

// SExecTempl executes the specified template and returns its result as template.HTML.
func SExecTempl(templ string, data interface{}) template.HTML {
	buf := &bytes.Buffer{}
	template.Must(template.New("").Parse(templ)).Execute(buf, data)
	return template.HTML(buf.String())
}

// checkContactEmail checks the specified Contact Email and sets an appropriate erorr message
// if there's something wrong with it.
// Returns true if email is acceptable (valid or empty).
func checkContactEmail(p *page.Params, email string) (ok bool) {
	if email == "" {
		return true
	}
	if len(email) > 500 {
		p.ErrorMsg = template.HTML(`<span class="code">Contact email</span> is too long! (cannot be longer than 500 characters)`)
		return false
	}

	if _, err := mail.ParseAddressList(email); err != nil {
		p.ErrorMsg = template.HTML(`Invalid <span class="code">Contact email</span>!`)
		return false
	}
	return true
}

// checkGoogleAccounts checks if previous and current Google Accounts match and sets an error message
// informing the user that the currently logged in Google Account has changed.
// Returns true if Google Accounts match.
func checkGoogleAccounts(p *page.Params, googleAccount string) (ok bool) {
	if googleAccount == p.User.Email {
		return true
	}

	const msg = `The current logged in user <span class="highlight">{{.User}}</span> does not match the <span class="code">Google Account</span>.<br/>
This is most likely due to you switched Google Accounts.<br/>
Click here to reload the {{.Page.Link}} page.`

	p.ErrorMsg = SExecTempl(msg, p)
	return false
}
