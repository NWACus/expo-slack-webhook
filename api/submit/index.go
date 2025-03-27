package submit

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/slack-go/slack"

	"github.com/NWACus/expo-slack-webhook/config"
	"github.com/NWACus/expo-slack-webhook/expo"
)

type WebhookPayload struct {
	Id       string        `json:"id"`
	Details  string        `json:"submissionDetailsPageUrl"`
	Platform expo.Platform `json:"platform"`
	Status   expo.Status   `json:"status"`
	Info     Info          `json:"submissionInfo"`
}

type Info struct {
	Error expo.Error `json:"error"`
}

// Handler is the entrypoint for Vercel serverless functions.
func Handler(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	Handle(cfg, w, r)
}

// Handle consumes the webhook and posts the data to Slack.
func Handle(cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	log.Printf("Submission webhook received")
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read request body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	digest := hmac.New(sha1.New, []byte(cfg.ExpoHMACSecret))
	digest.Write(body)
	receivedSignature := r.Header.Get("expo-signature")
	log.Printf("Received signature: %v\n", receivedSignature)
	expectedSignature := fmt.Sprintf("sha1=%v", hex.EncodeToString(digest.Sum(nil)))
	if expectedSignature != receivedSignature {
		log.Printf("Invalid HMAC, received %v, expected %v\n", receivedSignature, expectedSignature)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if _, debug := os.LookupEnv("DEBUG"); debug {
		log.Printf("Received payload: %v\n", string(body))
	}

	payload := WebhookPayload{}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("failed to unmarshal payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// we want to signal to Expo that we got the webhook OK as soon as we can, as they have short timeouts on this
	w.WriteHeader(http.StatusOK)

	log.Printf("Received submission webhook for %s.\n", payload.Platform)

	// we can handle forwarding the data to Slack on our own time
	handlePayload(r.Context(), cfg, &payload)
}

func handlePayload(ctx context.Context, cfg *config.Config, w *WebhookPayload) {
	submission, err := cfg.ExpoClient.FetchSubmission(ctx, w.Id)
	if err != nil {
		log.Printf("failed to fetch submission: %v", err)
	}

	blocks, err := blocksFor(cfg, w, submission)
	if err != nil {
		log.Printf("failed to get blocks: %v", err)
		return
	}

	_, _, err = cfg.SlackClient.PostMessageContext(ctx, cfg.SlackChannel, slack.MsgOptionBlocks(blocks...), slack.MsgOptionDisableLinkUnfurl(), slack.MsgOptionDisableMediaUnfurl())
	if err != nil {
		log.Printf("failed to post message: %v", err)
	}
}

func blocksFor(cfg *config.Config, w *WebhookPayload, submission *expo.Submission) ([]slack.Block, error) {
	msg := expo.FormatTitle(":arrow_up:", "submission", w.Platform, w.Status)
	if submission != nil {
		msg = fmt.Sprintf(`:arrow_up:%s%s| %s submission of %s %s %s.`, expo.PlatformEmoji(w.Platform), expo.StatusEmoji(w.Status), expo.PlatformDisplay(w.Platform), submission.App.Name, expo.FormatBuildVersion(submission.SubmittedBuild.BuildVersionMetadata), expo.StatusDisplay(w.Status))
	}
	blocks := []slack.Block{
		&slack.HeaderBlock{
			Type: slack.MBTSection,
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: msg,
			},
		},
	}
	blocks = append(blocks,
		&slack.SectionBlock{
			Type: slack.MBTSection,
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: func() string {
					msg := ""
					if w.Info.Error.Failed() {
						msg += fmt.Sprintf("Error %s\n", w.Info.Error.Error())
					}
					msg += fmt.Sprintf("See details <%s|here>.", w.Details)
					return msg
				}(),
			},
		},
	)
	return blocks, nil
}
