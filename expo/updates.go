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
)

type updateChannelVariables struct {
	AppId       string `json:"appId"`
	ChannelName string `json:"channelName"`
}

const updateChannelOperation = "ViewUpdateChannelOnApp"
const updateChannelQuery = "query ViewUpdateChannelOnApp($appId: String!, $channelName: String!, $filter: UpdatesFilter) {\n  app {\n    byId(appId: $appId) {\n      id\n      updateChannelByName(name: $channelName) {\n        id\n        isPaused\n        name\n        updatedAt\n        createdAt\n        branchMapping\n        updateBranches(offset: 0, limit: 5) {\n          id\n          name\n          updateGroups(offset: 0, limit: 1, filter: $filter) {\n            id\n            ...UpdateFragment\n            __typename\n          }\n          __typename\n        }\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}\nfragment UpdateFragment on Update {\n  id\n  group\n  message\n  createdAt\n  runtimeVersion\n  platform\n  manifestFragment\n  isRollBackToEmbedded\n  manifestPermalink\n  gitCommitHash\n  actor {\n    __typename\n    id\n    ... on UserActor {\n      username\n      __typename\n    }\n    ... on Robot {\n      firstName\n      __typename\n    }\n  }\n  branch {\n    id\n    name\n    __typename\n  }\n  codeSigningInfo {\n    keyid\n    sig\n    alg\n    __typename\n  }\n  rolloutPercentage\n  rolloutControlUpdate {\n    id\n    __typename\n  }\n  fingerprint {\n    id\n    hash\n    debugInfoUrl\n    source {\n      type\n      bucketKey\n      isDebugFingerprint\n      __typename\n    }\n    __typename\n  }\n  __typename\n}"

type updateChannelResponse struct {
	Data struct {
		App struct {
			ById struct {
				UpdateChannelByName UpdateChannel `json:"updateChannelByName"`
			} `json:"byId"`
		} `json:"app"`
	} `json:"data"`
}

func (c *Client) FetchUpdateChannel(ctx context.Context, projectId, channel string) (*UpdateChannel, error) {
	log.Printf("Fetching update channel %s for app %s", channel, projectId)
	query := graphQLQuery[updateChannelVariables]{
		OperationName: updateChannelOperation,
		Query:         updateChannelQuery,
		Variables: updateChannelVariables{
			AppId:       projectId,
			ChannelName: channel,
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
		return nil, fmt.Errorf("failed to fetch update channel: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read response: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		log.Printf("failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch update channel: %d: %s", resp.StatusCode, string(body))
	}
	if _, debug := os.LookupEnv("DEBUG"); debug {
		log.Printf("response body: %s", string(body))
	}

	var parsed updateChannelResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	log.Printf("Resolved update channel %s for app %s to %s", channel, projectId, parsed.Data.App.ById.UpdateChannelByName.Id)
	return &parsed.Data.App.ById.UpdateChannelByName, nil
}

type updateVariables struct {
	AppId      string `json:"appId"`
	BranchName string `json:"branchName"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

const updateOperation = "ViewUpdateGroupsOnBranch"
const updateQuery = "query ViewUpdateGroupsOnBranch($appId: String!, $branchName: String!, $limit: Int!, $offset: Int!, $filter: UpdatesFilter) {\n  app {\n    byId(appId: $appId) {\n      id\n      updateBranchByName(name: $branchName) {\n        id\n        updateGroups(limit: $limit, offset: $offset, filter: $filter) {\n          id\n          ...UpdateFragment\n          __typename\n        }\n        __typename\n      }\n      __typename\n    }\n    __typename\n  }\n}\nfragment UpdateFragment on Update {\n  id\n  group\n  message\n  createdAt\n  runtimeVersion\n  platform\n  manifestFragment\n  isRollBackToEmbedded\n  manifestPermalink\n  gitCommitHash\n  actor {\n    __typename\n    id\n    ... on UserActor {\n      username\n      __typename\n    }\n    ... on Robot {\n      firstName\n      __typename\n    }\n  }\n  branch {\n    id\n    name\n    __typename\n  }\n  codeSigningInfo {\n    keyid\n    sig\n    alg\n    __typename\n  }\n  rolloutPercentage\n  rolloutControlUpdate {\n    id\n    __typename\n  }\n  fingerprint {\n    id\n    hash\n    debugInfoUrl\n    source {\n      type\n      bucketKey\n      isDebugFingerprint\n      __typename\n    }\n    __typename\n  }\n  __typename\n}"

type updateResponse struct {
	Data struct {
		App struct {
			ById struct {
				UpdateBranchByName struct {
					UpdateGroups [][]Update `json:"updateGroups"`
				} `json:"updateBranchByName"`
			} `json:"byId"`
		} `json:"app"`
	} `json:"data"`
}

func (c *Client) FetchUpdates(ctx context.Context, projectId, branch string, limit, offset int) ([][]Update, error) {
	log.Printf("Fetching %d+%d updates for branch %s for app %s", offset, limit, branch, projectId)
	query := graphQLQuery[updateVariables]{
		OperationName: updateOperation,
		Query:         updateQuery,
		Variables: updateVariables{
			AppId:      projectId,
			BranchName: branch,
			Limit:      limit,
			Offset:     offset,
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
		return nil, fmt.Errorf("failed to fetch update channel: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read response: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		log.Printf("failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch update channel: %d: %s", resp.StatusCode, string(body))
	}
	if _, debug := os.LookupEnv("DEBUG"); debug {
		log.Printf("response body: %s", string(body))
	}

	var parsed updateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	log.Printf("Fetched %d update groups for branch %s for app %s", len(parsed.Data.App.ById.UpdateBranchByName.UpdateGroups), branch, projectId)
	return parsed.Data.App.ById.UpdateBranchByName.UpdateGroups, nil
}
