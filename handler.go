package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

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
	var images []string
	var videos []string
	if strings.Contains(u.Path, "/stories/") {
		u.RawQuery = ""

		// get user from url
		user := strings.Split(u.Path, "/")[2]
		postid := strings.Split(u.Path, "/")[3]

		url := "https://instagram-scraper-2022.p.rapidapi.com/ig/user_id/?user=" + user
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Add("X-RapidAPI-Key", "66uH5cmqYXmshLAfsZcC3khMQsH1p1WK8Jjjsni8nzqEJ6lgM2")
		req.Header.Add("X-RapidAPI-Host", "instagram-scraper-2022.p.rapidapi.com")
		res1, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(err)
		}
		defer res1.Body.Close()
		body, _ := io.ReadAll(res1.Body)

		userId := gjson.Get(string(body), "id").String()
		// retry until we get a response with images or videos (sometimes it takes a few tries)

		for i := 0; i < 5; i++ {
			url = fmt.Sprintf("https://instagram-scraper-2022.p.rapidapi.com/ig/get_stories_hd/?id_user=%s&id_stories=%s", userId, postid)
			req, _ = http.NewRequest("GET", url, nil)
			req.Header.Add("X-RapidAPI-Key", "66uH5cmqYXmshLAfsZcC3khMQsH1p1WK8Jjjsni8nzqEJ6lgM2")
			req.Header.Add("X-RapidAPI-Host", "instagram-scraper-2022.p.rapidapi.com")

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Println(err)
			}
			defer res.Body.Close()
			body, _ = io.ReadAll(res.Body)

			// response is {"image": ""} or {"video": ""}
			if gjson.Get(string(body), "video").Exists() {
				videos = append(images, gjson.Get(string(body), "video").String())
			} else if gjson.Get(string(body), "image").Exists() {
				images = append(images, gjson.Get(string(body), "image").String())
			}

			// if the response contains {"answer": "bad"} then we retry
			if gjson.Get(string(body), "answer").Exists() && gjson.Get(string(body), "answer").String() == "bad" {
				time.Sleep(time.Duration(2*(i+1)) * time.Second)
				continue
			} else {
				if len(images) == 0 && len(videos) == 0 {
					// send message that the story is not available if we didn't get any images or videos
					msg, _ := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
						Content:   "This story is not available",
						Reference: m.Reference(),
					})
					// delete the response message after 5 seconds
					time.Sleep(5 * time.Second)
					s.ChannelMessageDelete(msg.ChannelID, msg.ID)
					break
				} else {
					break
				}
			}
		}

		if len(images) == 0 && len(videos) == 0 {
			return
		}
	}

	if strings.Contains(u.Path, "/p/") || strings.Contains(u.Path, "/tv/") || strings.Contains(u.Path, "/reel/") {

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

		images, videos = getInstagramPostLinks(string(body))
	}

	if len(images) == 0 && len(videos) == 0 {
		return
	}

	imageFiles, videoFiles := getInstagramFiles(images, videos)

	s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      append(imageFiles, videoFiles...),
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
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
