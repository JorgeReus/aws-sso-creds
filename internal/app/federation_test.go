package app

import (
	"net/url"
	"testing"
)

func TestLoginUrlParamsGetUrl(t *testing.T) {
	params := &LoginUrlParams{
		Issuer:      "issuer",
		Destination: "https://console.aws.amazon.com/",
		SigninToken: "token",
	}

	got := params.GetUrl()
	want := "https://signin.aws.amazon.com/federation?Action=login&Issuer=issuer&Destination=https://console.aws.amazon.com/&SigninToken=token"
	if got != want {
		t.Fatalf("GetUrl() = %q, want %q", got, want)
	}
}

func TestSessionUrlParamsEncodeReturnsEscapedJSON(t *testing.T) {
	params := &SessionUrlParams{
		AccessKeyId:     "id",
		SecretAccessKey: "key",
		SessionToken:    "token",
	}

	got, err := params.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	want := url.QueryEscape(`{"sessionId":"id","sessionKey":"key","sessionToken":"token"}`)
	if got != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}
}
