package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	tele "gopkg.in/telebot.v3"
)

type BotFile struct {
	dir  string
	name string
}

func checkExtensions(file string) bool {
	for _, ext := range Cfg.ExtensionsToLookFor {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

func findSubject(name string) (string, bool) {
	for _, subject := range Cfg.Subjects {
		if subject.Name[0] == name {
			return subject.Name[1], true
		}
	}
	return "", false
}

const (
	homeButtonText = "Назад"
	baseName       = "start.pdf"
)

func handleStartMessage(c tele.Context) error {
	algebra := tele.ReplyButton{Text: Cfg.Subjects[0].Name[0]}
	calculus := tele.ReplyButton{Text: Cfg.Subjects[1].Name[0]}
	discrete := tele.ReplyButton{Text: Cfg.Subjects[2].Name[0]}
	algorithms := tele.ReplyButton{Text: Cfg.Subjects[3].Name[0]}

	replyButtons := [][]tele.ReplyButton{
		{algebra, calculus},
		{discrete, algorithms},
	}

	replyKeyboard := tele.ReplyMarkup{
		ReplyKeyboard: replyButtons,
	}

	err := c.Send("Выбери предмет", &replyKeyboard)
	if err != nil {
		log.Println("Ошибка отправки клавиатуры: ", err)
		return err
	}

	return nil
}

func getContents(ctx context.Context, client *github.Client, bf *BotFile, doesRequestDir bool) (*github.RepositoryContent, []*github.RepositoryContent, error) {
	fmt.Println("Запрашивание контента: " + bf.dir + ", " + bf.name)
	path := filepath.Join(bf.dir, bf.name)
	if doesRequestDir {
		path = filepath.Dir(path)
	}
	log.Println("Запрашивается папка? :", doesRequestDir)
	log.Println("Запрашивается контент по пути: " + path)
	fileContent, directoryContent, _, err := client.Repositories.GetContents(ctx, Cfg.GithubInfo.Owner, Cfg.GithubInfo.Repo, path, &github.RepositoryContentGetOptions{Ref: Cfg.GithubInfo.Branch})
	if err != nil {
		log.Println("Ошибка доступа к файлу: " + path)
		log.Println(err)
		return nil, nil, err
	}
	return fileContent, directoryContent, nil
}

func handleSubjectMessageHelper(ctx context.Context, c tele.Context, bot *tele.Bot, bf *BotFile, client *github.Client) error {
	var ok bool
	bf.dir, ok = findSubject(c.Text())

	if !ok {
		log.Printf("Запросили предмет, которого не существует: " + bf.dir)
		_, err := bot.Send(c.Sender(), "Такого предмета не существует")
		if err != nil {
			log.Println("Ошибка отправки сообщения: " + err.Error())
			return err
		}
		return nil
	}

	_, directoryContent, err := getContents(ctx, client, bf, true)
	if err != nil {
		log.Println("Ошибка получения контента из репозитория: " + err.Error())
		return err
	} else if directoryContent == nil {
		log.Println("Такой директории не сущетсвует: " + bf.dir)
		return nil
	}

	var replyButtons [][]tele.ReplyButton
	var replyButtonsRow []tele.ReplyButton
	for i, file := range directoryContent {
		if !checkExtensions(file.GetName()) {
			continue
		}
		replyButtonsRow = append(replyButtonsRow, tele.ReplyButton{Text: file.GetName()})
		if i%3 == 0 {
			replyButtons = append(replyButtons, replyButtonsRow)
			replyButtonsRow = []tele.ReplyButton{}
		}
	}

	if len(replyButtons) > 0 {
		replyButtons[0] = append(replyButtons[0], tele.ReplyButton{Text: homeButtonText})
	}

	replyKeyboard := tele.ReplyMarkup{
		ReplyKeyboard: replyButtons,
	}

	err = c.Send("Выбери файл", &replyKeyboard)
	if err != nil {
		log.Println("Ошибка отправки клавиатуры: ", err)
		return err
	}

	return nil
}

func handleGetFileMessage(ctx context.Context, c tele.Context, bot *tele.Bot, bf *BotFile, client *github.Client) error {
	bf.name = c.Text()
	fileContent, _, err := getContents(ctx, client, bf, false)
	if err != nil {
		log.Println("Ошибка получения контента из репозитория: " + err.Error())
		return err
	} else if fileContent == nil {
		log.Println("Такого файла не сущетсвует: " + bf.dir)
		return nil
	}

	data, err := fileContent.GetContent()
	if err != nil {
		log.Println("Ошибка чтения контента из файла: " + err.Error())
		return err
	}

	tempFile, err := os.CreateTemp("", "lecture_github.pdf")
	if err != nil {
		log.Println("Ошибка при создании временного файла: " + err.Error())
		return err
	}

	defer func(tempFile *os.File) {
		err := tempFile.Close()
		if err != nil {
			log.Println("Ошибка при закрытии временного файла: " + err.Error())
		}
	}(tempFile)

	_, err = tempFile.Write([]byte(data))
	if err != nil {
		log.Println("Ошибка записи во временный файл: " + err.Error())
		return err
	}

	fileName := filepath.Join(bf.dir, bf.name)

	pdf := &tele.Document{
		File:     tele.FromDisk(tempFile.Name()),
		FileName: fileName,
	}

	_, err = bot.Send(c.Sender(), pdf)
	if err != nil {
		log.Println("Ошибка отправки сообщения: " + err.Error())
		return err
	}

	err = os.Remove(tempFile.Name())
	if err != nil {
		log.Println("Ошибка удаления временного файла: " + err.Error())
		return err
	}

	return nil
}

func handleMessage(ctx context.Context, bot *tele.Bot, bf *BotFile, client *github.Client) func(c tele.Context) error {
	return func(c tele.Context) error {
		message := c.Text()
		var err error

		if _, ok := findSubject(message); ok {
			err = handleSubjectMessageHelper(ctx, c, bot, bf, client)
		} else if checkExtensions(message) {
			err = handleGetFileMessage(ctx, c, bot, bf, client)
		}
		return err
	}
}

func main() {
	ReadConfig()

	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: Cfg.GithubInfo.Token},
	)

	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	preferences := tele.Settings{
		Token:  Cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(preferences)
	if err != nil {
		log.Fatal("Ошибка создания бота: ", err.Error())
		return
	}

	bf := new(BotFile)
	bf.name = baseName

	bot.Handle("/start", handleStartMessage)
	bot.Handle("Назад", handleStartMessage)
	bot.Handle(tele.OnText, handleMessage(ctx, bot, bf, client))

	bot.Start()
}
