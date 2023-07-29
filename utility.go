package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	discordgo "github.com/bwmarrin/discordgo"
)

func getInstagramMedia(url string) map[string]interface{} {
	userAgent := os.Getenv("USERAGENT")
	cookie := os.Getenv("COOKIE")
	xIgAppId := os.Getenv("XIGAPPID")
	idUrl := GetId(url)

	// Fetch data from Instagram post
	if idUrl == "" {
		fmt.Println("Invalid URL")
	} else {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", fmt.Sprintf(`https://www.instagram.com/p/%s?__a=1&__d=di`, idUrl), nil)
		req.Header.Add("cookie", cookie)
		req.Header.Add("user-agent", userAgent)
		req.Header.Add("x-ig-app-id", xIgAppId)
		req.Header.Add("sec-fetch-site", "same-origin")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var dat map[string]interface{}
		json.Unmarshal(body, &dat)

		// Check if post is a carousel
		items := dat["items"].([]interface{})[0].(map[string]interface{})
		carouselMedia := make([]map[string]interface{}, 0)
		if items["product_type"] == "carousel_container" {
			for _, element := range items["carousel_media"].([]interface{}) {
				carouselMedia = append(carouselMedia, map[string]interface{}{"image_versions": element.(map[string]interface{})["image_versions2"], "video_versions": element.(map[string]interface{})["video_versions"]})
			}
		}

		// Create JSON data
		jsonData := map[string]interface{}{
			"code":                items["code"],
			"created_at":          items["taken_at"],
			"username":            items["user"].(map[string]interface{})["username"],
			"full_name":           items["user"].(map[string]interface{})["full_name"],
			"profile_picture":     items["user"].(map[string]interface{})["profile_pic_url"],
			"is_verified":         items["user"].(map[string]interface{})["is_verified"],
			"is_paid_partnership": items["is_paid_partnership"],
			"product_type":        items["product_type"],
			"caption":             items["caption"].(map[string]interface{})["text"],
			"like_count":          items["like_count"],
			"comment_count":       items["comment_count"],
			"view_count":          items["view_count"],
			"video_duration":      items["video_duration"],
			"location":            items["location"],
			"height":              items["original_height"],
			"width":               items["original_width"],
			"image_versions":      items["image_versions2"],
			"video_versions":      items["video_versions"],
			"carousel_media":      carouselMedia,
		}

		return jsonData
	}
	return map[string]interface{}{
		"code":                "",
		"created_at":          "",
		"username":            "",
		"full_name":           "",
		"profile_picture":     "",
		"is_verified":         "",
		"is_paid_partnership": "",
		"product_type":        "",
		"caption":             "",
		"like_count":          "",
		"comment_count":       "",
		"view_count":          "",
		"video_duration":      "",
		"location":            "",
		"height":              "",
		"width":               "",
		"image_versions":      "",
		"video_versions":      "",
		"carousel_media":      "",
	}
}

func getRedditVideoLink(url string) string {
	// Fetch data from Reddit post
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url+".json", nil)
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
		return result[0]
	}

	regex = regexp.MustCompile(`(?m)fallback_url\": \"(.*?)\"`)
	result = regex.FindStringSubmatch(string(body))
	if len(result) > 1 {
		return result[1]
	}
	return ""
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
		maxSize = 25 // MB
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
