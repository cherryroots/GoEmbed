package main

import (
	"log"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var deleteMessageButton = discordgo.Button{
	Label:    "Delete Message",
	Style:    discordgo.DangerButton,
	CustomID: "delete_message",
	Emoji:    &discordgo.ComponentEmoji{Name: "❌"},
}

var retryButton = discordgo.Button{
	Label:    "Retry",
	Style:    discordgo.PrimaryButton,
	CustomID: "retry",
	Emoji:    &discordgo.ComponentEmoji{Name: "🔁"},
}

var unsuppressButton = discordgo.Button{
	Label:    "Unsuppress",
	Style:    discordgo.PrimaryButton,
	CustomID: "unsuppress",
	Emoji:    &discordgo.ComponentEmoji{Name: "🌟"},
}

var messageActionRow = discordgo.ActionsRow{
	Components: []discordgo.MessageComponent{retryButton, unsuppressButton, deleteMessageButton},
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func main() {
	Token := os.Getenv("TOKEN")
	// TwUsername := os.Getenv("TWUSERNAME")
	// TwPassword := os.Getenv("TWPASSWORD")

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		interactionCreate(s, i)
	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages
	dg.ShouldReconnectOnError = true

	err = dg.Open()
	if err != nil {
		log.Println("error opening connection,", err)
		return
	}

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()

	defer log.Println("Bot is now offline.")
}

func handleURL(s *discordgo.Session, m *discordgo.Message, link string) {
	u, err := url.Parse(link)
	if err != nil {
		return
	}

	if m.GuildID == "" {
		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			log.Println(err)
		}
		m.GuildID = channel.GuildID
	}

	if u.Host == "twitter.com" || u.Host == "mobile.twitter.com" || u.Host == "www.twitter.com" {
		handleTwitter(s, m, u)
		return
	}

	if u.Host == "x.com" || u.Host == "mobile.x.com" || u.Host == "www.x.com" {
		handleTwitter(s, m, u)
		return
	}

	if u.Host == "threads.net" || u.Host == "www.threads.net" {
		handleThreads(s, m, u)
		return
	}

	if u.Host == "arazu.io" || u.Host == "www.arazu.io" {
		handleArazu(s, m, u)
		return
	}

	if u.Host == "www.instagram.com" || u.Host == "instagram.com" {
		handleInstagram(s, m, u)
		return
	}

	if u.Host == "www.tiktok.com" || u.Host == "tiktok.com" || u.Host == "vm.tiktok.com" {
		handleTiktok(s, m, u)
	}

	if u.Host == "reddit.com" || u.Host == "v.redd.it" || u.Host == "www.reddit.com" || u.Host == "old.reddit.com" {
		handleReddit(s, m, u)
	}

	if u.Host == "clips.twitch.tv" || u.Host == "www.twitch.tv" || u.Host == "twitch.tv" {
		handleTwitch(s, m, u)
	}

	if u.Host == "www.vimeo.com" || u.Host == "vimeo.com" {
		handleVimeo(s, m, u)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Author.Bot {
		return
	}

	regex := regexp.MustCompile(`(?m)<?(https?://[^\s<>]+)>?\b`)
	result := regex.FindAllStringSubmatch(m.Content, -1)
	for _, element := range result {
		go handleURL(s, m.Message, element[1])
	}
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionMessageComponent {
		if i.MessageComponentData().CustomID == "delete_message" {
			msg, err := s.ChannelMessage(i.ChannelID, i.Message.MessageReference.MessageID)
			if err != nil {
				if err.(*discordgo.RESTError).Response.StatusCode == 404 {
					err = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
					if err != nil {
						log.Println(err)
					}
				}
				return
			}
			if msg.Author.ID != i.Member.User.ID {
				return
			}
			err = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
			if err != nil {
				log.Println(err)
			}
		}
		if i.MessageComponentData().CustomID == "retry" {
			msg, err := s.ChannelMessage(i.ChannelID, i.Message.MessageReference.MessageID)
			if err != nil {
				if err.(*discordgo.RESTError).Response.StatusCode == 404 {
					return
				}
			}
			if msg.Author.ID != i.Member.User.ID {
				return
			}
			regex := regexp.MustCompile(`(?m)<?(https?://[^\s<>]+)>?\b`)
			result := regex.FindAllStringSubmatch(msg.Content, -1)
			for _, element := range result {
				go handleURL(s, msg, element[1])
			}
			s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
		}
		if i.MessageComponentData().CustomID == "unsuppress" {
			msg, err := s.ChannelMessage(i.ChannelID, i.Message.MessageReference.MessageID)
			if err != nil {
				if err.(*discordgo.RESTError).Response.StatusCode == 404 {
					return
				}
			}
			if msg.Author.ID != i.Member.User.ID {
				return
			}
			setEmbedSuppression(s, msg, false)

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
			})
		}

	}
}
