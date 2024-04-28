package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	tele "gopkg.in/telebot.v3"
)

const githubToken = "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"
const telegramToken = "6315259403:AAGo-T87jXeUXO7Fhk1KX2B6z_0cp9a56iU"
const owner = "DaniilZ77"
const repo = "notes_spbu"
const branch = "notes_without_changes"
const fileNameSuffix = "_lectures.pdf"

var subjectsNames = []string{
	"Алгебра",
	"Матан",
	"Дискретка",
	"Алгосы",
}
var subjectsMap = map[string]string{
	subjectsNames[0]: "algebra",
	subjectsNames[1]: "matan",
	subjectsNames[2]: "discrete",
	subjectsNames[3]: "algorithms",
}

func main() {
	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	pref := tele.Settings{
		Token:  telegramToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	bot.Handle("/start", func(c tele.Context) error {
		algebra := tele.ReplyButton{Text: subjectsNames[0]}
		matan := tele.ReplyButton{Text: subjectsNames[1]}
		discrete := tele.ReplyButton{Text: subjectsNames[2]}
		algorithms := tele.ReplyButton{Text: subjectsNames[3]}

		replyButtons := [][]tele.ReplyButton{
			{algebra, matan},
			{discrete, algorithms},
		}
		replyKeyboard := tele.ReplyMarkup{
			ReplyKeyboard:   replyButtons,
			OneTimeKeyboard: false,
		}
		err := c.Send("Выбери предмет", &replyKeyboard)
		if err != nil {
			log.Fatal("Проблема с отправкой клавиатуры: ", err)
		}
		return nil
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		path, isIn := subjectsMap[c.Text()]
		if !isIn {
			log.Println("Такого предмета не существует: " + c.Text())

			_, err = bot.Send(c.Sender(), "Выбери предмет еще раз")

			return nil
		}

		path += "/" + path + fileNameSuffix

		fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
		if err != nil {
			log.Println("Ошибка получения контента с github репозитория: ", err)
			_, err = bot.Send(c.Sender(), "Не нашел такого предмета")
			if err != nil {
				log.Fatal("Ошибка отправки сообщения ботом: ", err)
			}
			return nil
		}

		data, err := fileContent.GetContent()
		if err != nil {
			log.Println("Ошибка чтения контента: ", err)
			_, err = bot.Send(c.Sender(), "Не могу прочитать файл")
			if err != nil {
				log.Fatal("Ошибка отправки сообщения ботом: ", err)
			}
			return nil
		}

		tempFile, err := os.CreateTemp("", "lecture_github.pdf")
		if err != nil {
			log.Println("Ошибка при создании временного файла: ", err)
			_, err = bot.Send(c.Sender(), "Ошибка при создании временного файла")
			if err != nil {
				log.Fatal("Ошибка отправки сообщения ботом: ", err)
			}
		}
		defer tempFile.Close()

		_, err = tempFile.Write([]byte(data))
		if err != nil {
			log.Println("Ошибка при записи во временный файл: ", err)
			_, err = bot.Send(c.Sender(), "Ошибка при записи во временный файл")
			if err != nil {
				log.Fatal("Ошибка отправки сообщения ботом: ", err)
			}
		}

		pdf := &tele.Document{
			File:     tele.FromDisk(tempFile.Name()),
			FileName: path,
		}

		_, err = bot.Send(c.Sender(), pdf)
		if err != nil {
			log.Fatal("Ошибка отправки сообщения ботом: ", err)
		}
		os.Remove(tempFile.Name())
		return nil
	})
	bot.Start()
}
