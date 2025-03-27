package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Options struct {
	ExpoHMACSecret string
	PayloadPath    string
	Endpoint       string
}

func DefaultOptions() *Options {
	return &Options{}
}

func BindOptions(fs *flag.FlagSet, opts *Options) {
	fs.StringVar(&opts.PayloadPath, "payload", opts.PayloadPath, "Path to a JSON file we send as a payload.")
	fs.StringVar(&opts.Endpoint, "endpoint", opts.Endpoint, "Endpoint to send payloads to.")

	fs.StringVar(&opts.ExpoHMACSecret, "hmac-secret", opts.ExpoHMACSecret, "HMAC token to sign webhook payloads.")
}

func (o *Options) Validate() error {
	if o.PayloadPath == "" {
		return fmt.Errorf("payload is required")
	}
	if o.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if o.ExpoHMACSecret == "" {
		return fmt.Errorf("hmac-secret is required")
	}
	return nil
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

	payload, err := os.ReadFile(opts.PayloadPath)
	if err != nil {
		log.Fatalf("failed to read payload file: %v", err)
	}

	digest := hmac.New(sha1.New, []byte(opts.ExpoHMACSecret))
	digest.Write(payload)

	req, err := http.NewRequest("POST", opts.Endpoint, bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	encoded := hex.EncodeToString(digest.Sum(nil))
	req.Header.Set("expo-signature", "sha1="+encoded)
	req.Header.Set("signature", "sha1="+encoded)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("failed to post: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read response: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		log.Fatalf("failed to close response body: %v", err)
	}
	fmt.Printf("POST %s: %d\n", opts.Endpoint, resp.StatusCode)
	fmt.Printf("%s\n", body)
}
