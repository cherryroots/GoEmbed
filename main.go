package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strings"
	"sync"
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

var reduceButton = discordgo.Button{
	Label:    "Reduce",
	Style:    discordgo.PrimaryButton,
	CustomID: "reduce",
}

// make a discord action row with the delete message button
var deleteMessageActionRow = discordgo.ActionsRow{
	Components: []discordgo.MessageComponent{deleteMessageButton},
}

var instagramActionRow = discordgo.ActionsRow{
	Components: []discordgo.MessageComponent{reduceButton, deleteMessageButton},
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

	if u.Host == "x.com" || u.Host == "mobile.x.com" || u.Host == "www.x.com" {
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
		if i.MessageComponentData().CustomID == "reduce" {
			// get the message that was replied to
			// if the message was sent by the same user that clicked the button, add the attachment select menu
			// the menu will be a list of all the attachments
			// the user will be able to select multiple ones
			msg, err := s.ChannelMessage(i.ChannelID, i.Message.MessageReference.MessageID)
			if err != nil {
				// if the message was deleted, do nothing
				if err.(*discordgo.RESTError).Response.StatusCode == 404 {
					return
				}
			}

			minValues := 1

			attachmentSelectMenu := discordgo.SelectMenu{
				Placeholder: "Select an attachment",
				MinValues:   &minValues,
				MaxValues:   1,
				CustomID:    "attachment_select_menu",
				Options:     []discordgo.SelectMenuOption{},
			}

			for count, attachment := range i.Message.Attachments {
				attachmentSelectMenu.Options = append(attachmentSelectMenu.Options, discordgo.SelectMenuOption{
					Label: fmt.Sprint(count + 1),
					Value: attachment.URL,
				})
			}

			// make two rows, one for the attachment select menu and one for the delete message button
			var reduceActionRow = discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{attachmentSelectMenu},
			}

			// if the message was sent by the same user that clicked the button
			if msg.Author.ID == i.Member.User.ID {
				// add a new action row with the attachment select menu to the interaction message
				err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Components: []discordgo.MessageComponent{reduceActionRow, deleteMessageActionRow},
					},
				})
				if err != nil {
					fmt.Println(err)
				}
			}
		}
		if i.MessageComponentData().CustomID == "attachment_select_menu" {
			// get the message that was replied to
			// if the message was sent by the same user that clicked the button, edit the message with the new attachments only
			msg, err := s.ChannelMessage(i.ChannelID, i.Message.MessageReference.MessageID)
			if err != nil {
				// if the message was deleted, do nothing
				if err.(*discordgo.RESTError).Response.StatusCode == 404 {
					return
				}
			}

			// download all the attachments

			files := make([]*discordgo.File, 0)
			emptyAttachment := make([]*discordgo.MessageAttachment, 0)

			var wg sync.WaitGroup
			wg.Add(len(i.MessageComponentData().Values))
			for _, url := range i.MessageComponentData().Values {
				go func(url string) {
					defer wg.Done()
					// strip the file extension from the url
					extension := strings.TrimPrefix(path.Ext(url), url)
					// Create a temp file starting with twitter and ending with .mp4
					f, err := os.CreateTemp("", "discordattachment*"+extension)
					if err != nil {
						return
					}

					err = DownloadFile(f.Name(), url)
					if err != nil {
						return
					}

					file, err := os.Open(f.Name())
					if err != nil {
						return

					}

					files = append(files, &discordgo.File{
						Name:   "image" + extension,
						Reader: file,
					})

					defer os.Remove(f.Name())
				}(url)
			}

			wg.Wait()

			if msg.Author.ID == i.Member.User.ID {
				// remove the attachments from the message before updating it
				_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
					ID:          i.Message.ID,
					Channel:     i.Message.ChannelID,
					Attachments: &emptyAttachment,
					Files:       files,
				})
				if err != nil {
					return
				}
				err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Components: []discordgo.MessageComponent{instagramActionRow},
					},
				})
				if err != nil {
					return
				}
			}
		}
	}
}
