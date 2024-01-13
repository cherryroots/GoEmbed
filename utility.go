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
	"github.com/tidwall/gjson"
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
	regex := regexp.MustCompile(`(?m)\/r\/[a-zA-Z0-9_]+\/comments\/([a-zA-Z0-9_]+)`)
	result := regex.FindStringSubmatch(url)
	if len(result) > 1 {
		return result[1]
	}
	return ""
}

func getTikTokID(url string) string {
	// parse an url like : https://www.tiktok.com/@therock/video/6817233442149108486?lang=en
	// to get the video id : 6817233442149108486
	regex := regexp.MustCompile(`(?m)\/video\/([0-9]+)`)
	result := regex.FindStringSubmatch(url)
	if len(result) > 1 {
		return result[1]
	}
	return ""
}

func getTwitchVodId(url *url.URL) string {
	var vodid string
	if url.Host == "clips.twitch.tv" {
		vodid = strings.Split(url.Path, "/")[1]
	} else if url.Host == "www.twitch.tv" {
		if !strings.Contains(url.Path, "/clip/") {
			return ""
		}
		vodid = strings.Split(url.Path, "/")[3]
	}
	return vodid
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
	regex := regexp.MustCompile(`(?m)clips\.twitch\.tv\/([a-zA-Z0-9_-]+)|(?m)www\.twitch\.tv\/[a-zA-Z0-9_-]+\/clip\/([a-zA-Z0-9_-]+)`)
	result := regex.FindStringSubmatch(string(body))
	if len(result) > 1 {
		fullUrl := "https://" + result[0]
		u, _ := url.Parse(fullUrl)
		return u
	}

	regex = regexp.MustCompile(`(?m)dash_url\": \"(.*?)\"`)
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

func getInstagramPostLinks(body string) (images []string, videos []string) {
	mediaType := gjson.Get(body, "__typename").String()
	imageLinks := []string{}
	videoLinks := []string{}
	if mediaType == "GraphVideo" {
		link := gjson.Get(body, "video_url").String()
		videoLinks = append(videoLinks, link)
	}
	if mediaType == "GraphImage" {
		link := gjson.Get(body, "display_url").String()
		imageLinks = append(imageLinks, link)
	}
	if mediaType == "GraphSidecar" {
		sidecarLinks := gjson.Get(body, "edge_sidecar_to_children.edges").Array()
		for _, link := range sidecarLinks {
			if link.Get("node.__typename").String() == "GraphVideo" {
				videoLinks = append(videoLinks, link.Get("node.video_url").String())
			}
			if link.Get("node.__typename").String() == "GraphImage" {
				imageLinks = append(imageLinks, link.Get("node.display_url").String())
			}
		}
	}

	return imageLinks, videoLinks
}

func getTikTokVideoLink(id string) string {
	apiUrl := "https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/?aweme_id=" + id
	// get api response
	client := &http.Client{}
	req, _ := http.NewRequest("GET", apiUrl, nil)
	req.Header.Add("user-agent", "'User-Agent', 'TikTok 26.2.0 rv:262018 (iPhone; iOS 14.4.2; en_US) Cronet'")
	req.Header.Add("sec-fetch-site", "same-origin")

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}

	// parse the response json
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
	var jsonData map[string]interface{}
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		log.Println(err)
	}

	mediaUrl := jsonData["aweme_list"].([]interface{})[0].(map[string]interface{})["video"].(map[string]interface{})["play_addr"].(map[string]interface{})["url_list"].([]interface{})[0].(string)

	defer resp.Body.Close()
	return mediaUrl
}

func GetId(url string) string {
	regex := regexp.MustCompile(`/([a-zA-Z0-9_-]+)(?:\.[a-zA-Z0-9]+)?(?:\?|$|\/\?|\/$)`)
	result := regex.FindStringSubmatch(url)
	if len(result) > 1 {
		return result[1]
	}
	return ""
}

func compressVideo(file *os.File, guild *discordgo.Guild) {
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
	if fileInfo.Size() < int64(maxSize*1000000) {
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
	kbps := ((float64(maxSize) / duration) * 0.85) * 8000

	// compress video
	cmd = exec.Command("ffmpeg",
		"-i", file.Name(),
		"-b:v", fmt.Sprintf("%fk", kbps),
		"-minrate", fmt.Sprintf("%fk", kbps),
		"-maxrate", fmt.Sprintf("%fk", kbps),
		"-bufsize", fmt.Sprintf("%fk", kbps*2),
		"-preset", "fast",
		"-y", f.Name())
	err = cmd.Run()
	if err != nil {
		log.Println(err)
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
