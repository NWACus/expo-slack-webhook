package build

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
	"time"

	"github.com/slack-go/slack"

	"github.com/NWACus/expo-slack-webhook/config"
	"github.com/NWACus/expo-slack-webhook/expo"
)

type WebhookPayload struct {
	Id        string        `json:"id"`
	AppId     string        `json:"appId"`
	Details   string        `json:"buildDetailsPageUrl"`
	Platform  expo.Platform `json:"platform"`
	Status    expo.Status   `json:"status"`
	Metadata  Metadata      `json:"metadata"`
	Error     expo.Error    `json:"error"`
	CreatedAt string        `json:"createdAt"`
}

type Metadata struct {
	AppName                   string `json:"appName"`
	expo.BuildVersionMetadata `json:",inline"`
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

	log.Printf("Recieved build webhook for %s %s (%s).\n", payload.Metadata.AppName, payload.Metadata.AppVersion, payload.Metadata.AppBuildVersion)

	// we can handle forwarding the data to Slack on our own time
	handlePayload(r.Context(), cfg, &payload)
}

func handlePayload(ctx context.Context, cfg *config.Config, w *WebhookPayload) {
	previousBuild, err := fetchPreviousBuild(ctx, cfg, w)
	if err != nil {
		log.Printf("failed to fetch previous build: %v", err)
	}

	previousUpdate, err := fetchPreviousUpdate(ctx, cfg, w)
	if err != nil {
		log.Printf("failed to fetch previous update: %v", err)
	}

	blocks, err := blocksFor(cfg, w, previousBuild, previousUpdate)
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

func fetchPreviousUpdate(ctx context.Context, cfg *config.Config, w *WebhookPayload) (*expo.Update, error) {
	createdAt, err := time.Parse(time.RFC3339, w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse createdAt: %v", err)
	}

	channel, err := cfg.ExpoClient.FetchUpdateChannel(ctx, w.AppId, w.Metadata.Channel)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch update channel: %v", err)
	}

	updateBranch := updateBranchFor(w.Platform, channel)
	if updateBranch == "" {
		return nil, fmt.Errorf("failed to find update branch for platform %v", w.Platform)
	}

	updates, err := cfg.ExpoClient.FetchUpdates(ctx, w.AppId, updateBranch, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updates: %v", err)
	}

	return previousUpdateFor(w.Platform, createdAt, updates)
}

func updateBranchFor(platform expo.Platform, channel *expo.UpdateChannel) string {
	if channel == nil {
		return ""
	}
	for _, branch := range channel.UpdateBranches {
		for _, group := range branch.UpdateGroups {
			for _, update := range group {
				if update.Platform.Equal(platform) {
					return update.Branch.Name
				}
			}
		}
	}
	return ""
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

func fetchPreviousBuild(ctx context.Context, cfg *config.Config, w *WebhookPayload) (*expo.Build, error) {
	builds, err := cfg.ExpoClient.FetchBuilds(ctx, w.AppId, w.Metadata.Channel, w.Platform, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch build list: %v", err)
	}
	for i := 0; i < len(builds); i++ {
		if builds[i].Id == w.Id && i < len(builds)-1 {
			log.Printf("Found previous build: %v", builds[i+1].Id)
			return &builds[i+1], nil
		}
	}
	return nil, nil
}

func blocksFor(cfg *config.Config, w *WebhookPayload, build *expo.Build, update *expo.Update) ([]slack.Block, error) {
	blocks := []slack.Block{
		&slack.HeaderBlock{
			Type: slack.MBTSection,
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf(`:hammer_and_wrench:%s%s| %s build of %s %s %s.`, expo.PlatformEmoji(w.Platform), expo.StatusEmoji(w.Status), expo.PlatformDisplay(w.Platform), w.Metadata.AppName, expo.FormatBuildVersion(w.Metadata.BuildVersionMetadata), expo.StatusDisplay(w.Status)),
			},
		},
	}
	if build != nil {
		createdAt, err := time.Parse(time.RFC3339, build.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse createdAt for build %s: %v", build.Id, err)
		}
		blocks = append(blocks, &slack.SectionBlock{
			Type: slack.MBTSection,
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf(`The <https://expo.dev/accounts/nwac/projects/avalanche-forecast/builds/%s|previous build>, %s, was published %s ago. See the changelog on <https://github.com/NWACus/avy/compare/%s...%s|GitHub>`, build.Id, expo.FormatBuildVersion(build.BuildVersionMetadata), formatDuration(time.Since(createdAt)), build.GitCommitHash, w.Metadata.GitCommitHash),
			},
		})
	}
	if update != nil {
		createdAt, err := time.Parse(time.RFC3339, update.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse createdAt for update %s: %v", update.Id, err)
		}
		blocks = append(blocks, &slack.SectionBlock{
			Type: slack.MBTSection,
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf(`The <https://expo.dev/accounts/nwac/projects/avalanche-forecast/updates/%s|previous update>, for commit <https://github.com/NWACus/avy/commit/%s|%s>, was published %s ago. See the changelog on <https://github.com/NWACus/avy/compare/%s...%s|GitHub>`, update.Id, update.GitCommitHash, update.GitCommitHash[0:7], formatDuration(time.Since(createdAt)), update.GitCommitHash, w.Metadata.GitCommitHash),
			},
		})
	}
	blocks = append(blocks, &slack.SectionBlock{
		Type: slack.MBTSection,
		Text: &slack.TextBlockObject{
			Type: slack.MarkdownType,
			Text: func() string {
				msg := ""
				if w.Error.Failed() {
					msg += fmt.Sprintf("Error %s\n", w.Error.Error())
				}
				msg += fmt.Sprintf("See build details <%s|here>.", w.Details)
				return msg
			}(),
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
