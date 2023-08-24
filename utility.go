package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	discordgo "github.com/bwmarrin/discordgo"
	"github.com/tidwall/gjson"
)

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

func getRedditVideoLink(u *url.URL) *url.URL {
	// Fetch data from Reddit post
	client := &http.Client{}
	req, _ := http.NewRequest("GET", u.String()+".json", nil)
	req.Header.Add("user-agent", os.Getenv("USERAGENT"))
	req.Header.Add("sec-fetch-site", "same-origin")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Get video link
	// regex for clips.twitch.tv
	regex := regexp.MustCompile(`(?m)clips\.twitch\.tv\/([a-zA-Z0-9_-]+)|(?m)www\.twitch\.tv\/[a-zA-Z0-9_-]+\/clip\/([a-zA-Z0-9_-]+)`)
	result := regex.FindStringSubmatch(string(body))
	if len(result) > 1 {
		fullUrl := "https://" + result[0]
		url, _ := url.Parse(fullUrl)
		return url
	}

	regex = regexp.MustCompile(`(?m)dash_url\": \"(.*?)\"`)
	result = regex.FindStringSubmatch(string(body))
	if len(result) > 1 {
		url, _ := url.Parse(result[1])
		return url
	}
	return nil
}

func getInstagramPostLinks(body string) (images []string, videos []string) {
	mediatype := gjson.Get(body, "__typename").String()
	imageLinks := []string{}
	videoLinks := []string{}
	if mediatype == "GraphVideo" {
		link := gjson.Get(body, "video_url").String()
		videoLinks = append(videoLinks, link)
	}
	if mediatype == "GraphImage" {
		link := gjson.Get(body, "display_url").String()
		imageLinks = append(imageLinks, link)
	}
	if mediatype == "GraphSidecar" {
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
		fmt.Println(err)
	}

	// parse the response json
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	var jsonData map[string]interface{}
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		fmt.Println(err)
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
		maxSize = 8 // MB
	} else if guild.PremiumTier == 1 {
		maxSize = 25 // MB
	} else if guild.PremiumTier == 2 {
		maxSize = 50 // MB
	} else if guild.PremiumTier == 3 {
		maxSize = 100 // MB
	}

	// check if video is smaller than max size
	fileInfo, _ := file.Stat()
	if fileInfo.Size() < int64(maxSize*1000000) {
		return
	}

	// make new tmp file
	f, err := os.CreateTemp("", "twitch*.mp4")
	if err != nil {
		fmt.Println(err)
	}

	// get video length in seconds and calculate a bitrate to compress to
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", file.Name())
	out, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		fmt.Println(err)
	}
	// megabyte per second
	MBps := (float64(maxSize) / duration) * 0.9
	// kilobit per second
	kbps := MBps * 8192

	// compress video
	cmd = exec.Command("ffmpeg", "-threads", "10", "-i", file.Name(), "-b:v", fmt.Sprintf("%fk", kbps), "-y", f.Name())
	err = cmd.Run()
	if err != nil {
		fmt.Println(err)
	}

	// replace old file with new file
	err = os.Remove(file.Name())
	if err != nil {
		fmt.Println(err)
	}
	err = os.Rename(f.Name(), file.Name())
	if err != nil {
		fmt.Println(err)
	}

}
