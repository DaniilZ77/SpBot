package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	tele "gopkg.in/telebot.v3"
)

const (
	githubToken   = "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"
	telegramToken = "6315259403:AAGo-T87jXeUXO7Fhk1KX2B6z_0cp9a56iU"
)
const owner = "DaniilZ77"
const repo = "notes_spbu"
const branch = "notes_without_changes"
const fileNameSuffix = "_lectures.pdf"

const (
	calculus   = "Матан"
	algebra    = "Алгебра"
	discrete   = "Дискретка"
	algorithms = "Алгосы"
)

var extensions = []string{
	".pdf",
}

var subjectsMap = map[string]string{
	algebra:    "algebra",
	calculus:   "matan",
	discrete:   "discrete",
	algorithms: "algorithms",
}
var lastCommand string

const homeButtonText = "/start"

func checkExtensions(file string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

func handleStartMessage(c tele.Context) error {
	algebra := tele.ReplyButton{Text: algebra}
	calculus := tele.ReplyButton{Text: calculus}
	discrete := tele.ReplyButton{Text: discrete}
	algorithms := tele.ReplyButton{Text: algorithms}

	replyButtons := [][]tele.ReplyButton{
		{algebra, calculus},
		{discrete, algorithms},
	}

	replyKeyboard := tele.ReplyMarkup{
		ReplyKeyboard: replyButtons,
	}

	err := c.Send("Выбери предмет", &replyKeyboard)
	if err != nil {
		log.Fatal("Проблема с отправкой клавиатуры: ", err)
	}

	return nil
}

func getContents(ctx context.Context, client *github.Client, dir string, file string) (*github.RepositoryContent, []*github.RepositoryContent, error) {
	path := filepath.Join(dir, file)
	fileContent, directoryContent, _, err := client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		log.Println("Не удалось найти файл: " + path)
		return nil, nil, err
	}
	return fileContent, directoryContent, nil
}

func handleSubjectMessageHelper(c tele.Context, bot *tele.Bot, client *github.Client, ctx context.Context) error {
	path, isIn := subjectsMap[c.Text()]

	lastCommand = path

	if !isIn {
		log.Println("Такого предмета не существует")
		_, err := bot.Send(c.Sender(), "Такого предмета не существует")
		if err != nil {
			log.Fatal("К сожалению бот не смог отправить сообщение")
		}
		return err
	}

	_, directoryContent, err := getContents(ctx, client, path, "")
	if err != nil || directoryContent == nil {
		log.Println("Что то пошло не так")
		return err
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
		return err
	}

	return nil
}

func handleGetFileMessage(c tele.Context, bot *tele.Bot, client *github.Client, ctx context.Context) error {
	text := c.Text()
	fileContent, _, err := getContents(ctx, client, lastCommand, text)
	if err != nil || fileContent == nil {
		log.Println("Не удалось найти файл: ", err)
		return err
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
		return nil
	}
	defer func(tempFile *os.File) {
		err := tempFile.Close()
		if err != nil {
			log.Fatal("Программа не смогла закрыть временный файл")
		}
	}(tempFile)

	_, err = tempFile.Write([]byte(data))
	if err != nil {
		log.Println("Ошибка при записи во временный файл: ", err)
		_, err = bot.Send(c.Sender(), "Ошибка при записи во временный файл")
		if err != nil {
			log.Fatal("Ошибка отправки сообщения ботом: ", err)
		}
		return nil
	}

	fileName := filepath.Join(lastCommand, text)

	pdf := &tele.Document{
		File:     tele.FromDisk(tempFile.Name()),
		FileName: fileName,
	}

	_, err = bot.Send(c.Sender(), pdf)
	if err != nil {
		log.Fatal("Ошибка отправки сообщения ботом: ", err)
	}
	err = os.Remove(tempFile.Name())
	if err != nil {
		log.Println("Ошибка при удалении временного файла: ", err)
		_, err = bot.Send(c.Sender(), "Ошибка при удалении временного файла")
		if err != nil {
			log.Fatal("Ошибка отправки сообщения ботом: ", err)
		}
		return nil
	}

	return nil
}

func handleMessage(bot *tele.Bot, client *github.Client, ctx context.Context) func(c tele.Context) error {
	return func(c tele.Context) error {
		text := c.Text()
		var err error

		if _, isIn := subjectsMap[text]; isIn {
			err = handleSubjectMessageHelper(c, bot, client, ctx)
		} else if checkExtensions(text) {
			err = handleGetFileMessage(c, bot, client, ctx)
		}
		return err
	}
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

	bot.Handle("/start", handleStartMessage)
	//bot.Handle(tele.OnText, handleRequestLectureMessage(bot, client, ctx, err))
	bot.Handle(tele.OnText, handleMessage(bot, client, ctx))

	bot.Start()
}
