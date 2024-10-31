package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
)

type DataElemment struct {
	Time  time.Time
	Items map[string]string
}

type DataSet []*DataElemment

func NewDataSet() DataSet {
	return DataSet{}
}

func (ds *DataSet) AddDataElemment(time time.Time, items *map[string]string) {
	*ds = append(*ds, &DataElemment{
		time,
		*items,
	})
}

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
	dataSet := filterAndShapeMessages(messages, time.Hour*24)

	log.Println("creating graph...")
	pngs := make([]io.Reader, 0, len(dataSet[0].Items))
	for title := range dataSet[0].Items {
		png, err := createPngGraph(dataSet, title)
		if err != nil {
			errorlogAndRespondToDiscord(s, i, "error create graph.", err)
			return
		}
		pngs = append(pngs, png)
	}

	log.Println("respond messge...")
	err = respondPngsToDiscord(s, i, "Create graph!", pngs...)
	if err != nil {
		log.Println("error respond message :", err)
		return
	}
}

func respondPngsToDiscord(s *discordgo.Session, i *discordgo.InteractionCreate, content string, pngs ...io.Reader) error {
	var respondFiles []*discordgo.File
	for i, png := range pngs {
		respondFiles = append(respondFiles, &discordgo.File{
			ContentType: "image/png",
			Name:        fmt.Sprintf("%s-%d.png", time.Now().String(), i),
			Reader:      png,
		})
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Files:   respondFiles,
		},
	})
	if err != nil {
        _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
			Files:   respondFiles,
		})
        log.Println(err)
	}
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

func createPngGraph(dataSet DataSet, title string) (io.Reader, error) {
	gnuplotText := fmt.Sprintln("set timefmt '%Y/%m/%d-%H:%M:%S';")
	gnuplotText += fmt.Sprintf("set title '%s';", title)
	gnuplotText += fmt.Sprintln("set xdata time;")
	gnuplotText += fmt.Sprintln("set format x '%H:%M';")
	gnuplotText += fmt.Sprintln("set xlabel '時間';")
	gnuplotText += fmt.Sprintln("set xtics 60*60;")
	gnuplotText += fmt.Sprintln("set xtics rotate by 90 right;")
	gnuplotText += fmt.Sprintln("set ytics nomirror;")
	gnuplotText += fmt.Sprintln("set terminal pngcairo;")
	gnuplotText += fmt.Sprintf("plot '< cat -' using 1:2 axis x1y1 with line title '%s'", title)

	log.Println("	gnuplotText:\n" + gnuplotText)

	var dataText string
	for _, dataElem := range dataSet {
		dataText += fmt.Sprintln(dataElem.Time.Local().Format("2006/01/02-15:04:05"), dataElem.Items[title])
	}

	cmd := exec.Command("gnuplot", "-e", gnuplotText)
	cmd.Stdin = strings.NewReader(dataText)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error exec gnuplot: %w \n command stdout and stderr is\n%s", err, string(output))
	}

	return bytes.NewReader(output), nil
}

func filterAndShapeMessages(messages []*discordgo.Message, period time.Duration) (dataSet DataSet) {
	limitTime := messages[0].Timestamp.Add(-period)
	for _, m := range messages {
		if m.Timestamp.Before(limitTime) {
			break
		}

		items := make(map[string]string)
		itemStrs := strings.Split(m.Content, ",")
		for _, itemStr := range itemStrs {
			pair := strings.Split(itemStr, ":")
			key := strings.TrimSpace(pair[0])
			value := strings.TrimFunc(pair[1], isNotNum)
			items[key] = value
		}
		dataSet.AddDataElemment(m.Timestamp, &items)
	}
	log.Printf("	shapedData:\n%#v", dataSet)
	return
}

func isNotNum(c rune) bool {
	return c != '.' && !unicode.IsNumber(c)
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
