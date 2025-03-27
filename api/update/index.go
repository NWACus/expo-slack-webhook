package update

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
	"strings"
	"time"

	"github.com/slack-go/slack"

	"github.com/NWACus/expo-slack-webhook/config"
	"github.com/NWACus/expo-slack-webhook/expo"
)

type Update struct {
	Id            string        `json:"id"`
	AppId         string        `json:"appId"`
	Group         string        `json:"group"`
	CreatedAt     string        `json:"createdAt"`
	Branch        string        `json:"branch"`
	Platform      expo.Platform `json:"platform"`
	GitCommitHash string        `json:"gitCommitHash"`
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
	log.Printf("Update webhook received")
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
	log.Printf("body: %v", string(body))

	digest := hmac.New(sha1.New, []byte(cfg.ExpoHMACSecret))
	digest.Write(body)
	receivedSignature := r.Header.Get("signature")
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

	payload := []Update{}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("failed to unmarshal payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// we want to signal to Expo that we got the webhook OK as soon as we can, as they have short timeouts on this
	w.WriteHeader(http.StatusOK)

	var ids []string
	var group string
	for _, update := range payload {
		group = update.Group
		ids = append(ids, update.Id)
	}
	log.Printf("Recieved update webhook for group %s, updates: %v.\n", group, strings.Join(ids, ","))

	// we can handle forwarding the data to Slack on our own time
	handlePayload(r.Context(), cfg, payload)
}

func handlePayload(ctx context.Context, cfg *config.Config, updates []Update) {
	for _, update := range updates {
		if _, allowPreviews := os.LookupEnv("ALLOW_PREVIEW"); !allowPreviews && strings.HasPrefix(update.Branch, "xxx") {
			log.Printf("skipping update for preview branch %s\n", update.Branch)
			continue
		}
		previousUpdate, err := fetchPreviousUpdate(ctx, cfg, update)
		if err != nil {
			log.Printf("failed to fetch previous update: %v", err)
		}

		blocks, err := blocksFor(cfg, update, previousUpdate)
		if err != nil {
			log.Printf("failed to get blocks: %v", err)
			return
		}

		log.Printf("Posting %d blocks to Slack channel %s", len(blocks), cfg.SlackChannel)
		_, _, err = cfg.SlackClient.PostMessageContext(ctx, cfg.SlackChannel, slack.MsgOptionBlocks(blocks...), slack.MsgOptionDisableLinkUnfurl(), slack.MsgOptionDisableMediaUnfurl())
		if err != nil {
			log.Printf("failed to post message: %v", err)
		}
	}
}

func fetchPreviousUpdate(ctx context.Context, cfg *config.Config, update Update) (*expo.Update, error) {
	createdAt, err := time.Parse(time.RFC3339, update.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse createdAt: %v", err)
	}

	updates, err := cfg.ExpoClient.FetchUpdates(ctx, update.AppId, update.Branch, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updates: %v", err)
	}

	return previousUpdateFor(update.Platform, createdAt, updates)
}

func previousUpdateFor(platform expo.Platform, createdAt time.Time, updates [][]expo.Update) (*expo.Update, error) {
	for i := 0; i < len(updates); i++ {
		for j := 0; j < len(updates[i]); j++ {
			if !updates[i][j].Platform.Equal(platform) {
				continue
			}
			updateCreatedAt, err := time.Parse(time.RFC3339, updates[i][j].CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse createdAt for update %s: %v", updates[i][j].Id, err)
			}
			if updateCreatedAt.After(createdAt) {
				continue
			}
			return &updates[i][j], nil
		}
	}
	return nil, nil
}

func blocksFor(cfg *config.Config, update Update, previous *expo.Update) ([]slack.Block, error) {
	blocks := []slack.Block{
		&slack.HeaderBlock{
			Type: slack.MBTSection,
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf(`:arrows_counterclockwise:%s%s| %s OTA update %s.`, expo.PlatformEmoji(update.Platform), expo.StatusEmoji(expo.StatusFinished), expo.PlatformDisplay(update.Platform), expo.StatusDisplay(expo.StatusFinished)),
			},
		},
	}
	if previous != nil {
		createdAt, err := time.Parse(time.RFC3339, previous.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse createdAt for update %s: %v", update.Id, err)
		}
		blocks = append(blocks, &slack.SectionBlock{
			Type: slack.MBTSection,
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf(`The <https://expo.dev/accounts/nwac/projects/avalanche-forecast/updates/%s|previous update>, for commit <https://github.com/NWACus/avy/commit/%s|%s>, was published %s ago. See the changelog on <https://github.com/NWACus/avy/compare/%s...%s|GitHub>`, update.Id, previous.GitCommitHash, previous.GitCommitHash[0:7], formatDuration(time.Since(createdAt)), previous.GitCommitHash, update.GitCommitHash),
			},
		})
	}
	blocks = append(blocks, &slack.SectionBlock{
		Type: slack.MBTSection,
		Text: &slack.TextBlockObject{
			Type: slack.MarkdownType,
			Text: fmt.Sprintf("See update details <https://expo.dev/accounts/nwac/projects/avalanche-forecast/updates/%s|here>.", update.Id),
		},
	})
	return blocks, nil
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d months", int(d.Hours()/(30*24)))
	default:
		return fmt.Sprintf("%d years", int(d.Hours()/(365*24)))
	}
}
