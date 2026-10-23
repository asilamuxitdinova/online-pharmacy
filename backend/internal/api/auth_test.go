package api

import (
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLoginAndMe(t *testing.T) {
	e := newEnv(t)
	auth, tokens := e.login("AZIZA@pharmacy.test", "Customer123!")
	if tokens.str("token_type") != "Bearer" || tokens.num("expires_in") != 900 || tokens.str("refresh_token") == "" {
		t.Fatalf("token set: %s", tokens.Raw)
	}
	me := e.call("GET", "/me", nil, auth...).expect(200)
	if me.str("id") != "usr_aziza" || me.str("role") != "customer" || me.str("auth_method") != "bearer" {
		t.Fatalf("me: %s", me.Raw)
	}
	if strings.Contains(string(me.Raw), "301234567") {
		t.Fatal("a customer must never see their taxpayer number")
	}
}

func TestHonestUnauthorizedReasons(t *testing.T) {
	e := newEnv(t)
	r := e.call("GET", "/me", nil).expectErr(401, "token_missing")
	if !strings.HasPrefix(r.Header.Get("WWW-Authenticate"), "Bearer") {
		t.Fatal("401 must carry WWW-Authenticate")
	}
	e.call("GET", "/me", nil, "Authorization", "Bearer not.a.jwt").expectErr(401, "token_invalid")
	e.call("GET", "/me", nil, "Authorization", "Basic abc").expectErr(401, "token_invalid")

	auth := e.as("aziza")
	e.advance(16 * time.Minute)
	e.call("GET", "/me", nil, auth...).expectErr(401, "token_expired")
}

func TestLockoutAfterFiveWrongPasswords(t *testing.T) {
	e := newEnv(t)
	for i := 1; i <= 4; i++ {
		r := e.call("POST", "/auth/login", map[string]string{"email": "aziza@pharmacy.test", "password": "wrong"}).
			expectErr(401, "invalid_credentials")
		if r.num("error", "details", "attempts_left") != int64(5-i) {
			t.Fatalf("attempt %d: %s", i, r.Raw)
		}
	}
	r := e.call("POST", "/auth/login", map[string]string{"email": "aziza@pharmacy.test", "password": "wrong"}).expectErr(429, "too_many_attempts")
	if r.Header.Get("Retry-After") != "300" {
		t.Fatalf("Retry-After = %q", r.Header.Get("Retry-After"))
	}
	// Even the right password is refused while locked.
	e.call("POST", "/auth/login", map[string]string{"email": "aziza@pharmacy.test", "password": "Customer123!"}).expectErr(429, "too_many_attempts")
	e.advance(5*time.Minute + time.Second)
	e.login("aziza@pharmacy.test", "Customer123!")

	// An unknown email answers exactly like a wrong password.
	e.call("POST", "/auth/login", map[string]string{"email": "nobody@pharmacy.test", "password": "x"}).expectErr(401, "invalid_credentials")
}

func TestRefreshRotatesAndDetectsReuse(t *testing.T) {
	e := newEnv(t)
	_, first := e.login("aziza@pharmacy.test", "Customer123!")
	rt1 := first.str("refresh_token")

	second := e.call("POST", "/auth/refresh", map[string]string{"refresh_token": rt1}).expect(200)
	rt2, at2 := second.str("refresh_token"), second.str("access_token")
	if rt2 == rt1 || at2 == first.str("access_token") {
		t.Fatal("refresh must return a new access token and a new refresh token")
	}
	e.call("GET", "/me", nil, "Authorization", "Bearer "+at2).expect(200)

	// Replaying the spent token ends the whole sign-in, including the new tokens.
	e.call("POST", "/auth/refresh", map[string]string{"refresh_token": rt1}).expectErr(401, "refresh_reused")
	r := e.call("GET", "/me", nil, "Authorization", "Bearer "+at2).expectErr(401, "session_ended")
	if r.str("error", "details", "reason") != "refresh_reused" {
		t.Fatalf("reason: %s", r.Raw)
	}
	e.call("POST", "/auth/refresh", map[string]string{"refresh_token": rt2}).expectErr(401, "session_ended")
	e.call("POST", "/auth/refresh", map[string]string{"refresh_token": "rt_nonsense"}).expectErr(401, "refresh_invalid")
}

func TestRenewalKeepsWorkingAfterAccessExpiry(t *testing.T) {
	e := newEnv(t)
	_, tokens := e.login("aziza@pharmacy.test", "Customer123!")
	e.advance(20 * time.Minute)
	e.call("GET", "/me", nil, "Authorization", "Bearer "+tokens.str("access_token")).expectErr(401, "token_expired")
	fresh := e.call("POST", "/auth/refresh", map[string]string{"refresh_token": tokens.str("refresh_token")}).expect(200)
	e.call("GET", "/me", nil, "Authorization", "Bearer "+fresh.str("access_token")).expect(200)
}

func TestLogoutEndsTokensAtOnce(t *testing.T) {
	e := newEnv(t)
	a1 := e.as("aziza")
	a2, t2 := e.login("aziza@pharmacy.test", "Customer123!")

	e.call("POST", "/auth/logout", nil, a1...).expect(204)
	e.call("GET", "/me", nil, a1...).expectErr(401, "session_ended")
	e.call("GET", "/me", nil, a2...).expect(200) // the other session lives

	e.call("POST", "/auth/logout", map[string]bool{"all_sessions": true}, a2...).expect(204)
	e.call("GET", "/me", nil, a2...).expectErr(401, "session_ended")
	e.call("POST", "/auth/refresh", map[string]string{"refresh_token": t2.str("refresh_token")}).expectErr(401, "session_ended")
}

func TestRegister(t *testing.T) {
	e := newEnv(t)
	r := e.call("POST", "/auth/register", map[string]string{"email": "new@pharmacy.test", "password": "short", "name": "N"}).
		expectErr(422, "validation_failed")
	if n := len(r.JSON["error"].(map[string]any)["fields"].([]any)); n != 4 {
		t.Fatalf("want 4 field errors (too short, digit, uppercase, name), got %d: %s", n, r.Raw)
	}
	ok := e.call("POST", "/auth/register", map[string]string{"email": "new@pharmacy.test", "password": "Secret123!", "name": "New Person"}).expect(201)
	e.call("GET", "/me", nil, "Authorization", "Bearer "+ok.str("access_token")).expect(200)
	e.call("POST", "/auth/register", map[string]string{"email": "new@pharmacy.test", "password": "Secret123!", "name": "Again"}).expectErr(409, "account_exists")
}

func TestOneTimeCode(t *testing.T) {
	e := newEnv(t)
	e.call("POST", "/auth/otp/request", map[string]string{"channel": "email", "destination": "aziza@pharmacy.test"}).expect(202)
	e.call("POST", "/auth/otp/request", map[string]string{"channel": "email", "destination": "aziza@pharmacy.test"}).expectErr(429, "too_many_attempts")

	inbox := e.call("GET", "/_test/inbox?recipient=aziza@pharmacy.test&kind=otp", nil).expect(200)
	if !strings.Contains(string(inbox.Raw), "123456") {
		t.Fatalf("the code should be in the demo inbox: %s", inbox.Raw)
	}
	r := e.call("POST", "/auth/otp/verify", map[string]string{"destination": "aziza@pharmacy.test", "code": "000000"}).expectErr(401, "code_invalid")
	if r.num("error", "details", "attempts_left") != 4 {
		t.Fatalf("attempts_left: %s", r.Raw)
	}
	ok := e.call("POST", "/auth/otp/verify", map[string]string{"destination": "aziza@pharmacy.test", "code": "123456"}).expect(200)
	e.call("GET", "/me", nil, "Authorization", "Bearer "+ok.str("access_token")).expect(200)
	// Used once only.
	e.call("POST", "/auth/otp/verify", map[string]string{"destination": "aziza@pharmacy.test", "code": "123456"}).expectErr(401, "code_invalid")

	// SMS, then expiry.
	e.call("POST", "/auth/otp/request", map[string]string{"channel": "sms", "destination": "+998900000002"}).expect(202)
	e.advance(6 * time.Minute)
	e.call("POST", "/auth/otp/verify", map[string]string{"destination": "+998900000002", "code": "123456"}).expectErr(401, "code_expired")

	// Five wrong codes kill the code.
	e.call("POST", "/auth/otp/request", map[string]string{"channel": "sms", "destination": "+998900000002"}).expect(202)
	for range 5 {
		e.call("POST", "/auth/otp/verify", map[string]string{"destination": "+998900000002", "code": "999999"}).expectErr(401, "code_invalid")
	}
	e.call("POST", "/auth/otp/verify", map[string]string{"destination": "+998900000002", "code": "123456"}).expectErr(429, "too_many_attempts")

	// Unknown destinations look the same from outside.
	e.call("POST", "/auth/otp/request", map[string]string{"channel": "email", "destination": "ghost@pharmacy.test"}).expect(202)
	e.call("POST", "/auth/otp/verify", map[string]string{"destination": "ghost@pharmacy.test", "code": "123456"}).expectErr(401, "code_invalid")
}

func TestRolesAreCheckedOnEveryRequest(t *testing.T) {
	e := newEnv(t)
	aziza := e.as("aziza")
	r := e.call("GET", "/staff/orders", nil, aziza...).expectErr(403, "role_required")
	if r.str("error", "required_role") != "staff" {
		t.Fatalf("403 must name the needed role: %s", r.Raw)
	}
	admin := e.as("admin")
	e.call("PATCH", "/admin/users/usr_aziza/role", map[string]string{"role": "staff"}, admin...).expect(200)
	// Same token, next request: the new role applies.
	e.call("GET", "/staff/orders", nil, aziza...).expect(200)
	e.call("GET", "/admin/users", nil, aziza...).expectErr(403, "role_required")
	e.call("PATCH", "/admin/users/usr_aziza/role", map[string]string{"role": "customer"}, admin...).expect(200)
	e.call("GET", "/staff/orders", nil, aziza...).expectErr(403, "role_required")

	e.call("PATCH", "/admin/users/usr_admin/role", map[string]string{"role": "customer"}, admin...).expectErr(409, "cannot_modify_self")
}

func TestDisabledAccount(t *testing.T) {
	e := newEnv(t)
	e.call("POST", "/auth/login", map[string]string{"email": "timur@pharmacy.test", "password": "Customer123!"}).expectErr(403, "account_disabled")

	aziza := e.as("aziza")
	admin := e.as("admin")
	e.call("POST", "/admin/users/usr_aziza/disable", nil, admin...).expect(200)
	e.call("GET", "/me", nil, aziza...).expectErr(403, "account_disabled")
	e.call("POST", "/admin/users/usr_aziza/enable", nil, admin...).expect(200)
	e.call("GET", "/me", nil, aziza...).expect(200)
}

func TestDeleteUserNeedsConfirmation(t *testing.T) {
	e := newEnv(t)
	bekzod := e.as("bekzod")
	admin := e.as("admin")
	e.call("DELETE", "/admin/users/usr_bekzod", nil, admin...).expectErr(422, "validation_failed")
	e.call("DELETE", "/admin/users/usr_bekzod?confirm_email=aziza@pharmacy.test", nil, admin...).expectErr(422, "validation_failed")
	e.call("DELETE", "/admin/users/usr_bekzod?confirm_email=bekzod@pharmacy.test", nil, admin...).expect(204)
	r := e.call("GET", "/me", nil, bekzod...).expect(401)
	if c := r.errCode(); c != "session_ended" {
		t.Fatalf("deleted user's token: %s", r.Raw)
	}
	e.call("GET", "/staff/customers/usr_bekzod", nil, e.as("dilnoza")...).expectErr(404, "customer_not_found")
}

func TestAPIKeys(t *testing.T) {
	e := newEnv(t)
	seeded := "phk_test_customer_aziza_0000000000000001"
	e.call("GET", "/me", nil, "X-API-Key", seeded).expect(200)
	e.call("GET", "/me", nil, "Authorization", "Bearer "+seeded).expect(200)
	me := e.call("GET", "/me?api_key="+seeded, nil).expect(200)
	if me.str("auth_method") != "api_key" {
		t.Fatalf("auth_method: %s", me.Raw)
	}
	e.call("GET", "/me", nil, "X-API-Key", "phk_test_wrong").expectErr(401, "api_key_invalid")

	auth := e.as("aziza")
	created := e.call("POST", "/me/api-keys", map[string]string{"name": "script"}, auth...).expect(201)
	key := created.str("key")
	e.call("GET", "/orders", nil, "X-API-Key", key).expect(200)
	e.call("DELETE", "/me/api-keys/"+created.str("id"), nil, auth...).expect(204)
	e.call("GET", "/orders", nil, "X-API-Key", key).expectErr(401, "api_key_revoked")
	list := e.call("GET", "/me/api-keys", nil, auth...).expect(200)
	if strings.Contains(string(list.Raw), key) {
		t.Fatal("listing must never show a full key")
	}
}

func TestCookieSession(t *testing.T) {
	e := newEnv(t)
	r := e.call("POST", "/auth/session", map[string]string{"email": "aziza@pharmacy.test", "password": "Customer123!"}).expect(200)
	cookie := r.Header.Get("Set-Cookie")
	if !strings.Contains(cookie, "HttpOnly") || !strings.HasPrefix(cookie, sessionCookie+"=") {
		t.Fatalf("cookie: %q", cookie)
	}
	value := strings.SplitN(cookie, ";", 2)[0]
	me := e.call("GET", "/me", nil, "Cookie", value).expect(200)
	if me.str("auth_method") != "cookie" {
		t.Fatalf("me: %s", me.Raw)
	}
	e.call("DELETE", "/auth/session", nil, "Cookie", value).expect(204)
	e.call("GET", "/me", nil, "Cookie", value).expectErr(401, "session_ended")
}

func TestOAuthWithPKCE(t *testing.T) {
	e := newEnv(t)
	verifier := "this-is-a-long-random-verifier-0123456789abcdef"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	params := url.Values{
		"response_type": {"code"}, "client_id": {"frege-test"}, "redirect_uri": {"https://frege.example/callback"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"xyz"},
	}

	page := e.call("GET", "/oauth/authorize?"+params.Encode(), nil).expect(200)
	if !strings.Contains(string(page.Raw), "<form") {
		t.Fatal("authorize must show a sign-in form")
	}
	// Plain PKCE is refused by redirecting back with an error.
	plain := url.Values{}
	for k, v := range params {
		plain[k] = v
	}
	plain.Set("code_challenge_method", "plain")
	if loc := e.call("GET", "/oauth/authorize?"+plain.Encode(), nil).expect(302).Header.Get("Location"); !strings.Contains(loc, "error=invalid_request") {
		t.Fatalf("plain PKCE: %s", loc)
	}
	// An unregistered redirect is shown, never followed.
	bad := url.Values{}
	for k, v := range params {
		bad[k] = v
	}
	bad.Set("redirect_uri", "http://evil.example/cb")
	e.call("GET", "/oauth/authorize?"+bad.Encode(), nil).expect(400)

	form := url.Values{}
	for k, v := range params {
		form[k] = v
	}
	form.Set("email", "aziza@pharmacy.test")
	form.Set("password", "Customer123!")
	r := e.call("POST", "/oauth/authorize", form.Encode(), "Content-Type", "application/x-www-form-urlencoded").expect(302)
	loc, _ := url.Parse(r.Header.Get("Location"))
	code := loc.Query().Get("code")
	if code == "" || loc.Query().Get("state") != "xyz" {
		t.Fatalf("redirect: %s", loc)
	}

	exchange := func(verifier, secret string) *resp {
		body := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"https://frege.example/callback"},
			"code_verifier": {verifier}, "client_id": {"frege-test"}, "client_secret": {secret}}
		return e.call("POST", "/oauth/token", body.Encode(), "Content-Type", "application/x-www-form-urlencoded")
	}
	if r := exchange(verifier, "wrong").expect(401); r.str("error") != "invalid_client" {
		t.Fatalf("bad secret: %s", r.Raw)
	}
	if r := exchange("wrong-verifier", "oauth-secret").expect(400); r.str("error") != "invalid_grant" {
		t.Fatalf("bad verifier: %s", r.Raw)
	}
	tokens := exchange(verifier, "oauth-secret").expect(200)
	e.call("GET", "/me", nil, "Authorization", "Bearer "+tokens.str("access_token")).expect(200)
	if r := exchange(verifier, "oauth-secret").expect(400); !strings.Contains(r.str("error_description"), "already used") {
		t.Fatalf("code reuse: %s", r.Raw)
	}

	// Refresh through the token endpoint, with HTTP Basic client auth.
	body := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokens.str("refresh_token")}}
	req := e.call("POST", "/oauth/token", body.Encode(), "Content-Type", "application/x-www-form-urlencoded",
		"Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("frege-test:oauth-secret"))).expect(200)
	if req.str("access_token") == tokens.str("access_token") {
		t.Fatal("refresh must give a new token")
	}
}

func TestSignedPartnerRequests(t *testing.T) {
	e := newEnv(t)
	stamp := strconv.FormatInt(e.now.Unix(), 10)
	path := "/partner/inventory?sku=PH-0001"
	sig := SignRequest("partner-secret", stamp, "GET", path, nil)
	r := e.call("GET", path, nil, "X-Client-Id", "partner-demo", "X-Timestamp", stamp, "X-Signature", sig).expect(200)
	if len(r.list("data")) != 1 {
		t.Fatalf("inventory: %s", r.Raw)
	}
	e.call("GET", path, nil).expectErr(401, "signature_missing")
	e.call("GET", path, nil, "X-Client-Id", "partner-demo", "X-Timestamp", stamp, "X-Signature", strings.Repeat("0", 64)).expectErr(401, "signature_invalid")
	old := strconv.FormatInt(e.now.Add(-10*time.Minute).Unix(), 10)
	e.call("GET", path, nil, "X-Client-Id", "partner-demo", "X-Timestamp", old,
		"X-Signature", SignRequest("partner-secret", old, "GET", path, nil)).expectErr(401, "signature_expired")

	body := `{"sku":"PH-0017","quantity":5}`
	sig = SignRequest("partner-secret", stamp, "POST", "/partner/restock", []byte(body))
	e.call("POST", "/partner/restock", body, "X-Client-Id", "partner-demo", "X-Timestamp", stamp, "X-Signature", sig).expect(200)
	// Behind the /api proxy the client signs the path it sent, prefix included.
	proxied := SignRequest("partner-secret", stamp, "GET", "/api"+path, nil)
	e.call("GET", path, nil, "X-Client-Id", "partner-demo", "X-Timestamp", stamp, "X-Signature", proxied, "X-Forwarded-Prefix", "/api").expect(200)

	// The body is part of the signature.
	e.call("POST", "/partner/restock", `{"sku":"PH-0017","quantity":500}`, "X-Client-Id", "partner-demo", "X-Timestamp", stamp, "X-Signature", sig).
		expectErr(401, "signature_invalid")
}
