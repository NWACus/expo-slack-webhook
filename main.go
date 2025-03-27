package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/slack-go/slack"

	"github.com/NWACus/expo-slack-webhook/api/build"
	"github.com/NWACus/expo-slack-webhook/api/submit"
	"github.com/NWACus/expo-slack-webhook/api/update"
	"github.com/NWACus/expo-slack-webhook/config"
	"github.com/NWACus/expo-slack-webhook/expo"
)

type Options struct {
	ExpoHMACSecret string
	ExpoToken      string
	SlackToken     string
	SlackChannel   string

	Port int
}

func DefaultOptions() *Options {
	return &Options{
		Port: 8080,
	}
}

func BindOptions(fs *flag.FlagSet, opts *Options) {
	fs.StringVar(&opts.SlackToken, "slack-token", opts.SlackToken, "Slack API token.")
	fs.StringVar(&opts.SlackChannel, "slack-channel", opts.SlackChannel, "Slack channel to post updates to.")

	fs.StringVar(&opts.ExpoHMACSecret, "hmac-secret", opts.ExpoHMACSecret, "HMAC token to verify Expo webhook payloads.")
	fs.StringVar(&opts.ExpoToken, "expo-token", opts.ExpoToken, "Expo API token.")

	fs.IntVar(&opts.Port, "port", opts.Port, "Port to listen on.")
}

func (o *Options) Validate() error {
	if o.SlackToken == "" {
		return fmt.Errorf("slack-token is required")
	}
	if o.SlackChannel == "" {
		return fmt.Errorf("slack-channel is required")
	}
	if o.ExpoHMACSecret == "" {
		return fmt.Errorf("hmac-secret is required")
	}
	if o.ExpoToken == "" {
		return fmt.Errorf("expo-token is required")
	}
	return nil
}

func (o *Options) Complete() (*config.Config, error) {
	return &config.Config{
		ExpoHMACSecret: o.ExpoHMACSecret,
		SlackClient:    slack.New(o.SlackToken),
		SlackChannel:   o.SlackChannel,
		ExpoClient:     &expo.Client{Token: o.ExpoToken},
	}, nil
}

func main() {
	opts := DefaultOptions()
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	BindOptions(flags, opts)
	if err := flags.Parse(os.Args[1:]); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	if err := opts.Validate(); err != nil {
		log.Fatalf("failed to validate options: %v", err)
	}
	cfg, err := opts.Complete()
	if err != nil {
		log.Fatalf("failed to complete options: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/build", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		build.Handle(cfg, w, r)
	}))
	mux.Handle("/submit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		submit.Handle(cfg, w, r)
	}))
	mux.Handle("/update", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		update.Handle(cfg, w, r)
	}))
	server := &http.Server{Addr: fmt.Sprintf(":%d", opts.Port), Handler: mux}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		log.Printf("got an interrupt, shutting down server")
		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("failed to shutdown http server: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("failed to start http server: %v", err)
	}
}
