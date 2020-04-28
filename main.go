package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/pelletier/go-toml"

	"git.sr.ht/~mendelmaleh/tgbotapi"
	"git.sr.ht/~mendelmaleh/xkcd"
)

type Config struct {
	Bot struct {
		Token string
	}

	Bleve struct {
		Index, Data string
	}
}

type Bot struct {
	*tgbotapi.BotAPI

	Config Config
	Bleve  bleve.Index
	XKCD   *xkcd.XKCD

	client *http.Client
}

func main() {
	bot := &Bot{}

	// get config
	doc, err := ioutil.ReadFile("config.toml")
	if err != nil {
		log.Fatal(err)
	}

	// parse config
	bot.Config = Config{}
	err = toml.Unmarshal(doc, &bot.Config)
	if err != nil {
		log.Fatal(err)
	}

	// set http client
	bot.client = &http.Client{
		Timeout: 90 * time.Second,
	}

	// get botapi
	bot.BotAPI, err = tgbotapi.NewBotAPIWithClient(bot.Config.Bot.Token, bot.client)
	if err != nil {
		log.Fatal(err)
	}

	// bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// get xkcd
	bot.XKCD = &xkcd.XKCD{
		BaseURL: xkcd.DefaultBaseURL,
		Client:  bot.client,
	}

	// get bleve index
	bot.Bleve, err = bleve.Open(bot.Config.Bleve.Index)
	if err == bleve.ErrorIndexPathDoesNotExist {
		log.Println("Creating a new index...")

		// create index
		bot.Bleve, err = bot.NewBleve()
		if err != nil {
			log.Fatal(err)
		}

		// index data
		go func(bot *Bot) {
			if err := bot.IndexData(); err != nil {
				log.Fatal(err)
			}
			log.Println("Done indexing data")
		}(bot)

	} else if err != nil {
		log.Fatal(err)
	}

	// parse updates
	updates := bot.GetUpdatesChan(tgbotapi.UpdateConfig{Timeout: 60})
	for u := range updates {
		if u.InlineQuery != nil {
			q := u.InlineQuery

			if q.Query == "" {
				continue
			}

			results, err := bot.FTS(q.Query, 5)
			if err != nil {
				continue
			}

			if len(results) < 1 {
				log.Println("Less than one result")
				continue
			}

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
