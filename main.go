package main

import (
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"strconv"

	"github.com/pelletier/go-toml"

	"git.sr.ht/~mendelmaleh/tgbotapi"
	"git.sr.ht/~mendelmaleh/xkcd"
)

type Config struct {
	Bot struct {
		Token string
	}
}

func main() {
	doc, err := ioutil.ReadFile("config.toml")
	if err != nil {
		log.Panic(err)
	}

	config := Config{}
	err = toml.Unmarshal(doc, &config)
	if err != nil {
		log.Panic(err)
	}

	XKCD := xkcd.New()

	bot, err := tgbotapi.NewBotAPI(config.Bot.Token)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)
	updates := bot.GetUpdatesChan(tgbotapi.UpdateConfig{Timeout: 60})

	for u := range updates {
		if u.InlineQuery != nil {
			q := u.InlineQuery

			if q.Query == "" {
				continue
			}

			c, err := XKCD.Get(q.Query)
			if err != nil {
				log.Print(err)
				continue
			}

			a := strconv.Itoa(c.Num)
			r := tgbotapi.InlineQueryResultPhoto{
				Type:      "photo", // must be
				ID:        a,
				URL:       c.Img,
				ThumbURL:  c.Img,
				ParseMode: "html",
				Caption: fmt.Sprintf("<a href=\"%s\">#%s:</a> <i>%s</i>",
					c.URL(XKCD), a, html.EscapeString(c.Alt)),
			}

			results := make([]interface{}, 1)
			results[0] = r

			api, err := bot.Send(tgbotapi.InlineConfig{
				InlineQueryID: q.ID,
				Results:       results,
			})

			if err != nil {
				log.Println(api, err)
			}
		}
	}
}
