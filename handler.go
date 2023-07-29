package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	discordgo "github.com/bwmarrin/discordgo"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

func handleTwitter(s *discordgo.Session, m *discordgo.MessageCreate, scraper *twitterscraper.Scraper, u *url.URL) {
	if !strings.Contains(u.Path, "/status/") {
		return
	}

	id := strings.Split(u.Path, "/")[3]

	tweet, err := scraper.GetTweet(id)
	if err != nil {
		return
	}

	if len(tweet.Videos) == 0 {
		return
	}

	videoFiles := getTwitterVideoFiles(tweet)

	s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      videoFiles,
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
}

func handleInstagram(s *discordgo.Session, m *discordgo.MessageCreate, u *url.URL) {
	jsonData := getInstagramMedia(u.String())

	if jsonData["product_type"] == "carousel_container" {
		// collect all images into one message and all videos into another
		imageFiles, videoFiles := getInstagramCarouselFiles(jsonData)
		if len(imageFiles) > 0 {
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Files:      imageFiles,
				Components: []discordgo.MessageComponent{deleteMessageActionRow},
				Reference:  m.Reference(),
			})
		}
		if len(videoFiles) > 0 {
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Files:      videoFiles,
				Components: []discordgo.MessageComponent{deleteMessageActionRow},
				Reference:  m.Reference(),
			})
		}
	} else {
		if jsonData["video_versions"] != nil {
			videoFile := getInstagramVideoFile(jsonData["video_versions"].([]interface{})[0].(map[string]interface{})["url"].(string))
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Files:      videoFile,
				Components: []discordgo.MessageComponent{deleteMessageActionRow},
				Reference:  m.Reference(),
			})
		} else {
			imageFile := getInstagramImageFile(jsonData["image_versions"].(map[string]interface{})["candidates"].([]interface{})[0].(map[string]interface{})["url"].(string))
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Files:      imageFile,
				Components: []discordgo.MessageComponent{deleteMessageActionRow},
				Reference:  m.Reference(),
			})
		}
	}
}

func handleReddit(s *discordgo.Session, m *discordgo.MessageCreate, u *url.URL) {

	// remove query params
	u.RawQuery = ""

	// if url is v.redd.it then get the redirect url
	if u.Host == "v.redd.it" {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", u.String(), nil)
		req.Header.Add("user-agent", os.Getenv("USERAGENT"))
		req.Header.Add("sec-fetch-site", "same-origin")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
		}
		defer resp.Body.Close()

		u, err = url.Parse(resp.Request.URL.String())
		if err != nil {
			fmt.Println(err)
		}
	}

	guild, err := s.Guild(m.GuildID)
	if err != nil {
		fmt.Println(err)
	}

	videoLink := getRedditVideoLink(u.String())
	if strings.Contains(videoLink, "v.redd.it") {
		videoFile := getRedditVideoFile(videoLink)
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files:      videoFile,
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
	}
	if strings.Contains(videoLink, "twitch.tv") {
		videoFile := getTwitchClipFile(strings.Split(videoLink, "/")[3], guild)
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files:      videoFile,
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
	}
}

func handleTwitch(s *discordgo.Session, m *discordgo.MessageCreate, u *url.URL) {
	if u.Host == "clips.twitch.tv" {
		vodid := strings.Split(u.Path, "/")[1]

		guild, err := s.Guild(m.GuildID)
		if err != nil {
			fmt.Println(err)
		}

		videoFile := getTwitchClipFile(vodid, guild)

		// send message and handle filesize error
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files:      videoFile,
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
	} else if u.Host == "www.twitch.tv" {
		if !strings.Contains(u.Path, "/clip/") {
			return
		}
		vodid := strings.Split(u.Path, "/")[3]

		guild, err := s.Guild(m.GuildID)
		if err != nil {
			fmt.Println(err)
		}

		videoFile := getTwitchClipFile(vodid, guild)

		// send message and handle filesize error
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files:      videoFile,
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
	}
}
