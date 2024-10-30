package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	delCmd = flag.Bool("delcmd", false, "delete registered commands when program finish.")
)

func main() {
	flag.Parse()
	session, err := discordgo.New("Bot " + os.Getenv("TOKEN"))
	if err != nil {
		log.Fatalln("error discordgo new: ", err)
	}
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	if err = session.Open(); err != nil {
		log.Fatalln("error discord open: ", err)
	}
	defer func() {
		log.Println("closing discord...")
		if err = session.Close(); err != nil {
			log.Fatalln("error discord close: ", err)
		}
	}()

	commands := NewCommandSet()
	commands.ResisterCommand(
		session,
		&discordgo.ApplicationCommand{
			Name:        "graph",
			Description: "create graph",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "Hour",
					Description: "The time span for plotting the graph",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "1day",
							Value: 24,
						},
						{
							Name:  "2day",
							Value: 48,
						},
					},
				},
			},
		},
		GraphHandler)
	if *delCmd {
		log.Println("Delete commands before discord session closed.")
		defer func() {
			if err := commands.DeleteCommands(session); err != nil {
				log.Fatalln("error delete commands: ", err)
			}
		}()
	}
	session.AddHandler(commands.ReturnHandler())

	log.Println("now working...")

	stopBot := make(chan os.Signal, 1)
	signal.Notify(stopBot, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-stopBot
	log.Println("quitting...")
	return
}
