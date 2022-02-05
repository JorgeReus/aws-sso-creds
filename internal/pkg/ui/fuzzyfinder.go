package ui

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bigkevmcd/go-configparser"
	mapset "github.com/deckarep/golang-set"
	"github.com/ktr0731/go-fuzzyfinder"
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

func NewFuzzyPreviewer(credentialsPath string, rolesPath string) (*FuzzyPreviewer, error) {
	creds, err := configparser.NewConfigParserFromFile(credentialsPath)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot parse file %s: %v", credentialsPath, err))
	}
	roles, err := configparser.NewConfigParserFromFile(rolesPath)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Cannot parse file %s: %v", rolesPath, err))
	}

	rolesMapping := map[string]string{}

	outputSections := []string{}
	extraSections := mapset.NewSet()
	entries := configparser.New()

	// Go though the sections of the credentials file (~/.aws/credentials)
	for _, sec := range creds.Sections() {
		err := entries.AddSection(sec)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Cannot add section %s: %v", sec, err))
		}
		extraSections.Add(sec)
		items, err := creds.Items(sec)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Cannot get the %s entries from the credentials file (~/.aws/credentials): %v", sec, err))
		}

		rolesMapping[sec] = sec
		for k, v := range items {
			entries.Set(sec, k, v)
		}
	}

	var outputName string
	var extraItems configparser.Dict

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
					return nil, errors.New(fmt.Sprintf("Cannot erase section %s from configFile: %v", profileName, err))
				}
				extraSections.Remove(profileName)
			}
			outputName = fmt.Sprintf("(profile) %s", profileName)
		} else {
			outputName = fmt.Sprintf("(SSO profile) %s", profileName)
		}

		rolesMapping[outputName] = sec

		outputSections = append(outputSections, outputName)
		entries.AddSection(sec)

		items, _ := roles.Items(sec)
		for k, v := range items {
			entries.Set(sec, k, v)
		}

		for k, v := range extraItems {
			entries.Set(sec, k, v)
		}
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
	for _, k := range keys {
		v := items[k]
		switch k {
		case "aws_secret_access_key", "aws_session_token":
			continue
		case "expires_time":
			exp, err := strconv.Atoi(v)
			if err != nil {
				s += fmt.Sprintln(fmt.Sprintf("Cannot parse: %s", k))
			}
			expiresAt := time.Unix(int64(exp), 0)
			s += fmt.Sprintln(fmt.Sprintf("Expires Time: %s", expiresAt.String()))
			var expiredTxt string
			if expiresAt.Before(time.Now()) {
				expiredTxt = EXPIRED_TEXT
			} else {
				expiredTxt = VALID_TEXT
			}
			s += fmt.Sprintln(fmt.Sprintf("Status: %s", expiredTxt))
			break
		case "issued_time":
			iss, err := strconv.Atoi(v)
			if err != nil {
				s += fmt.Sprintln(fmt.Sprintf("Cannot parse %s", k))
			}
			issuedAt := time.Unix(int64(iss), 0)
			s += fmt.Sprintln(fmt.Sprintf("Issued Time: %s", issuedAt.String()))
			break
		default:
			k = strings.Title(strings.ReplaceAll(k, "_", " "))
			s += fmt.Sprintln(fmt.Sprintf("%s: %s", k, v))
		}
	}
	return &s, nil
}

func (fp *FuzzyPreviewer) Preview() (*string, error) {
	var selected string
	_, err := fuzzyfinder.FindMulti(
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
				return fmt.Sprintf("Cannot parse attributes from %s", selected)
			}
			return *s
		}))

	if err != nil {
		return nil, err
	}

	result := strings.TrimPrefix((*fp.rolesMapping)[selected], "profile ")
	return &result, nil
}
