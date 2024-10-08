package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	twitterscraper "github.com/n0madic/twitter-scraper"
	"github.com/tidwall/gjson"
)

func handleTwitter(s *discordgo.Session, m *discordgo.Message, scraper *twitterscraper.Scraper, u *url.URL) {
	if !strings.Contains(u.Path, "/status/") {
		return
	}

	guild, err := s.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
	}

	id := strings.Split(u.Path, "/")[3]

	tweet, err := scraper.GetTweet(id)
	if err != nil {
		log.Println(err)
		return
	}

	if tweet.IsQuoted {
		if len(tweet.QuotedStatus.Videos) == 0 && len(tweet.QuotedStatus.GIFs) == 0 {
			return
		}

		videoFiles := getTwitterVideoFiles(tweet.QuotedStatus, guild)

		if len(videoFiles) == 0 {
			_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:    "Could not fetch post. Please retry",
				Components: []discordgo.MessageComponent{deleteMessageActionRow},
				Reference:  m.Reference(),
			})
			if err != nil {
				log.Println(err)
			}
			return
		}

		_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files:      videoFiles,
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Println(err)
		}
	}

	if len(tweet.Videos) == 0 && len(tweet.GIFs) == 0 {
		return
	}

	videoFiles := getTwitterVideoFiles(tweet, guild)

	if len(videoFiles) == 0 {
		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Content:    "Could not fetch post. Please retry",
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Println(err)
		}
		return
	}

	_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      videoFiles,
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
	}
}

func handleInstagram(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	var images []string
	var videos []string
	if strings.Contains(u.Path, "/stories/") {
		u.RawQuery = ""

		// get user from u2
		user := strings.Split(u.Path, "/")[2]
		postID := strings.Split(u.Path, "/")[3]

		u2 := "https://instagram-scraper-2022.p.rapidapi.com/ig/user_id/?user=" + user
		req, _ := http.NewRequest("GET", u2, nil)
		req.Header.Add("X-RapidAPI-Key", os.Getenv("RAPIDAPI_KEY"))
		req.Header.Add("X-RapidAPI-Host", "instagram-scraper-2022.p.rapidapi.com")
		res1, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println(err)
		}
		defer res1.Body.Close()
		body, _ := io.ReadAll(res1.Body)

		userID := gjson.Get(string(body), "id").String()
		// retry until we get a response with images or videos (sometimes it takes a few tries)

		// sleep for 1 second
		time.Sleep(1 * time.Second)

		for i := 0; i < 5; i++ {
			u2 = fmt.Sprintf("https://instagram-scraper-2022.p.rapidapi.com/ig/get_stories_hd/?id_user=%s&id_stories=%s", userID, postID)
			req, _ = http.NewRequest("GET", u2, nil)
			req.Header.Add("X-RapidAPI-Key", os.Getenv("RAPIDAPI_KEY"))
			req.Header.Add("X-RapidAPI-Host", "instagram-scraper-2022.p.rapidapi.com")

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Println(err)
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
			}
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
			}
			break
		}

		if len(images) == 0 && len(videos) == 0 {
			return
		}
	}

	if strings.Contains(u.Path, "/p/") || strings.Contains(u.Path, "/tv/") || strings.Contains(u.Path, "/reel/") {

		// get id from u2
		id := strings.Split(u.Path, "/")[2]

		u2 := "https://instagram-scraper-2022.p.rapidapi.com/ig/post_info/?shortcode=" + id

		req, _ := http.NewRequest("GET", u2, nil)

		req.Header.Add("X-RapidAPI-Key", os.Getenv("RAPIDAPI_KEY"))
		req.Header.Add("X-RapidAPI-Host", "instagram-scraper-2022.p.rapidapi.com")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println(err)
		}

		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)

		images, videos = getInstagramPostLinks(string(body))
	}

	if len(images) == 0 && len(videos) == 0 {
		return
	}

	imageFiles, videoFiles := getInstagramFiles(images, videos)

	if len(imageFiles) == 0 && len(videoFiles) == 0 {
		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Content:    "Could not fetch post. Please retry",
			Components: []discordgo.MessageComponent{instagramActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Println(err)
		}
		return
	}

	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      append(imageFiles, videoFiles...),
		Components: []discordgo.MessageComponent{instagramActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
	}
}

func handleReddit(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	// remove query params
	u.RawQuery = ""

	auth, err := requestRedditToken()
	if err != nil {
		log.Println(err)
		return
	}

	// if url is v.redd.it then get the redirect url
	if u.Host == "v.redd.it" {
		// get /video url
		u, err = followRedirect(u, auth)
		if err != nil {
			log.Println(err)
			return
		}
		// get full url
		u, err = followRedirect(u, auth)
		if err != nil {
			log.Println(err)
			return
		}
	} else if u.Host == "www.reddit.com" && strings.Contains(u.Path, "/s/") {
		// get full url
		u, err = followRedirect(u, auth)
		if err != nil {
			log.Println(err)
			return
		}
	}

	guild, err := s.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
	}

	videoLink := getRedditVideoLink(u, auth)
	if videoLink == nil {
		log.Println(err)
		return
	}
	if videoLink.Host == "v.redd.it" {
		videoFile := getRedditVideoFile(videoLink.String(), guild)

		if len(videoFile) == 0 {
			_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:    "Could not fetch post. Please retry",
				Components: []discordgo.MessageComponent{deleteMessageActionRow},
				Reference:  m.Reference(),
			})
			if err != nil {
				log.Println(err)
			}
			return
		}

		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files:      videoFile,
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Println(err)
		}
	}
	if videoLink.Host == "www.twitch.tv" || videoLink.Host == "clips.twitch.tv" {
		vodID := getTwitchVodID(videoLink)
		videoFile := getTwitchClipFile(vodID, guild)

		if len(videoFile) == 0 {
			_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:    "Could not fetch post. Please retry",
				Components: []discordgo.MessageComponent{deleteMessageActionRow},
				Reference:  m.Reference(),
			})
			if err != nil {
				log.Println(err)
			}
			return
		}

		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files:      videoFile,
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Println(err)
		}
	}
}

func handleTwitch(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	guild, err := s.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
	}

	vodID := getTwitchVodID(u)
	videoFile := getTwitchClipFile(vodID, guild)

	if len(videoFile) == 0 {
		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Content:    "Could not fetch post. Please retry",
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Println(err)
		}
		return
	}

	// send message and handle filesize error
	_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      videoFile,
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
	}
}

func handleTiktok(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	// follow url and get the redirect url
	if u.Host == "vm.tiktok.com" {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", u.String(), nil)
		req.Header.Add("user-agent", os.Getenv("USERAGENT"))
		req.Header.Add("sec-fetch-site", "same-origin")

		resp, err := client.Do(req)
		if err != nil {
			log.Println(err)
		}
		defer resp.Body.Close()

		u, err = url.Parse(resp.Request.URL.String())
		if err != nil {
			log.Println(err)
		}
	}

	guild, err := s.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
	}

	// get the video linl
	mediaURL := getTikTokVideoLink(u.String())

	// get the video file
	videoFile := getTikTokFile(mediaURL, guild)

	if len(videoFile) == 0 {
		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Content:    "Could not fetch post. Please retry",
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Println(err)
		}
		return
	}

	// send message and handle filesize error
	_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      videoFile,
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
	}
}

func handleVimeo(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	guild, err := s.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
	}

	// get the video file
	videoFile := getVimeoFile(u.String(), guild)

	if len(videoFile) == 0 {
		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Content:    "Could not fetch post. Please retry",
			Components: []discordgo.MessageComponent{deleteMessageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Println(err)
		}
		return
	}

	// send message and handle filesize error
	_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      videoFile,
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
	}
}
