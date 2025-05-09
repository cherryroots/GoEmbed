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
		Components: []discordgo.MessageComponent{messageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Printf("Failed to send twitter post: %v", err)
		return
	}
	time.Sleep(time.Millisecond * 250)
	setEmbedSuppression(s, m, true)
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
		Components: []discordgo.MessageComponent{messageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Printf("Failed to send threads post: %v", err)
		return
	}
	time.Sleep(time.Millisecond * 250)
	setEmbedSuppression(s, m, true)
}

func handleBsky(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	if !strings.Contains(u.Path, "/post/") {
		return
	}
	u.Host = "bskyx.app"
	postUser := strings.Split(u.Path, "/")[2]
	postUser = strings.Replace(postUser, ".bsky.social", "", -1)
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[Bsky • %s](%s)", postUser, u.String()),
		Components: []discordgo.MessageComponent{messageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Printf("Failed to send bsky post: %v", err)
		return
	}
	time.Sleep(time.Millisecond * 250)
	setEmbedSuppression(s, m, true)
}

func handleInstagram(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	if !strings.Contains(u.Path, "/p/") && !strings.Contains(u.Path, "/reel/") && !strings.Contains(u.Path, "/reels/") {
		return
	}
	u.Host = "ddinstagram.com"
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[Instagram](%s)", u.String()),
		Components: []discordgo.MessageComponent{messageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Printf("Failed to send instagram post: %v", err)
		return
	}
	time.Sleep(time.Millisecond * 250)
	setEmbedSuppression(s, m, true)
}

func handleTiktok(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	u.Host = "a.tnktok.com"
	postUser := strings.Split(u.Path, "/")[1][1:]
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[TikTok • %s](%s)", postUser, u.String()),
		Components: []discordgo.MessageComponent{messageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Printf("Failed to send tiktok post: %v", err)
		return
	}
	time.Sleep(time.Millisecond * 250)
	setEmbedSuppression(s, m, true)
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
				Components: []discordgo.MessageComponent{messageActionRow},
				Reference:  m.Reference(),
			})
			if err != nil {
				log.Println(err)
			}
			return
		}

		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files:      videoFile,
			Components: []discordgo.MessageComponent{messageActionRow},
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
				Components: []discordgo.MessageComponent{messageActionRow},
				Reference:  m.Reference(),
			})
			if err != nil {
				log.Printf("Failed to get reddit video file: %v", err)
			}
			return
		}

		info, err := getTwitchClipInfo(vodID)
		if err != nil {
			log.Println(err)
			return
		}

		_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Content:    fmt.Sprintf("%s playing %s • %s\n%ds • %d views", info.Broadcaster.DisplayName, info.Game.DisplayName, info.Title, info.DurationSeconds, info.ViewCount),
			Files:      videoFile,
			Components: []discordgo.MessageComponent{messageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Printf("Failed to send twitch clip post: %v", err)
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
			Components: []discordgo.MessageComponent{messageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Printf("Failed to fetch twitch clip: %v", err)
		}
		return
	}

	info, err := getTwitchClipInfo(vodID)
	if err != nil {
		return
	}

	// send message and handle filesize error
	_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("%s playing %s • %s\n%ds • %d views", info.Broadcaster.DisplayName, info.Game.DisplayName, info.Title, info.DurationSeconds, info.ViewCount),
		Files:      videoFile,
		Components: []discordgo.MessageComponent{messageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Printf("Twitch file too large: %v", err)
	}
	time.Sleep(time.Millisecond * 250)
	setEmbedSuppression(s, m, true)
}

func handleArazu(s *discordgo.Session, m *discordgo.Message, u *url.URL) {
	info, err := getArazuVideoInfo(u)
	if err != nil {
		log.Println(err)
		return
	}
	_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("[Arazu link](<%s>) [CDN](%s)\n%s • %s", u.String(), info.URL, info.Channel, info.Title),
		Components: []discordgo.MessageComponent{messageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Printf("Failed to send arazu post: %v", err)
		return
	}
	time.Sleep(time.Millisecond * 250)
	setEmbedSuppression(s, m, true)
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
			Components: []discordgo.MessageComponent{messageActionRow},
			Reference:  m.Reference(),
		})
		if err != nil {
			log.Printf("Failed to get vimeo file: %v", err)
		}
		return
	}

	// send message and handle filesize error
	_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files:      videoFile,
		Components: []discordgo.MessageComponent{messageActionRow},
		Reference:  m.Reference(),
	})
	if err != nil {
		log.Printf("Failed to send vimeo post: %v", err)
	}
}
