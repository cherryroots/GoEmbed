package main

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func handleTwitter(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	if !strings.Contains(u.Path, "/status/") {
		return
	}
	u.Host = "fxtwitter.com"
	postUser := strings.Split(u.Path, "/")[1]
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[Tweet • %s](%s)", postUser, u.String()),
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
		return
	}
	time.Sleep(time.Second * 1)
	supressEmbed(s, m)
}

func handleThreads(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	if !strings.Contains(u.Path, "/post/") {
		return
	}
	u.Host = "fixthreads.net"
	postUser := strings.Split(u.Path, "/")[1]
	postUser = strings.Replace(postUser, "@", "", -1)
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[Threads • %s](%s)", postUser, u.String()),
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
		return
	}
	time.Sleep(time.Second * 1)
	supressEmbed(s, m)
}

func handleBsky(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	if !strings.Contains(u.Path, "/post/") {
		return
	}
	u.Host = "bskyx.net"
	postUser := strings.Split(u.Path, "/")[2]
	postUser = strings.Replace(postUser, ".bsky.social", "", -1)
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[Bsky • %s](%s)", postUser, u.String()),
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
		return
	}
	time.Sleep(time.Second * 1)
	supressEmbed(s, m)
}

func handleInstagram(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	u.Host = "instagramez.com"
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[Instagram](%s)", u.String()),
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
		return
	}
	time.Sleep(time.Second * 1)
	supressEmbed(s, m)
}

func handleTiktok(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	u.Host = "a.tnktok.com"
	postUser := strings.Split(u.Path, "/")[1]
	postUser = strings.Replace(postUser, "@", "", -1)
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[TikTok • %s](%s)", postUser, u.String()),
		Components: []discordgo.MessageComponent{deleteMessageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Println(err)
		return
	}
	time.Sleep(time.Second * 1)
	supressEmbed(s, m)
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
		if vodID == "" {
			return
		}
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
	if vodID == "" {
		return
	}
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
