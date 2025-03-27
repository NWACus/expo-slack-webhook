package expo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type buildVariables struct {
	AppId  string      `json:"appId"`
	Filter buildFilter `json:"filter"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type buildFilter struct {
	Channel  string `json:"channel"`
	Platform string `json:"platform"`
}

const buildOperation = "ViewBuildsOnApp"
const buildQuery = "query ViewBuildsOnApp($appId: String!, $offset: Int!, $limit: Int!, $filter: BuildFilter) {\n  app {\n    byId(appId: $appId) {\n      id\n      builds(offset: $offset, limit: $limit, filter: $filter) {\n        id\n        ...BuildFragment\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}\nfragment BuildFragment on Build {\n  id\n  status\n  platform\n  error {\n    errorCode\n    message\n    docsUrl\n    __typename\n  }\n  artifacts {\n    buildUrl\n    xcodeBuildLogsUrl\n    applicationArchiveUrl\n    buildArtifactsUrl\n    __typename\n  }\n  initiatingActor {\n    __typename\n    id\n    displayName\n  }\n  project {\n    __typename\n    id\n    name\n    slug\n    ... on App {\n      ownerAccount {\n        id\n        name\n        __typename\n      }\n      __typename\n    }\n  }\n  channel\n  distribution\n  iosEnterpriseProvisioning\n  buildProfile\n  sdkVersion\n  appVersion\n  appBuildVersion\n  runtimeVersion\n  gitCommitHash\n  gitCommitMessage\n  initialQueuePosition\n  queuePosition\n  estimatedWaitTimeLeftSeconds\n  priority\n  createdAt\n  updatedAt\n  message\n  completedAt\n  expirationDate\n  isForIosSimulator\n  metrics {\n    buildWaitTime\n    buildQueueTime\n    buildDuration\n    __typename\n  }\n  __typename\n}"

type buildResponse struct {
	Data struct {
		App struct {
			ById struct {
				Builds []Build `json:"builds"`
			} `json:"byId"`
		} `json:"app"`
	} `json:"data"`
}

func (c *Client) FetchBuilds(ctx context.Context, projectId, channel string, platform Platform, limit, offset int) ([]Build, error) {
	log.Printf("Fetching %d+%d builds for %s, on channel %s on app %s", offset, limit, platform, channel, projectId)
	query := graphQLQuery[buildVariables]{
		OperationName: buildOperation,
		Query:         buildQuery,
		Variables: buildVariables{
			AppId: projectId,
			Filter: buildFilter{
				Channel:  channel,
				Platform: strings.ToUpper(string(platform)),
			},
			Limit:  limit,
			Offset: offset,
		},
	}

	payload, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", expoAPIURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("accept", "application/graphql-response+json")
	req.Header.Add("accept", "application/graphql+json")
	req.Header.Add("accept", "application/json")
	req.Header.Add("authorization", "bearer "+c.Token)
	req.Header.Add("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch builds: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read response: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		log.Printf("failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch builds: %d: %s", resp.StatusCode, string(body))
	}
	if _, debug := os.LookupEnv("DEBUG"); debug {
		log.Printf("response body: %s", string(body))
	}

	var parsed buildResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	log.Printf("Fetched %d builds for %s, on channel %s on app %s", len(parsed.Data.App.ById.Builds), platform, channel, projectId)
	return parsed.Data.App.ById.Builds, nil
}
