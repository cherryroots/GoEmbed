package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	discordgo "github.com/bwmarrin/discordgo"
	twitterscraper "github.com/n0madic/twitter-scraper"
	"github.com/tidwall/gjson"
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

	if len(tweet.Videos) == 0 && len(tweet.GIFs) == 0 {
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
	// get id from url
	id := strings.Split(u.Path, "/")[2]

	url := "https://instagram-scraper-2022.p.rapidapi.com/ig/post_info/?shortcode=" + id

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("X-RapidAPI-Key", os.Getenv("RAPIDAPI_KEY"))
	req.Header.Add("X-RapidAPI-Host", "instagram-scraper-2022.p.rapidapi.com")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	images, videos := getInstagramLinks(string(body))

	imageFiles, videoFiles := getInstagramFiles(images, videos)

	s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      append(imageFiles, videoFiles...),
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
}

func getInstagramLinks(body string) (images []string, videos []string) {
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
		fmt.Println(videoFile)
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

func handleTiktok(s *discordgo.Session, m *discordgo.MessageCreate, u *url.URL) {
	// follow url and get the redirect url
	if u.Host == "vm.tiktok.com" {
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

	videoId := getTikTokID(u.String())

	// get the video linl
	mediaUrl := getTikTokVideoLink(videoId)

	// get the video file
	videoFile := getTikTokFile(mediaUrl)

	// send message and handle filesize error
	s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      videoFile,
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
}
