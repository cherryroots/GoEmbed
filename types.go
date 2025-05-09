package main

type twitchClipInfo struct {
	// First JSON Response Fields
	Title              string
	ThumbnailURL       string
	CreatedAt          string
	DurationSeconds    int
	ViewCount          int
	VideoOffsetSeconds *int
	Video              *string

	// Curator information
	Curator struct {
		ID          string
		DisplayName string
		Login       string
	}

	// Broadcaster information
	Broadcaster struct {
		ID          string
		DisplayName string
		Login       string
	}

	// Game information
	Game struct {
		ID          string
		DisplayName string
		BoxArtURL   string
	}

	// Second JSON Response Fields
	ID string

	// Playback access token
	PlaybackAccessToken struct {
		Signature string
		Value     string
	}

	// Video qualities
	VideoQualities []struct {
		FrameRate float64
		Quality   string
		SourceURL string
	}
}

type arazuVideoInfo struct {
	Title     string
	URL       string
	Channel   string
	RedditURL string
}
