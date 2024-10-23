package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"unicode"

	"github.com/bwmarrin/discordgo"
)

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "graph",
			Description: "create graph",
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"graph": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			log.Println("getting messages...")
			messageData := getMessageData(s, i)

			log.Println("making files...")
			var fileErrs []error
			dataFile, err := os.CreateTemp("", "*.txt")
			fileErrs = append(fileErrs, err)
			gnuplotFile, err := os.CreateTemp("", "*.plt")
			fileErrs = append(fileErrs, err)
			pngFile, err := os.CreateTemp("", "*.png")
			fileErrs = append(fileErrs, err)
			for _, err := range fileErrs {
				if err != nil {
					log.Println("error create tempfile:", err)
					return
				}
			}
			defer func(files ...*os.File) {
				for _, f := range files {
					if err := f.Close(); err != nil {
						log.Println("error close tempfile:", err)
					}
					os.Remove(f.Name())
				}
			}(dataFile, gnuplotFile, pngFile)

			log.Println("making & writing shapedData...")
			err = createDataFile(messageData, dataFile)
			if err != nil {
				log.Println("error create data file:", err)
			}
			log.Println("making graph...")
			err = createGraphPng(dataFile, gnuplotFile, pngFile)
			if err != nil {
				log.Println("error create graph:", err)
			}

			log.Println("sending messge...")
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Make graph",
					Files: []*discordgo.File{
						{
							ContentType: "image/png",
							Name:        pngFile.Name(),
							Reader:      pngFile,
						},
					},
				},
			})
		},
	}
)

func createGraphPng(dataFile, gnuplotFile, pngFile *os.File) error {
	gnuplotText := `set timefmt '%H:%M:%S'
set title 'temp, humidity, soil moisture'
set xdata time
set format x '%H:%M'
set xlabel 'time'
set xtics 60*15
set yrange [0:40]
set ylabel 'temperature'
set ytics nomirror
set y2label 'humidity and soil moisture'
set y2range [0:100]
set y2tics nomirror
set my2tics 10
set terminal png
`
	gnuplotText += fmt.Sprintf("set output '%s'\n", pngFile.Name())
	gnuplotText += fmt.Sprintf("plot '%s' using 1:2 axis x1y1 with line title 'temp', '%s' using 1:3 axis x1y2 with line title 'humid', '%s' using 1:4 axis x1y2 with line title 'soil'", dataFile.Name(), dataFile.Name(), dataFile.Name())

	log.Println("gnuplotText:\n" + gnuplotText)

	if _, err := gnuplotFile.Write([]byte(gnuplotText)); err != nil {
		return fmt.Errorf("error create gnuplot file:", err)
	}

	cmd := exec.Command("gnuplot", gnuplotFile.Name())
	if stdoutAndErr, err := cmd.CombinedOutput(); err != nil {
		log.Println(string(stdoutAndErr))
		return fmt.Errorf("error exec gnuplot:", err)
	}

	return nil
}

func createDataFile(messages []*discordgo.Message, dataFile *os.File) error {
	var shapedData string
	for _, m := range messages {
		if !strings.HasPrefix(m.Content, "温度") {
			continue
		}
		extractedData := strings.FieldsFunc(m.Content, func(c rune) bool {
			return c != '.' && !unicode.IsNumber(c)
		})
		shapedData += fmt.Sprintln(m.Timestamp.Local().Format("15:04:05"), extractedData[0], extractedData[1], extractedData[2])
	}
	log.Println("shapedData:\n" + shapedData)

	if _, err := dataFile.Write([]byte(shapedData)); err != nil {
		return err
	}
	return nil
}

func getMessageData(s *discordgo.Session, i *discordgo.InteractionCreate) []*discordgo.Message {
	var dataNum = 50
	var beforeID string
	var buffer []*discordgo.Message

	// このループは5回程度に止めないと時間がかかりすぎるのかbotが応答しなかった扱いになる
	for num := 0; num < 5; num++ {
		// メッセージを100件単位で取ってくる
		messages, err := s.ChannelMessages(i.ChannelID, 100, beforeID, "", "")
		if err != nil {
			log.Println("error get messages: ", err)
		}
		if len(messages) == 0 {
			break
		}

		// 取ってきたメッセージのうち条件を満たすものを集める
		for _, m := range messages {
			// println(m.Author.Username, " : ", m.Content)
			// fmt.Printf("%#v\n", m)
			if m.Author.Username != "aiueo" {
				continue
			}
			buffer = append(buffer, m)
		}

		if len(buffer) == dataNum {
			break
		}
		beforeID = buffer[len(buffer)-1].ID
	}

	var messageContents string
	for _, b := range buffer {
		messageContents += fmt.Sprintln(b.Content)
	}
	log.Println("get message:\n" + messageContents)

	return buffer
}

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
		cmd, err := discord.ApplicationCommandCreate(discord.State.User.ID, "", v)
		if err != nil {
			log.Println("error create command(", v.Name, "): ", err)
		}
		log.Println("	create cmd:", cmd.Name)
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
