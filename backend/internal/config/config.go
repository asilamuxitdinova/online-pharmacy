// Package config reads the service settings from the environment.
//
// Every value has a default that works on a laptop, so `make run` needs no
// setup. In production the systemd unit loads them from an EnvironmentFile.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr         string // listen address, e.g. ":8080"
	PublicURL    string // the one https address written into the OpenAPI spec
	DatabasePath string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CookieSecure    bool

	LoginMaxAttempts int
	LoginLockout     time.Duration
	OTPTTL           time.Duration
	FixedOTPCode     string // when set, every one-time code is this value

	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURIs []string // empty means any https (or localhost) redirect is accepted

	PartnerClientID string
	PartnerSecret   string
	SignatureSkew   time.Duration

	WebhookSecret string // used by the /_test/webhooks receiver and outgoing events
	WebhookURL    string // when set, order events are POSTed here

	AutoRefundLimit       int64 // refunds up to this amount (UZS) are approved automatically
	FreeDeliveryThreshold int64
	DeliveryFee           int64
	JobDuration           time.Duration

	TestHelpers    bool   // mounts /_test/*
	TestResetToken string // when set, /_test/reset requires X-Test-Token
}

func Load() (Config, error) {
	c := Config{
		Addr:         env("ADDR", ":8080"),
		PublicURL:    strings.TrimRight(env("PUBLIC_URL", "https://pharmacy.localhost"), "/"),
		DatabasePath: env("DATABASE_PATH", "./data/pharmacy.db"),

		JWTSecret:    env("JWT_SECRET", "dev-only-jwt-secret-change-me"),
		CookieSecure: envBool("COOKIE_SECURE", true),

		FixedOTPCode: env("FIXED_OTP_CODE", ""),

		OAuthClientID:     env("OAUTH_CLIENT_ID", "partner-app"),
		OAuthClientSecret: env("OAUTH_CLIENT_SECRET", "dev-only-oauth-secret"),
		OAuthRedirectURIs: envList("OAUTH_REDIRECT_URIS"),

		PartnerClientID: env("PARTNER_CLIENT_ID", "wholesale-partner"),
		PartnerSecret:   env("PARTNER_SECRET", "dev-only-partner-secret"),

		WebhookSecret: env("WEBHOOK_SECRET", "dev-only-webhook-secret"),
		WebhookURL:    env("WEBHOOK_URL", ""),

		TestHelpers:    envBool("TEST_HELPERS", true),
		TestResetToken: env("TEST_RESET_TOKEN", ""),
	}

	var err error
	durations := []struct {
		dst  *time.Duration
		name string
		def  time.Duration
	}{
		{&c.AccessTokenTTL, "ACCESS_TOKEN_TTL", 15 * time.Minute},
		{&c.RefreshTokenTTL, "REFRESH_TOKEN_TTL", 30 * 24 * time.Hour},
		{&c.LoginLockout, "LOGIN_LOCKOUT", 5 * time.Minute},
		{&c.OTPTTL, "OTP_TTL", 5 * time.Minute},
		{&c.SignatureSkew, "SIGNATURE_SKEW", 5 * time.Minute},
		{&c.JobDuration, "JOB_DURATION", 15 * time.Second},
	}
	for _, d := range durations {
		if *d.dst, err = envDuration(d.name, d.def); err != nil {
			return c, err
		}
	}

	ints := []struct {
		dst  *int64
		name string
		def  int64
	}{
		{&c.AutoRefundLimit, "AUTO_REFUND_LIMIT", 200_000},
		{&c.FreeDeliveryThreshold, "FREE_DELIVERY_THRESHOLD", 300_000},
		{&c.DeliveryFee, "DELIVERY_FEE", 25_000},
	}
	for _, i := range ints {
		if *i.dst, err = envInt(i.name, i.def); err != nil {
			return c, err
		}
	}
	attempts, err := envInt("LOGIN_MAX_ATTEMPTS", 5)
	if err != nil {
		return c, err
	}
	c.LoginMaxAttempts = int(attempts)

	if !strings.HasPrefix(c.PublicURL, "https://") {
		return c, fmt.Errorf("PUBLIC_URL must be an https address, got %q", c.PublicURL)
	}
	return c, nil
}

func env(name, def string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return def
}

func envBool(name string, def bool) bool {
	v, err := strconv.ParseBool(env(name, strconv.FormatBool(def)))
	if err != nil {
		return def
	}
	return v
}

func envInt(name string, def int64) (int64, error) {
	v, err := strconv.ParseInt(env(name, strconv.FormatInt(def, 10)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return v, nil
}

func envDuration(name string, def time.Duration) (time.Duration, error) {
	v, err := time.ParseDuration(env(name, def.String()))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return v, nil
}

func envList(name string) []string {
	var out []string
	for _, s := range strings.Split(env(name, ""), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
