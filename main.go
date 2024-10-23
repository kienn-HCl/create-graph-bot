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
	gnuplotText := `set timefmt '%Y/%m/%d-%H:%M:%S'
set title '温度・湿度・土壌水分'
set xdata time
set format x '%H:%M'
set xlabel '時間'
set xtics 60*60
set xtics rotate by 90 right
set yrange [0:40]
set ylabel '温度[℃]'
set ytics nomirror
set y2label '湿度・土壌水分[%]'
set y2range [0:100]
set y2tics nomirror
set my2tics 10
set terminal pngcairo
`
	gnuplotText += fmt.Sprintf("set output '%s'\n", pngFile.Name())
	gnuplotText += fmt.Sprintf("plot '%s' using 1:2 axis x1y1 with line title '温度', '%s' using 1:3 axis x1y2 with line title '湿度', '%s' using 1:4 axis x1y2 with line title '土壌水分'", dataFile.Name(), dataFile.Name(), dataFile.Name())

	log.Println("	gnuplotText:\n" + gnuplotText)

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
		shapedData += fmt.Sprintln(m.Timestamp.Local().Format("2006/01/02-15:04:05"), extractedData[0], extractedData[1], extractedData[2])
	}
	log.Println("	shapedData:\n" + shapedData)

	if _, err := dataFile.Write([]byte(shapedData)); err != nil {
		return err
	}
	return nil
}

func getMessageData(s *discordgo.Session, i *discordgo.InteractionCreate) []*discordgo.Message {
	var dataNum = 100
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
			if m.Author.Username != "aiueo" {
				continue
			}
			buffer = append(buffer, m)
		}

		if len(buffer) >= dataNum {
			buffer = buffer[0:dataNum]
			break
		}
		beforeID = buffer[len(buffer)-1].ID
	}

	var messageContents string
	for _, b := range buffer {
		messageContents += fmt.Sprintln(b.Content)
	}
	log.Printf("	get message(num=%s):\n%s\n", len(buffer), messageContents)

	return buffer
}

func main() {
	discord, err := discordgo.New("Bot " + os.Getenv("TOKEN"))
	if err != nil {
		log.Fatalln("error discordgo new: ", err)
	}

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
