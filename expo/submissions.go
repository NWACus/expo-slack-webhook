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

type submissionVariables struct {
	Id string `json:"id"`
}

const submissionOperation = "SubmissionByIdQuery"
const submissionQuery = "query SubmissionByIdQuery($id: ID!) {\n  submissions {\n    byId(submissionId: $id) {\n      ...SubmissionFragment\n      __typename\n    }\n    __typename\n  }\n}\n\nfragment SubmissionFragment on Submission {\n  id\n  status\n  createdAt\n  updatedAt\n  platform\n  priority\n  app {\n    id\n    name\n    icon {\n      url\n      __typename\n    }\n    fullName\n    __typename\n  }\n  initiatingActor {\n    __typename\n    firstName\n    displayName\n    ... on UserActor {\n      username\n      fullName\n      profilePhoto\n      __typename\n    }\n  }\n  logFiles\n  error {\n    errorCode\n    message\n    __typename\n  }\n  submittedBuild {\n    ...Build\n    __typename\n  }\n  canRetry\n  childSubmission {\n    id\n    __typename\n  }\n  __typename\n}\n\nfragment Build on Build {\n  __typename\n  id\n  platform\n  status\n  app {\n    id\n    fullName\n    slug\n    name\n    iconUrl\n    githubRepository {\n      githubRepositoryUrl\n      __typename\n    }\n    ownerAccount {\n      name\n      __typename\n    }\n    __typename\n  }\n  artifacts {\n    applicationArchiveUrl\n    buildArtifactsUrl\n    xcodeBuildLogsUrl\n    __typename\n  }\n  distribution\n  logFiles\n  metrics {\n    buildWaitTime\n    buildQueueTime\n    buildDuration\n    __typename\n  }\n  initiatingActor {\n    id\n    displayName\n    ... on UserActor {\n      username\n      fullName\n      profilePhoto\n      __typename\n    }\n    ... on User {\n      primaryAccount {\n        profileImageUrl\n        __typename\n      }\n      __typename\n    }\n    ... on Robot {\n      isManagedByGitHubApp\n      __typename\n    }\n    __typename\n  }\n  createdAt\n  enqueuedAt\n  provisioningStartedAt\n  workerStartedAt\n  completedAt\n  updatedAt\n  expirationDate\n  sdkVersion\n  runtime {\n    ...RuntimeBasicInfo\n    __typename\n  }\n  channel\n  updateChannel {\n    id\n    name\n    __typename\n  }\n  fingerprint {\n    ...FingerprintData\n    __typename\n  }\n  buildProfile\n  appVersion\n  appBuildVersion\n  gitCommitHash\n  gitCommitMessage\n  isGitWorkingTreeDirty\n  message\n  resourceClassDisplayName\n  gitRef\n  projectRootDirectory\n  projectMetadataFileUrl\n  childBuild {\n    id\n    buildMode\n    __typename\n  }\n  priority\n  queuePosition\n  initialQueuePosition\n  estimatedWaitTimeLeftSeconds\n  submissions {\n    id\n    status\n    canRetry\n    __typename\n  }\n  canRetry\n  retryDisabledReason\n  maxRetryTimeMinutes\n  buildMode\n  customWorkflowName\n  isWaived\n  developmentClient\n  selectedImage\n  customNodeVersion\n  isForIosSimulator\n  resolvedEnvironment\n  cliVersion\n}\n\nfragment RuntimeBasicInfo on Runtime {\n  __typename\n  id\n  version\n  isFingerprint\n}\n\nfragment FingerprintData on Fingerprint {\n  __typename\n  id\n  hash\n  debugInfoUrl\n  createdAt\n}"

type submissionResponse struct {
	Data struct {
		Submissions struct {
			ById Submission `json:"byId"`
		} `json:"submissions"`
	} `json:"data"`
}

func (c *Client) FetchSubmission(ctx context.Context, id string) (*Submission, error) {
	log.Printf("Fetching submission %s", id)
	query := graphQLQuery[submissionVariables]{
		OperationName: submissionOperation,
		Query:         submissionQuery,
		Variables: submissionVariables{
			Id: id,
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
		return nil, fmt.Errorf("failed to fetch submissions: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read response: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		log.Printf("failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch submissions: %d: %s", resp.StatusCode, string(body))
	}
	if _, debug := os.LookupEnv("DEBUG"); debug {
		log.Printf("response body: %s", string(body))
	}

	var parsed submissionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	log.Printf("Fetched submissions %s, for build %s", parsed.Data.Submissions.ById.Id, FormatBuildVersion(parsed.Data.Submissions.ById.SubmittedBuild.BuildVersionMetadata))
	return &parsed.Data.Submissions.ById, nil
}
