package main

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

type commandElement struct {
	*discordgo.ApplicationCommand
	Handler func(*discordgo.Session, *discordgo.InteractionCreate)
}

func NewCommandElement(command *discordgo.ApplicationCommand, handler func(*discordgo.Session, *discordgo.InteractionCreate)) *commandElement {
	return &commandElement{
		ApplicationCommand: command,
		Handler:            handler,
	}
}

type CommandSet map[string]*commandElement

func NewCommandSet() CommandSet {
	return make(map[string]*commandElement)
}

func (c *CommandSet) ResisterCommand(s *discordgo.Session, command *discordgo.ApplicationCommand, handler func(*discordgo.Session, *discordgo.InteractionCreate)) error {
	cmd, err := s.ApplicationCommandCreate(s.State.User.ID, "", command)
	if err != nil {
		return fmt.Errorf("error register command(%s): %w", command.Name, err)
	}
	log.Println("create cmd:", cmd.Name)
	(*c)[cmd.Name] = NewCommandElement(cmd, handler)
	return nil
}

func (c *CommandSet) DeleteCommands(s *discordgo.Session) error {
	log.Println("removing commands...")
	var errs []error
	for name, cmd := range *c {
		err := s.ApplicationCommandDelete(s.State.User.ID, "", cmd.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("error delete command(%s): %w", name, err))
		}
	}
	if errs != nil {
		err := fmt.Errorf("error delete commands")
		for _, e := range errs {
			err = fmt.Errorf("%w : %w", err, e)
		}
		return err
	}
	return nil
}

func (c *CommandSet) ReturnHandler() func(*discordgo.Session, *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if cmd, ok := (*c)[i.ApplicationCommandData().Name]; ok {
			cmd.Handler(s, i)
		}
	}
}
