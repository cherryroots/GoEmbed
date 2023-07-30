package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"

	discordgo "github.com/bwmarrin/discordgo"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func getTwitterVideoFiles(tweet *twitterscraper.Tweet) []*discordgo.File {
	videoFiles := make([]*discordgo.File, 0)

	for _, video := range tweet.Videos {
		// Create a temp file starting with twitter and ending with .mp4
		f, err := os.CreateTemp("", "twitter*.mp4")
		if err != nil {
			continue
		}

		err = DownloadFile(f.Name(), video.URL)
		if err != nil {
			continue
		}

		file, err := os.Open(f.Name())
		if err != nil {
			continue
		}

		videoFiles = append(videoFiles, &discordgo.File{
			Name:   "video.mp4",
			Reader: file,
		})

		defer os.Remove(f.Name())
	}

	return videoFiles
}

func getInstagramVideoFile(url string) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "instagram*.mp4")
	if err != nil {
		return nil
	}

	err = DownloadFile(f.Name(), url)
	if err != nil {
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		return nil
	}

	videoFile := []*discordgo.File{
		{
			Name:   "video.mp4",
			Reader: file,
		},
	}

	defer os.Remove(f.Name())

	return videoFile
}

func getInstagramImageFile(url string) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "instagram*.jpg")
	if err != nil {
		return nil
	}

	err = DownloadFile(f.Name(), url)
	if err != nil {
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		return nil
	}

	imageFile := []*discordgo.File{
		{
			Name:   "image.jpg",
			Reader: file,
		},
	}

	defer os.Remove(f.Name())

	return imageFile
}

func getRedditVideoFile(url string) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "reddit*.mp4")
	if err != nil {
		return nil
	}

	err = DownloadFile(f.Name(), url)
	if err != nil {
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		return nil
	}

	videoFile := []*discordgo.File{
		{
			Name:   "video.mp4",
			Reader: file,
		},
	}

	defer os.Remove(f.Name())

	return videoFile
}

func getTwitchClipFile(vodid string, guild *discordgo.Guild) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "twitch*.mp4")
	if err != nil {
		return nil
	}

	cmd := exec.Command("./TwitchDownloaderCLI", "clipdownload", "--id", vodid, "-o", f.Name())
	err = cmd.Run()
	if err != nil {
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		return nil
	}

	// compress video if it is too big
	compressVideo(file, guild)

	file, err = os.Open(f.Name())
	if err != nil {
		return nil
	}

	videoFile := []*discordgo.File{
		{
			Name:   "video.mp4",
			Reader: file,
		},
	}

	defer os.Remove(f.Name())

	return videoFile
}

func getInstagramCarouselFiles(carousel map[string]interface{}) ([]*discordgo.File, []*discordgo.File) {
	imageFiles := make([]*discordgo.File, 0)
	videoFiles := make([]*discordgo.File, 0)

	var wg sync.WaitGroup
	for _, element := range carousel["carousel_media"].([]map[string]interface{}) {
		wg.Add(1)
		go func(element map[string]interface{}) {
			defer wg.Done()
			if element["video_versions"] != nil {
				videoFiles = append(videoFiles, getInstagramVideoFile(element["video_versions"].([]interface{})[0].(map[string]interface{})["url"].(string))...)
			} else {
				imageFiles = append(imageFiles, getInstagramImageFile(element["image_versions"].(map[string]interface{})["candidates"].([]interface{})[0].(map[string]interface{})["url"].(string))...)
			}
		}(element)
	}

	wg.Wait()
	return imageFiles, videoFiles
}

func getTikTokFile(url string) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "tiktok*.mp4")
	if err != nil {
		return nil
	}

	err = DownloadFile(f.Name(), url)
	if err != nil {
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		return nil
	}

	videoFile := []*discordgo.File{
		{
			Name:   "video.mp4",
			Reader: file,
		},
	}

	defer os.Remove(f.Name())

	return videoFile
}
