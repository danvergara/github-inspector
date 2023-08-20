// This is the function that is called by google functions,
// the structure of the code must be like specified in the
// docs https://cloud.google.com/functions/docs/writing#directory-structure

package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"text/template"

	"github.com/Arturomtz8/github-inspector/pkg/github"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

const (
	searchCommand          string = "/search"
	RepoURL                string = "https://api.github.com/search/repositories"
	telegramApiBaseUrl     string = "https://api.telegram.org/bot"
	telegramApiSendMessage string = "/sendMessage"
	telegramTokenEnv       string = "GITHUB_BOT_TOKEN"
)

var lenSearchCommand int = len(searchCommand)

// Chat struct stores the id of the chat in question.
type Chat struct {
	Id int `json:"id"`
}

// Message struct store Chat and text data.
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

// Update event.
type Update struct {
	UpdateId int     `json:"update_id"`
	Message  Message `json:"message"`
}

// Register an HTTP function with the Functions Framework
func init() {
	functions.HTTP("HandleTelegramWebhook", HandleTelegramWebhook)
}

// HandleTelegramWebhook is the web hook that has to have the handler signature.
// Listen for incoming web requests from Telegram events and
// responds back with the treding repositories on GitHub.
func HandleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	var update, err = parseTelegramRequest(r)
	if err != nil {
		log.Printf("error parsing update, %s", err.Error())
		return
	}

	sanitizedString, err := sanitize(update.Message.Text)
	if err != nil {
		sendTextToTelegramChat(update.Message.Chat.Id, sanitizedString)
		fmt.Fprint(w, "Invald input")
	}

	result, err := github.SearchGithubTrending(sanitizedString)
	if err != nil {
		fmt.Fprintf(w, "An error has ocurred, %s!", err)
	}

	const templ = `{{.TotalCount}} repositories:
	{{range .Items}}----------------------------------------
	Name:          {{.Full_name}}
	Url:           {{.Html_url}}
	Description:   {{.Description}}
	Created at:    {{.Created_at }}
	Update 	at:    {{.Updated_at}} 
	Pushed at:     {{.Pushed_at}}
	Size(KB):      {{.Size}}
	Language:      {{.Language}}
	Stargazers:    {{.Stargazers_count}}
	Archived:      {{.Archived}}
	Open Issues:   {{.Open_issues_count}}
	Topics:        {{.Topics}}
	{{end}}`

	var report = template.Must(template.New("trendinglist").Parse(templ))
	buf := &bytes.Buffer{}
	if err := report.Execute(buf, result); err != nil {
		panic(err)
	}

	s := buf.String()

	var telegramResponseBody, errTelegram = sendTextToTelegramChat(update.Message.Chat.Id, s)
	if errTelegram != nil {
		log.Printf("got error %s from telegram, response body is %s", errTelegram.Error(), telegramResponseBody)

	} else {
		log.Printf("successfully distributed to chat id %d", update.Message.Chat.Id)
	}

}

// parseTelegramRequest decodes and incoming request from the Telegram hook,
// and returns an Update pointer.
func parseTelegramRequest(r *http.Request) (*Update, error) {
	var update Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("could not decode incoming update %s", err.Error())
		return nil, err
	}
	return &update, nil
}

// returns the term that wants to be searched
func sanitize(s string) (string, error) {
	if len(s) >= lenSearchCommand {
		if s[:lenSearchCommand] == searchCommand {
			s = s[lenSearchCommand:]
		}

	} else {
		return "You must enter /search {languague}", fmt.Errorf("Invalid value")
	}
	return s, nil

}

// sendTextToTelegramChat sends the response from the GitHub back to the chat,
// given a chat it and the text from GitHub.
func sendTextToTelegramChat(chatId int, text string) (string, error) {
	log.Printf("Sending %s to chat_id: %d", text, chatId)

	var telegramApi string = "https://api.telegram.org/bot" + os.Getenv("GITHUB_BOT_TOKEN") + "/sendMessage"

	response, err := http.PostForm(
		telegramApi,
		url.Values{
			"chat_id": {strconv.Itoa(chatId)},
			"text":    {text},
		})
	if err != nil {
		log.Printf("error when posting text to the chat: %s", err.Error())
		return "", err
	}
	// defer response.Body.Close()
	var bodyBytes, errRead = ioutil.ReadAll(response.Body)
	if errRead != nil {
		log.Printf("error parsing telegram answer %s", errRead.Error())
		return "", err
	}

	bodyString := string(bodyBytes)
	log.Printf("body of telegram response: %s", bodyString)
	response.Body.Close()
	return bodyString, nil

}
