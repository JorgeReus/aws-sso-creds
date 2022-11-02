package app

import (
	"encoding/json"
	"fmt"
	"net/url"
)

func (l *LoginUrlParams) GetUrl() string {
	return fmt.Sprintf("%s?Action=login&Issuer=%s&Destination=%s&SigninToken=%s",
		AWS_FEDERATED_URL, l.Issuer, l.Destination,
		l.SigninToken)
}

func (s *SessionUrlParams) Encode() (string, error) {
	sess, err := json.Marshal(s)
	if err != nil {
		return "", nil
	}
	return url.QueryEscape(string(sess)), nil
}
