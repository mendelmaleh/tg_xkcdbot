package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
		Index string
	}
}

type Bot struct {
	*tgbotapi.BotAPI

	Config Config
	Bleve  bleve.Index
	XKCD   *xkcd.XKCD

	client *http.Client
}

var lastKey = []byte("last")

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

		// init internal last key
		if err := bot.Bleve.SetInternal(lastKey, []byte{}); err != nil {
			log.Fatal(err)
		}

	} else if err != nil {
		log.Fatal(err)
	}

	// index data
	go func(bot *Bot) {
		t := time.NewTicker(10 * time.Minute)
		for {
			log.Println("Starting indexing data")
			if err := bot.Update(); err != nil {
				log.Fatal(err)
			}
			log.Println("Done indexing data")

			// wait for next tick
			<-t.C
		}
	}(bot)

	// parse updates
	updates := bot.GetUpdatesChan(tgbotapi.UpdateConfig{Timeout: 60})
	for u := range updates {
		if u.InlineQuery != nil {
			q := u.InlineQuery
			query := q.Query

			if query == "" {
				continue
			}

			if len(query) > 1 && query[0] == '#' && isDigit(query[1]) {
				query = "num:" + query[1:]
			}

			results, err := bot.FTS(query, 0)
			if err != nil {
				switch {
				case err == ErrNoHits:
					desc := fmt.Sprintf("No search results found for %q", query)
					results = make([]interface{}, 1)

					results[0] = tgbotapi.InlineQueryResultArticle{
						Type:                "article", // must be
						ID:                  "ErrNoHits",
						Title:               "No results",
						Description:         desc,
						InputMessageContent: tgbotapi.InputTextMessageContent{Text: desc},
					}
				case err.Error() == "syntax error":
					desc := fmt.Sprintf("Invalid query syntax %q", query)
					results = make([]interface{}, 1)

					results[0] = tgbotapi.InlineQueryResultArticle{
						Type:                "article", // must be
						ID:                  "ErrInvalidSyntax",
						Title:               "Invalid Syntax",
						Description:         desc,
						InputMessageContent: tgbotapi.InputTextMessageContent{Text: desc},
					}
				default:
					log.Printf("%T: %s\n", err, err)
					continue
				}
			}

			api, err := bot.Send(tgbotapi.InlineConfig{
				InlineQueryID: q.ID,
				Results:       results,
			})

			if err != nil && errors.Is(err, &json.UnmarshalTypeError{}) {
				log.Println(api, err)
			}
		}
	}
}

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}
