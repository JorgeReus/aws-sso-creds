package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bigkevmcd/go-configparser"
	"github.com/ktr0731/go-fuzzyfinder"
)

func fuzzyPreviewer(credentialsPath string, rolesPath string) string {
	var selected string
	creds, err := configparser.NewConfigParserFromFile(credentialsPath)

	roles, err := configparser.NewConfigParserFromFile(rolesPath)

	sections := creds.Sections()
	sections = append(sections, roles.Sections()...)
	sort.Strings(sections)
	_, err = fuzzyfinder.FindMulti(
		sections,
		func(i int) string {
			return sections[i]
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			selected = sections[i]
			s := fmt.Sprintf("[%s]\n", selected)

			// if is a profile (~/.aws/confg)
			var aux configparser.Dict
			var showedKeys configparser.Dict = make(configparser.Dict)
			if strings.HasPrefix(selected, "profile") {
				aux, err = roles.Items(selected)
				if err != nil {
					return ""
				}
				showedKeys["Account name"] = aux["sso_account_name"]
				showedKeys["Account ID"] = aux["sso_account_id"]
				showedKeys["Region"] = aux["region"]

			} else {
				aux, err = creds.Items(selected)
				if err != nil {
					return ""
				}
				showedKeys["AWS access key id"] = aux["aws_access_key_id"]
				iss, err := strconv.Atoi(aux["issued_time"])
				if err != nil {
					return ""
				}
				exp, err := strconv.Atoi(aux["expires_time"])
				if err != nil {
					return ""
				}
				expiredAt := time.Unix(int64(exp), 0)
				showedKeys["Issued at"] = time.Unix(int64(iss), 0).String()
				showedKeys["Expires at"] = expiredAt.String()
				if expiredAt.Before(time.Now()) {
					showedKeys["Status"] = "Expiradas"
				} else {
					showedKeys["Status"] = "Válidas"
				}
			}

			for _, key := range showedKeys.Keys() {
				s += fmt.Sprintf("%s: %s\n", key, showedKeys[key])
			}
			return s
		}))

	parts := strings.Split(selected, " ")
	var role string
	if len(parts) == 1 {
		role = parts[0]
	} else {
		role = parts[1]
	}
	return role
}
