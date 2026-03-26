package ui

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	configparser "github.com/bigkevmcd/go-configparser"
	mapset "github.com/deckarep/golang-set"
	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	VALID_TEXT   = "Valid"
	EXPIRED_TEXT = "Expired"
)

type FuzzyPreviewer struct {
	entries        *configparser.ConfigParser
	outputSections []string
	rolesMapping   *map[string]string
}

type findMultiFunc func(slice interface{}, itemFunc func(i int) string, opts ...fuzzyfinder.Option) ([]int, error)

var findMulti findMultiFunc = fuzzyfinder.FindMulti

func NewFuzzyPreviewer(credentialsPath string, rolesPath string) (*FuzzyPreviewer, error) {
	var err error
	var creds, roles *configparser.ConfigParser

	rolesMapping := map[string]string{}

	outputSections := []string{}
	extraSections := mapset.NewSet()
	entries := configparser.New()

	if _, err = os.Stat(credentialsPath); err == nil {
		creds, err = configparser.NewConfigParserFromFile(credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("cannot parse file %s: %w", credentialsPath, err)
		}

		// Go though the sections of the credentials file (~/.aws/credentials)
		for _, sec := range creds.Sections() {
			err := entries.AddSection(sec)
			if err != nil {
				return nil, fmt.Errorf("cannot add section %s: %w", sec, err)
			}
			extraSections.Add(sec)
			items, err := creds.Items(sec)
			if err != nil {
				return nil, fmt.Errorf(
					"cannot get the %s entries from the credentials file (~/.aws/credentials): %w",
					sec,
					err,
				)
			}

			rolesMapping[sec] = sec
			for k, v := range items {
				if err := entries.Set(sec, k, v); err != nil {
					return nil, fmt.Errorf("cannot set %s.%s: %w", sec, k, err)
				}
			}
		}
	}

	var outputName string
	var extraItems configparser.Dict
	if _, err = os.Stat(rolesPath); err == nil {
		roles, err = configparser.NewConfigParserFromFile(rolesPath)
		if err != nil {
			return nil, fmt.Errorf("cannot parse file %s: %w", rolesPath, err)
		}

		// Go though the sections of the config file (~/.aws/config)
		for _, sec := range roles.Sections() {
			profileName := strings.TrimPrefix(sec, "profile ")
			_, err := roles.GetBool(sec, "sso_auto_populated")
			// If is not autopopulated
			if err != nil {
				if entries.HasSection(profileName) {
					extraItems, _ = entries.Items(profileName)
					err = entries.RemoveSection(profileName)
					if err != nil {
						return nil, fmt.Errorf(
							"cannot erase section %s from configFile: %w",
							profileName,
							err,
						)
					}
					extraSections.Remove(profileName)
				}
				outputName = fmt.Sprintf("(profile) %s", profileName)
			} else {
				outputName = fmt.Sprintf("(SSO profile) %s", profileName)
			}

			rolesMapping[outputName] = sec

			outputSections = append(outputSections, outputName)
			if err := entries.AddSection(sec); err != nil {
				return nil, fmt.Errorf("cannot add section %s: %w", sec, err)
			}

			items, _ := roles.Items(sec)
			for k, v := range items {
				if err := entries.Set(sec, k, v); err != nil {
					return nil, fmt.Errorf("cannot set %s.%s: %w", sec, k, err)
				}
			}

			for k, v := range extraItems {
				if err := entries.Set(sec, k, v); err != nil {
					return nil, fmt.Errorf("cannot set %s.%s: %w", sec, k, err)
				}
			}
		}

	}

	if roles == nil && creds == nil {
		return nil, fmt.Errorf("neither %s nor %s exist, nothing to do", credentialsPath, rolesPath)
	}

	for sec := range extraSections.Iter() {
		outputSections = append(outputSections, sec.(string))
	}
	return &FuzzyPreviewer{
		rolesMapping:   &rolesMapping,
		entries:        entries,
		outputSections: outputSections,
	}, nil
}

func (fp *FuzzyPreviewer) generatePreviewAttrs(selected string) (*string, error) {
	s := fmt.Sprintf("[%s]\n", (*fp.rolesMapping)[selected])
	items, _ := fp.entries.Items((*fp.rolesMapping)[selected])
	keys := sort.StringSlice(items.Keys())
	titleCaser := cases.Title(language.English)
	for _, k := range keys {
		v := items[k]
		switch k {
		case "aws_secret_access_key", "aws_session_token":
			continue
		case "expires_time":
			exp, err := strconv.Atoi(v)
			if err != nil {
				s += fmt.Sprintf("Cannot parse: %s\n", k)
				continue
			}
			expiresAt := time.Unix(int64(exp), 0)
			s += fmt.Sprintf("Expires Time: %s\n", expiresAt.String())
			var expiredTxt string
			if expiresAt.Before(time.Now()) {
				expiredTxt = EXPIRED_TEXT
			} else {
				expiredTxt = VALID_TEXT
			}
			s += fmt.Sprintf("Status: %s\n", expiredTxt)
			continue
		case "issued_time":
			iss, err := strconv.Atoi(v)
			if err != nil {
				s += fmt.Sprintf("Cannot parse %s\n", k)
				continue
			}
			issuedAt := time.Unix(int64(iss), 0)
			s += fmt.Sprintf("Issued Time: %s\n", issuedAt.String())
			continue
		default:
			k = titleCaser.String(strings.ReplaceAll(k, "_", " "))
			s += fmt.Sprintf("%s: %s\n", k, v)
		}
	}
	return &s, nil
}

func (fp *FuzzyPreviewer) Preview() (*string, error) {
	var selected string
	indices, err := findMulti(
		fp.outputSections,
		func(i int) string {
			return fp.outputSections[i]
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			selected = fp.outputSections[i]
			s, err := fp.generatePreviewAttrs(selected)
			if err != nil {
				return fmt.Sprintf("cannot parse attributes from %s", selected)
			}
			return *s
		}))

	if err != nil {
		return nil, err
	}
	if len(indices) > 0 && indices[0] >= 0 && indices[0] < len(fp.outputSections) {
		selected = fp.outputSections[indices[0]]
	}

	result := strings.TrimPrefix((*fp.rolesMapping)[selected], "profile ")
	return &result, nil
}
