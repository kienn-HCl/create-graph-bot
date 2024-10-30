package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func main() {
	session, err := discordgo.New("Bot " + os.Getenv("TOKEN"))
	if err != nil {
		log.Fatalln("error discordgo new: ", err)
	}

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
		},
		GraphHandler)
	defer func() {
		log.Println("removing commands...")
		if err := commands.DeleteCommands(session); err != nil {
			log.Fatalln("error delete commands: ", err)
		}
	}()
	session.AddHandler(commands.ReturnHandler)

	log.Println("now working...")

	stopBot := make(chan os.Signal, 1)
	signal.Notify(stopBot, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-stopBot
	log.Println("quitting...")
	return
}
