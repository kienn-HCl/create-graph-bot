package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
)

func GraphHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	log.Println("getting messages...")
	messages, err := getNumOfTargetMessages(s, i, 100)
	if err != nil {
		errorlogAndRespondToDiscord(s, i, "error getting messages.", err)
		return
	}
	if 0 == len(messages) {
		errorlogAndRespondToDiscord(s, i, "got 0 messages.", err)
		return
	}

	log.Println("shape messages...")
	data, err := filterAndShapeMessages(messages)
	if err != nil {
		errorlogAndRespondToDiscord(s, i, "error shape messages.", err)
		return
	}
	if 0 == len(data) {
		errorlogAndRespondToDiscord(s, i, "filtered messages is 0.", err)
		return
	}

	log.Println("creating files...")
	files, err := createFiles("*.txt", "temps*.plt", "temps*.png", "battery*.plt", "battery*.png")
	if err != nil {
		errorlogAndRespondToDiscord(s, i, "error create required file.", err)
		return
	}
	defer func(files map[string]*os.File) {
		for name, file := range files {
			if err := file.Close(); err != nil {
				log.Println("error close file", "("+name+") :", err)
			}
			os.Remove(file.Name())
		}
	}(files)

	log.Println("writing data to file...")
	if _, err := files["*.txt"].Write([]byte(data)); err != nil {
		errorlogAndRespondToDiscord(s, i, "error write data to file.", err)
		return
	}

	log.Println("creating graph...")
	err = createTempsGraphPng(files["*.txt"], files["temps*.plt"], files["temps*.png"])
	if err != nil {
		errorlogAndRespondToDiscord(s, i, "error create graph.", err)
		return
	}

	log.Println("creating graph...")
	err = createBatteryGraphPng(files["*.txt"], files["battery*.plt"], files["battery*.png"])
	if err != nil {
		errorlogAndRespondToDiscord(s, i, "error create graph.", err)
		return
	}

	log.Println("respond messge...")
	err = respondPngFileToDiscord(s, i, "Create graph!", files["temps*.png"], files["battery*.png"])
	if err != nil {
		log.Println("error respond message :", err)
		return
	}
}

func createFiles(names ...string) (map[string]*os.File, error) {
	files := make(map[string]*os.File)
	for _, name := range names {
		file, err := os.CreateTemp("", name)
		if err != nil {
			return nil, fmt.Errorf("error at %s: %w", name, err)
		}
		files[name] = file
	}
	return files, nil
}

func respondPngFileToDiscord(s *discordgo.Session, i *discordgo.InteractionCreate, content string, files ...*os.File) error {
	var respondFiles []*discordgo.File
	for _, file := range files {
		respondFiles = append(respondFiles, &discordgo.File{
			ContentType: "image/png",
			Name:        file.Name(),
			Reader:      file,
		})
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Files:   respondFiles,
		},
	})
	return err
}

func errorlogAndRespondToDiscord(s *discordgo.Session, i *discordgo.InteractionCreate, text string, err error) {
	log.Println(text, ":", err)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Sorry, " + text,
		},
	})
}

func createBatteryGraphPng(data, gnuplot, png *os.File) error {
	gnuplotText := fmt.Sprintln("set timefmt '%Y/%m/%d-%H:%M:%S'")
	gnuplotText += fmt.Sprintln("set title 'バッテリー'")
	gnuplotText += fmt.Sprintln("set xdata time")
	gnuplotText += fmt.Sprintln("set format x '%H:%M'")
	gnuplotText += fmt.Sprintln("set xlabel '時間'")
	gnuplotText += fmt.Sprintln("set xtics 60*60")
	gnuplotText += fmt.Sprintln("set xtics rotate by 90 right")
	gnuplotText += fmt.Sprintln("set ylabel '電圧[V]'")
	gnuplotText += fmt.Sprintln("set yrange [0:2]")
	gnuplotText += fmt.Sprintln("set ytics nomirror")
	gnuplotText += fmt.Sprintln("set terminal pngcairo")
	gnuplotText += fmt.Sprintf("set output '%s'\n", png.Name())
	gnuplotText += fmt.Sprintf("plot '%s' using 1:5 axis x1y1 with line title 'バッテリー'", data.Name())

	log.Println("	gnuplotText:\n" + gnuplotText)

	if _, err := gnuplot.Write([]byte(gnuplotText)); err != nil {
		return fmt.Errorf("error write to gnuplot file: %w", err)
	}

	cmd := exec.Command("gnuplot", gnuplot.Name())
	if stdoutAndErr, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error exec gnuplot: %w \n command stdout and stderr is\n%s", err, string(stdoutAndErr))
	}

	return nil
}

func createTempsGraphPng(data, gnuplot, png *os.File) error {
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
	gnuplotText += fmt.Sprintf("set output '%s'\n", png.Name())
	gnuplotText += fmt.Sprintf("plot '%s' using 1:2 axis x1y1 with line title '温度', '%s' using 1:3 axis x1y2 with line title '湿度', '%s' using 1:4 axis x1y2 with line title '土壌水分'", data.Name(), data.Name(), data.Name())

	log.Println("	gnuplotText:\n" + gnuplotText)

	if _, err := gnuplot.Write([]byte(gnuplotText)); err != nil {
		return fmt.Errorf("error create gnuplot file: %w", err)
	}

	cmd := exec.Command("gnuplot", gnuplot.Name())
	if stdoutAndErr, err := cmd.CombinedOutput(); err != nil {
		log.Println(string(stdoutAndErr))
		return fmt.Errorf("error exec gnuplot: %w", err)
	}

	return nil
}

func filterAndShapeMessages(messages []*discordgo.Message) (data string, err error) {
	limitTime := messages[0].Timestamp.Add(-time.Hour * 24)
	for _, m := range messages {
		if m.Timestamp.Before(limitTime) {
			break
		}
		extracted := strings.FieldsFunc(m.Content, func(c rune) bool {
			return c != '.' && !unicode.IsNumber(c)
		})
		data += fmt.Sprintln(m.Timestamp.Local().Format("2006/01/02-15:04:05"), strings.Join(extracted, " "))
	}
	log.Println("	shapedData:\n" + data)
	return
}

func getNumOfTargetMessages(s *discordgo.Session, i *discordgo.InteractionCreate, num int) ([]*discordgo.Message, error) {
	messages := make([]*discordgo.Message, 0, 2*num)
	var beforeID string
	// 目的のメッセージを指定個数用意するループ
	// このループは5回程度に止めないと時間がかかりすぎるのかbotが応答しなかった扱いになる
	for j := 0; j < 5; j++ {
		// メッセージを100件単位で取ってくる
		buffer, err := s.ChannelMessages(i.ChannelID, 100, beforeID, "", "")
		if err != nil {
			return nil, fmt.Errorf("error get messages: %w", err)
		}
		if len(buffer) == 0 {
			// ここでループから抜ける場合、messagesがnum未満の可能性がある。
			break
		}

		messages = appendFilteredMessages(messages, buffer)

		if len(messages) >= num {
			messages = messages[0:num]
			break
		}
		beforeID = messages[len(messages)-1].ID
	}
	logMessages(messages)
	return messages, nil
}

func appendFilteredMessages(beAppended, beFiltered []*discordgo.Message) []*discordgo.Message {
	for _, b := range beFiltered {
		if !strings.HasPrefix(b.Content, "温度: ") {
			continue
		}
		beAppended = append(beAppended, b)
	}
	return beAppended
}

func logMessages(messages []*discordgo.Message) {
	var messageContents string
	for _, b := range messages {
		messageContents += fmt.Sprintln(b.Content)
	}
	log.Printf("	get message(num=%d):\n%s\n", len(messages), messageContents)
}
