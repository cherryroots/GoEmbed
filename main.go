package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	discordgo "github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

var (
	Token      string
	TwUsername string
	TwPassword string
)

// make a discord button called "Delete Message" that deletes the message
var deleteMessageButton = discordgo.Button{
	Label:    "Delete Message",
	Style:    discordgo.DangerButton,
	CustomID: "delete_message",
}

// make a discord action row with the delete message button
var deleteMessageActionRow = discordgo.ActionsRow{
	Components: []discordgo.MessageComponent{deleteMessageButton},
}

func init() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func main() {
	Token := os.Getenv("TOKEN")
	TwUsername := os.Getenv("TWUSERNAME")
	TwPassword := os.Getenv("TWPASSWORD")

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	twScraper := twitterscraper.New()
	err = twScraper.Login(TwUsername, TwPassword)
	if err != nil {
		panic(err)
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m, twScraper)
	})

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		interactionCreate(s, i)
	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages
	dg.ShouldReconnectOnError = true

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
	twScraper.Logout()

	defer fmt.Print("Bot is now offline.")

}

func handleUrl(s *discordgo.Session, m *discordgo.MessageCreate, scraper *twitterscraper.Scraper, link string) {
	u, err := url.Parse(link)
	if err != nil {
		return
	}
	if u.Host == "twitter.com" || u.Host == "mobile.twitter.com" || u.Host == "www.twitter.com" {
		handleTwitter(s, m, scraper, u)
	}

	if u.Host == "www.instagram.com" || u.Host == "instagram.com" {
		handleInstagram(s, m, u)
	}

	if u.Host == "reddit.com" || u.Host == "v.redd.it" || u.Host == "www.reddit.com" || u.Host == "old.reddit.com" {
		handleReddit(s, m, u)
	}

	if u.Host == "clips.twitch.tv" || u.Host == "www.twitch.tv" || u.Host == "twitch.tv" {
		handleTwitch(s, m, u)
	}

	//handle tiktok
	if u.Host == "www.tiktok.com" || u.Host == "tiktok.com" || u.Host == "vm.tiktok.com" {
		handleTiktok(s, m, u)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate, scraper *twitterscraper.Scraper) {

	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Author.Bot {
		return
	}

	// get all the url in the message with regex
	// handle them individually with a go routine
	regex := regexp.MustCompile(`(?m)[<]?(https?:\/\/[^\s<>]+)[>]?\b`)
	result := regex.FindAllStringSubmatch(m.Content, -1)
	for _, element := range result {
		go handleUrl(s, m, scraper, element[1])
	}
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionMessageComponent {
		if i.MessageComponentData().CustomID == "delete_message" {
			// get the message that was replied to
			// if the message was sent by the same user that clicked the button, delete it
			msg, err := s.ChannelMessage(i.ChannelID, i.Message.MessageReference.MessageID)
			if err != nil {
				// if the message was deleted, allow the post to be deleted
				if err.(*discordgo.RESTError).Response.StatusCode == 404 {
					err = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
					if err != nil {
						fmt.Println(err)
					}
				}
				return
			}
			if msg.Author.ID == i.Member.User.ID {
				err = s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}
}
