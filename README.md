# expo-slack-webhook
Serverless webhook to bridge Expo notifications to Slack. For builds, we subscribe to `eas` webhooks natively, as per [the docs](https://docs.expo.dev/eas/webhooks/).

For OTA updates, as `eas` does not support any notifications, we implemented a quick facsimile with [a step](https://github.com/NWACus/avy/blob/5951fb61e2cdc39f875936e31f346c9485eed6dd/.github/actions/expo/update/action.yaml#L73-L79) using `curl` in the GitHub Action used to build the updates.

We use Slack's [blocks](https://docs.expo.dev/accounts/programmatic-access/#robot-users-and-access-tokens) abstraction to build messages.

## Setup

HMAC tokens can be generated with OpenSSL:

```shell
$ openssl rand -base64 128 | tr -d '\n' 
FgqMm/Bi2rIlvuaGQBEKMUDxUquxFn+T3I6m+o1ocOX4IPMZjzXOZt48wRpQoQ53TzDmmBI6lw81UnNfX84VtHwTU3BaP+h2gOEYg5Iv4Lh2QLS9/1SPhLsqQ8aHr5X7PUFRSpG1p0snhnNVXkKLhhrCblcaGf0/p/BERdG8pAo=
```

Expo robot tokens are set up per [the docs](https://docs.expo.dev/accounts/programmatic-access/#robot-users-and-access-tokens).

Slack integration uses [an app](https://api.slack.com/apps/A08K98W4ET0) with minimal permissions; an OAuth token is generated to use in the serverless function.

## Running

After getting the requisite secrets, start the server:

```shell
$ ALLOW_PREVIEWS=1 DEBUG=1 go run main.go --slack-token $SLACK_TOKEN --slack-channel $SLACK_CHANNEL --hmac-secret $EXPO_HMAC_TOKEN --expo-token $EXPO_ACCESS_TOKEN
```

## Testing

### Locally

Use the test program to send payloads to your webhook server:

```shell
$ go run test/main.go --endpoint http://localhost:8080/build --payload ./test/build.sample.json --hmac-secret $EXPO_HMAC_TOKEN
$ go run test/main.go --endpoint http://localhost:8080/submit --payload ./test/submit.sample.json --hmac-secret $EXPO_HMAC_TOKEN
$ go run test/main.go --endpoint http://localhost:8080/update --payload ./test/update.sample.json --hmac-secret $EXPO_HMAC_TOKEN
```

### On the web

Using [`ngrok`](https://ngrok.com/), forward the address that the server is listening for locally to the web, then send requests through `ngrok`'s servers.

## Developing

None of the GrapqhQL endpoints we use to talk to the Expo servers are documented anywhere, so [`mitmproxy`](https://mitmproxy.org/) was used to intercept traffic from the `eas` CLI. After adding the `mitmproxy` CA to the developer machine's root trust bundle, run the EAS CLI like so:

```shell
$ NODE_TLS_REJECT_UNAUTHORIZED=0 https_proxy=https://localhost:9090 eas update:list --branch=preview --limit=2 --non-interactive
```

## Deploying

We use Vercel's [serverless offering for Golang](https://vercel.com/docs/functions/runtimes/go) for no reason other than NWAC already has a business relationship with Vercel which makes this an easy on-ramp.