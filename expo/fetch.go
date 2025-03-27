package expo

type Client struct {
	Token string
}

const expoAPIURL = "https://api.expo.dev/graphql"

type graphQLQuery[V any] struct {
	OperationName string `json:"operationName"`
	Query         string `json:"query"`
	Variables     V      `json:"variables"`
}
