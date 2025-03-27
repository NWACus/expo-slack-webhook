package config

import (
	"fmt"
	"os"

	"github.com/slack-go/slack"

	"github.com/NWACus/expo-slack-webhook/expo"
)

type Config struct {
	ExpoHMACSecret string
	ExpoClient     *expo.Client

	SlackClient  *slack.Client
	SlackChannel string
}

func LoadFromEnv() (*Config, error) {
	config := &Config{}
	var slackToken, expoToken string
	for from, into := range map[string]*string{
		"SLACK_TOKEN":      &slackToken,
		"SLACK_CHANNEL":    &config.SlackChannel,
		"EXPO_HMAC_SECRET": &config.ExpoHMACSecret,
		"EXPO_TOKEN":       &expoToken,
	} {
		value, set := os.LookupEnv(from)
		if !set || value == "" {
			return nil, fmt.Errorf("%s not set", from)
		}
		*into = value
	}

	config.SlackClient = slack.New(slackToken)
	config.ExpoClient = &expo.Client{Token: expoToken}

	return config, nil
}
