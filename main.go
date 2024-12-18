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
	GuildID  = flag.String("guild", "", "Test guild ID. If not passed - bot registers commands globally")
	BotToken = flag.String("token", "", "Bot access token")
	DelCmd   = flag.Bool("delcmd", false, "delete registered commands when program finish.")
)

func init() { flag.Parse() }

var session *discordgo.Session

func init() {
	var err error
	session, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalln("error discordgo new session: ", err)
	}
}

func main() {
	log.Println("setup database...")
	db, err := setupDB("database.sqlite")
	if err != nil {
		log.Fatalln("error setup DB:", err)
	}
	defer db.Close()

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	if err := session.Open(); err != nil {
		log.Fatalln("error discord open: ", err)
	}
	defer func() {
		log.Println("closing discord...")
		if err := session.Close(); err != nil {
			log.Fatalln("error discord close: ", err)
		}
	}()

	commands := NewCommandSet()
	optionMin := 1.0
	commands.ResisterCommand(
		session,
		&discordgo.ApplicationCommand{
			Name: "graph",
			Options: []*discordgo.ApplicationCommandOption{

				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "hour",
					Description: "x's time range",
					MinValue:    &optionMin,
					Required:    false,
				},
			},
			Description: "create graph",
		},
		GraphHandler)
	if *DelCmd {
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
}
