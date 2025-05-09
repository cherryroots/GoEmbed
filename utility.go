package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type redditAuth struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

func requestRedditToken() (redditAuth, error) {
	client := &http.Client{}
	data := strings.NewReader("grant_type=password&username=" + os.Getenv("REDDIT_USER") + "&password=" + os.Getenv("REDDIT_PASS"))
	req, _ := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", data)
	req.Header.Add("user-agent", "Discord Bot by Garnet_D")
	req.SetBasicAuth(os.Getenv("REDDIT_CLIENT"), os.Getenv("REDDIT_SECRET"))
	resp, err := client.Do(req)
	if err != nil {
		return redditAuth{}, err
	}
	defer resp.Body.Close()
	var auth redditAuth
	if err := json.NewDecoder(resp.Body).Decode(&auth); err != nil {
		return redditAuth{}, err
	}
	return auth, nil
}

func redditFullname(url string) string {
	// extract https://www.reddit.com/r/birdstakingthetrain/comments/195emed/okay_everybody_out/
	// to get the fullname : 195emed with regex
	regex := regexp.MustCompile(`(?m)/r/[a-zA-Z0-9_]+/comments/([a-zA-Z0-9_]+)`)
	result := regex.FindStringSubmatch(url)
	if len(result) > 1 {
		return result[1]
	}
	return ""
}

func setEmbedSuppression(s *discordgo.Session, m *discordgo.Message, suppress bool) {
	var flags discordgo.MessageFlags
	if suppress {
		flags |= discordgo.MessageFlagsSuppressEmbeds
	} else {
		flags = m.Flags&^discordgo.MessageFlagsSuppressEmbeds | discordgo.MessageFlagsSuppressNotifications
	}

	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:      m.ID,
		Channel: m.ChannelID,
		Flags:   flags,
	})
	if err != nil {
		log.Printf("Failed to suppress embed: %v", err)
		log.Printf("Message link: https://discord.com/channels/%s/%s/%s", m.GuildID, m.ChannelID, m.ID)
	}
}

func getTwitchVodID(url *url.URL) string {
	if url.Host == "clips.twitch.tv" {
		return strings.Split(url.Path, "/")[1]
	} else if url.Host == "www.twitch.tv" || url.Host == "twitch.tv" {
		if strings.Contains(url.Path, "/clip/") {
			return strings.Split(url.Path, "/")[3]
		}
	}
	return ""
}

func getTwitchClipInfo(vodID string) (*twitchClipInfo, error) {
	// Run the TwitchDownloaderCLI info command to get clip info
	cmd := exec.Command("./TwitchDownloaderCLI/TwitchDownloaderCLI", "info", "--id", vodID, "--format", "raw")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to get Twitch clip info for ID %s: %s", vodID, err)
		log.Println(string(output))
		return nil, err
	}

	// Split the output into lines
	lines := strings.Split(string(output), "\n")

	// Find the lines containing JSON responses
	var jsonLines []string
	for _, line := range lines {
		// Check if line contains a JSON object starting with {"data":{"clip"
		if strings.Contains(line, "{\"data\":{\"clip\"") {
			jsonLines = append(jsonLines, line)
		}
	}

	// We expect two JSON responses
	if len(jsonLines) != 2 {
		return nil, fmt.Errorf("unexpected number of JSON responses: got %d, want 2", len(jsonLines))
	}

	// Process the first JSON response (basic clip info)
	type firstResponse struct {
		Data struct {
			Clip struct {
				Title        string `json:"title"`
				ThumbnailURL string `json:"thumbnailURL"`
				CreatedAt    string `json:"createdAt"`
				Curator      struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
					Login       string `json:"login"`
				} `json:"curator"`
				DurationSeconds int `json:"durationSeconds"`
				Broadcaster     struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
					Login       string `json:"login"`
				} `json:"broadcaster"`
				VideoOffsetSeconds *int        `json:"videoOffsetSeconds"`
				Video              interface{} `json:"video"`
				ViewCount          int         `json:"viewCount"`
				Game               struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
					BoxArtURL   string `json:"boxArtURL"`
				} `json:"game"`
			} `json:"clip"`
		} `json:"data"`
	}

	// Process the second JSON response (playback info)
	type secondResponse struct {
		Data struct {
			Clip struct {
				ID                  string `json:"id"`
				PlaybackAccessToken struct {
					Signature string `json:"signature"`
					Value     string `json:"value"`
				} `json:"playbackAccessToken"`
				VideoQualities []struct {
					FrameRate float64 `json:"frameRate"`
					Quality   string  `json:"quality"`
					SourceURL string  `json:"sourceURL"`
				} `json:"videoQualities"`
			} `json:"clip"`
		} `json:"data"`
	}

	var first firstResponse
	var second secondResponse

	// Parse the first JSON response
	err = json.Unmarshal([]byte(jsonLines[0]), &first)
	if err != nil {
		return nil, fmt.Errorf("failed to parse first JSON response: %w", err)
	}

	// Parse the second JSON response
	err = json.Unmarshal([]byte(jsonLines[1]), &second)
	if err != nil {
		return nil, fmt.Errorf("failed to parse second JSON response: %w", err)
	}

	// Create and populate the twitchClipInfo struct
	clipInfo := &twitchClipInfo{
		Title:              first.Data.Clip.Title,
		ThumbnailURL:       first.Data.Clip.ThumbnailURL,
		CreatedAt:          first.Data.Clip.CreatedAt,
		DurationSeconds:    first.Data.Clip.DurationSeconds,
		ViewCount:          first.Data.Clip.ViewCount,
		VideoOffsetSeconds: first.Data.Clip.VideoOffsetSeconds,
		ID:                 second.Data.Clip.ID,
	}

	// Handle the Video field separately - it could be null or an object
	if videoVal, ok := first.Data.Clip.Video.(string); ok {
		clipInfo.Video = &videoVal
	}

	// Copy nested struct fields individually to avoid type mismatch
	clipInfo.Curator.ID = first.Data.Clip.Curator.ID
	clipInfo.Curator.DisplayName = first.Data.Clip.Curator.DisplayName
	clipInfo.Curator.Login = first.Data.Clip.Curator.Login

	clipInfo.Broadcaster.ID = first.Data.Clip.Broadcaster.ID
	clipInfo.Broadcaster.DisplayName = first.Data.Clip.Broadcaster.DisplayName
	clipInfo.Broadcaster.Login = first.Data.Clip.Broadcaster.Login

	clipInfo.Game.ID = first.Data.Clip.Game.ID
	clipInfo.Game.DisplayName = first.Data.Clip.Game.DisplayName
	clipInfo.Game.BoxArtURL = first.Data.Clip.Game.BoxArtURL

	clipInfo.PlaybackAccessToken.Signature = second.Data.Clip.PlaybackAccessToken.Signature
	clipInfo.PlaybackAccessToken.Value = second.Data.Clip.PlaybackAccessToken.Value

	// Copy video qualities
	for _, quality := range second.Data.Clip.VideoQualities {
		clipInfo.VideoQualities = append(clipInfo.VideoQualities, struct {
			FrameRate float64
			Quality   string
			SourceURL string
		}{
			FrameRate: quality.FrameRate,
			Quality:   quality.Quality,
			SourceURL: quality.SourceURL,
		})
	}

	return clipInfo, nil
}

func getArazuVideoInfo(u *url.URL) (*arazuVideoInfo, error) {
	// Create an HTTP client and make a request to the URL
	client := &http.Client{}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set a user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Initialize video info with default values
	info := &arazuVideoInfo{}

	// Extract the title from h2 element with id="clip-title"
	titleRegex := regexp.MustCompile(`<h2 id="clip-title">(.*?)</h2>`)
	titleMatches := titleRegex.FindStringSubmatch(string(body))
	if len(titleMatches) > 1 {
		// Unescape HTML entities in title (e.g., &#39; to ')
		info.Title = html.UnescapeString(titleMatches[1])
	}
	
	// Extract the channel name from the anchor tag
	channelRegex := regexp.MustCompile(`<a data-umami-event="channel_click".*?href="/\?channel=(.*?)&`)
	channelMatches := channelRegex.FindStringSubmatch(string(body))
	if len(channelMatches) > 1 {
		// Unescape HTML entities in channel name
		info.Channel = html.UnescapeString(channelMatches[1])
	}

	// Extract the video URL from the poster attribute and replace .webp with .mp4
	posterRegex := regexp.MustCompile(`<video id="video-player".*?poster="(https://cdn\.arazu\.io/.*?\.webp)"`)
	posterMatches := posterRegex.FindStringSubmatch(string(body))
	if len(posterMatches) > 1 {
		// Replace .webp with .mp4 to get the video URL
		posterURL := posterMatches[1]
		info.URL = strings.Replace(posterURL, ".webp", ".mp4", 1)
	}
	
	// Extract the Reddit link
	redditRegex := regexp.MustCompile(`<a data-umami-event="reddit_click".*?href="(https://old\.reddit\.com/.*?/)"`)
	redditMatches := redditRegex.FindStringSubmatch(string(body))
	if len(redditMatches) > 1 {
		info.RedditURL = redditMatches[1]
	}

	// Return an error if we couldn't find any information
	if info.Title == "" && info.URL == "" {
		return nil, fmt.Errorf("could not extract title or image URL from page")
	}

	return info, nil
}

func getRedditVideoLink(u *url.URL, auth redditAuth) *url.URL {
	// Fetch data from Reddit post
	client := &http.Client{}
	u.Host = "oauth.reddit.com"
	// get reddit post data from the api using the full url
	req, _ := http.NewRequest("GET", "https://oauth.reddit.com/api/info?id=t3_"+redditFullname(u.String()), nil)
	req.Header.Add("Authorization", fmt.Sprintf("bearer %s", auth.AccessToken))
	req.Header.Add("user-agent", "Discord Bot by Garnet_D")
	req.Header.Add("sec-fetch-site", "same-origin")

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Get video link
	// regex for clips.twitch.tv
	regex := regexp.MustCompile(`(?m)clips\.twitch\.tv/([a-zA-Z0-9_-]+)|(?m)www\.twitch\.tv/[a-zA-Z0-9_-]+/clip/([a-zA-Z0-9_-]+)`)
	result := regex.FindStringSubmatch(string(body))
	if len(result) > 1 {
		fullURL := "https://" + result[0]
		u, _ := url.Parse(fullURL)
		return u
	}

	regex = regexp.MustCompile(`(?m)dash_url": "(.*?)"`)
	result = regex.FindStringSubmatch(string(body))
	if len(result) > 1 {
		u, _ := url.Parse(result[1])
		return u
	}
	return nil
}

func followRedirect(u *url.URL, auth redditAuth) (*url.URL, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("HEAD", u.String(), nil)
	req.Header.Set("Authorization", "bearer "+auth.AccessToken)
	resp, err := client.Do(req)
	if err != nil {
		return u, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		u, err = url.Parse(resp.Header.Get("Location"))
		if err != nil {
			return u, err
		}
	} else {
		u = resp.Request.URL
	}
	return u, nil
}

func compressVideo(file *os.File, guild *discordgo.Guild, force bool) {
	var maxSize int

	switch guild.PremiumTier {
	case 0:
		maxSize = 10 // 10 MB
	case 1:
		maxSize = 10 // 10 MB
	case 2:
		maxSize = 50 // 50 MB
	case 3:
		maxSize = 100 // 100 MB
	}

	// check if video is smaller than max size
	fileInfo, _ := file.Stat()
	if fileInfo.Size() < int64(maxSize*1000000) && !force {
		log.Printf("Video size is smaller than max size: %d of %d MB", fileInfo.Size()/1000000, maxSize)
		return
	}

	// make new tmp file
	f, err := os.CreateTemp("", "compress*.mp4")
	if err != nil {
		log.Println(err)
	}

	// get video length in seconds and calculate a bitrate to compress to
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", file.Name())
	out, err := cmd.Output()
	if err != nil {
		log.Println(err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		log.Println(err)
	}
	// megabyte per second
	kbps := ((float64(maxSize) / duration) * 0.80) * 8000

	// compress video
	os.Setenv("LIBVA_DRIVER_NAME", "iHD")
	cmd = exec.Command(
		"ffmpeg",
		"-hwaccel", "vaapi",
		"-hwaccel_device", "/dev/dri/renderD128",
		"-hwaccel_output_format", "vaapi",
		"-i", file.Name(),
		"-vf", "format=nv12|vaapi,hwupload",
		"-c:v", "h264_vaapi",
		"-b:v", fmt.Sprintf("%fk", kbps),
		"-minrate", fmt.Sprintf("%fk", kbps),
		"-maxrate", fmt.Sprintf("%fk", kbps),
		"-bufsize", fmt.Sprintf("%fk", kbps*2),
		"-preset", "veryfast",
		"-y", f.Name())
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf(string(out))
		log.Printf("Error compressing video: %v\n", err)
	}

	// replace old file with new file
	err = os.Remove(file.Name())
	if err != nil {
		log.Println(err)
	}
	err = os.Rename(f.Name(), file.Name())
	if err != nil {
		log.Println(err)
	}
}
