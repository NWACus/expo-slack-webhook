package expo

import "strings"

type Platform string

const (
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
)

func (p Platform) Equal(other Platform) bool {
	return strings.EqualFold(string(p), string(other))
}

type Status string

const (
	StatusFinished  Status = "finished"
	StatusCancelled Status = "cancelled"
	StatusErrored   Status = "errored"
)

func (p Status) Equal(other Status) bool {
	return strings.EqualFold(string(p), string(other))
}

type Error struct {
	Message   string `json:"message"`
	ErrorCode string `json:"errorCode"`
}

func (e Error) Failed() bool {
	return e.ErrorCode != "" || e.Message != ""
}

func (e Error) Error() string {
	return e.ErrorCode + ": " + e.Message
}

type Build struct {
	Id        string   `json:"id"`
	Status    Status   `json:"status"`
	Platform  Platform `json:"platform"`
	Error     Error    `json:"error"`
	CreatedAt string   `json:"createdAt"`

	BuildVersionMetadata `json:",inline"`
}

type BuildVersionMetadata struct {
	Channel         string `json:"channel"`
	AppVersion      string `json:"appVersion"`
	AppBuildVersion string `json:"appBuildVersion"`
	GitCommitHash   string `json:"gitCommitHash"`
}

type UpdateChannel struct {
	Id             string         `json:"id"`
	Name           string         `json:"name"`
	UpdateBranches []UpdateBranch `json:"updateBranches"`
}

type UpdateBranch struct {
	Id           string     `json:"id"`
	Name         string     `json:"name"`
	UpdateGroups [][]Update `json:"updateGroups"`
}

type Update struct {
	Id            string         `json:"id"`
	Group         string         `json:"group"`
	Platform      Platform       `json:"platform"`
	GitCommitHash string         `json:"gitCommitHash"`
	Branch        BranchFragment `json:"branch"`
	CreatedAt     string         `json:"createdAt"`
}

type BranchFragment struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Submission struct {
	Id string `json:"id"`
	App App `json:"app"`
	SubmittedBuild Build `json:"submittedBuild"`
}

type App struct {
	Id string `json:"id"`
	Name string `json:"name"`
}