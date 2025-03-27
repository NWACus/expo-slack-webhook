package expo

import "fmt"

func PlatformEmoji(platform Platform) string {
	switch platform {
	case PlatformAndroid:
		return ":android:"
	case PlatformIOS:
		return ":apple_logo:"
	}
	return ":grey_question:"
}

func PlatformDisplay(platform Platform) string {
	switch platform {
	case PlatformAndroid:
		return "Android"
	case PlatformIOS:
		return "iOS"
	}
	return "Unknown platform "
}

func StatusEmoji(status Status) string {
	switch status {
	case StatusFinished:
		return ":large_green_circle:"
	case StatusCancelled:
		return ":large_yellow_circle:"
	case StatusErrored:
		return ":red_circle:"
	}
	return ":black_circle:"
}

func StatusDisplay(status Status) string {
	switch status {
	case StatusFinished:
		return "succeeded"
	case StatusCancelled:
		return "cancelled"
	case StatusErrored:
		return "errored"
	}
	return "in an unknown state"
}

func FormatTitle(emoji, name string, platform Platform, status Status) string {
	return fmt.Sprintf(`%s %s %s | %s %s %s.`, emoji, PlatformEmoji(platform), StatusEmoji(status), PlatformDisplay(platform), name, StatusDisplay(status))
}

func FormatBuildVersion(build BuildVersionMetadata) string {
	return fmt.Sprintf(`%s (%s) [<https://github.com/NWACus/avy/commit/%s|%s>] @<https://expo.dev/accounts/nwac/projects/avalanche-forecast/channels/%s|%s>`, build.AppVersion, build.AppBuildVersion, build.GitCommitHash, build.GitCommitHash[0:7], build.Channel, build.Channel)
}