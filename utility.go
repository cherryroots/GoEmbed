package main

import (
	"encoding/json"
	"fmt"
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

func supressEmbed(s *discordgo.Session, m *discordgo.Message) {
	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:      m.ID,
		Channel: m.ChannelID,
		Flags:   discordgo.MessageFlagsSuppressEmbeds,
	})
	if err != nil {
		return
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

	if guild.PremiumTier == 0 {
		maxSize = 8
	} else if guild.PremiumTier == 1 {
		maxSize = 25
	} else if guild.PremiumTier == 2 {
		maxSize = 50
	} else if guild.PremiumTier == 3 {
		maxSize = 100
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
