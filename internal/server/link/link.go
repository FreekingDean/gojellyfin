package link

import (
	"net/http"
	"net/url"
	"regexp"
)

const (
	clientRoot       = "/web/#"
	quickConnectPage = "/quickconnect"
	ssoReturnCookie  = "gojellyfin_sso_return"
	ssoReturnPath    = "/sso/oidc"
	ssoReturnMaxAge  = 600
)

var Routes = []string{
	"GET /link",
	"HEAD /link",
	"GET /link/{code}",
	"HEAD /link/{code}",
}

var nonDigit = regexp.MustCompile(`\D`)

func Redirect(w http.ResponseWriter, r *http.Request) {
	route := Route(codeFrom(r))

	http.SetCookie(w, &http.Cookie{
		Name:     ssoReturnCookie,
		Value:    url.QueryEscape(route),
		Path:     ssoReturnPath,
		MaxAge:   ssoReturnMaxAge,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, clientRoot+route, http.StatusFound)
}

func Route(code string) string {
	if code == "" {
		return quickConnectPage
	}

	return quickConnectPage + "?" + url.Values{"code": {code}}.Encode()
}

func codeFrom(r *http.Request) string {
	code := r.PathValue("code")
	if code == "" {
		code = r.URL.Query().Get("code")
	}

	return nonDigit.ReplaceAllString(code, "")
}
