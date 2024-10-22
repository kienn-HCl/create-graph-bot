package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "test",
			Description: "test command",
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"test": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "hey, it is test!",
				},
			})
		},
	}
)

func main() {
	discord, err := discordgo.New("Bot " + os.Getenv("TOKEN"))
	if err != nil {
		log.Fatalln("error discordgo new: ", err)
	}

	discord.AddHandler(onMessageCreate)
	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

	if err = discord.Open(); err != nil {
		log.Fatalln("error discord open: ", err)
	}
	defer func() {
		log.Println("closing discord...")
		if err = discord.Close(); err != nil {
			log.Fatalln("error discord close: ", err)
		}
	}()

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	log.Println("creating commands...")
	for i, v := range commands {
		println(v.Name)
		cmd, err := discord.ApplicationCommandCreate(discord.State.User.ID, "", v)
		println(cmd.Name)
		if err != nil {
			log.Println("error create command(", v.Name, "): ", err)
		}
		registeredCommands[i] = cmd
	}
	defer func() {
		log.Println("removing commands...")
		for _, v := range registeredCommands {
			err := discord.ApplicationCommandDelete(discord.State.User.ID, "", v.ID)
			if err != nil {
				log.Println("error delete command(", v.Name, "): ", err)
			}
		}
	}()

	log.Println("now working...")

	stopBot := make(chan os.Signal, 1)
	signal.Notify(stopBot, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-stopBot
	log.Println("quit")
	return
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	appID := os.Getenv("AppID")
	u := m.Author
	if u.ID == appID {
		return
	}
	log.Println(m.ChannelID, u.Username, u.ID, m.Components)

	message := u.Mention() + "test uooo!"
	_, err := s.ChannelMessageSend(m.ChannelID, message)
	if err != nil {
		log.Println("error message send:", err)
	}
	log.Println("message : ", message)
}
