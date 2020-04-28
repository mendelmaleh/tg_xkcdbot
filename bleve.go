package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"html"
	"strconv"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/analysis/lang/en"

	"git.sr.ht/~mendelmaleh/tgbotapi"
	"git.sr.ht/~mendelmaleh/xkcd"
)

var ErrNoHits = errors.New("no search results")

// FTS queries the bleve index and returns a []interface, its
// elements are tgbotapi.InlineQueryResultPhoto.
func (bot *Bot) FTS(query string, max int) ([]interface{}, error) {
	search := bleve.NewSearchRequest(bleve.NewQueryStringQuery(query))
	results, err := bot.Bleve.Search(search)
	if err != nil {
		return make([]interface{}, 0), err
	}

	hits := results.Hits
	if len(hits) == 0 {
		return make([]interface{}, 0), ErrNoHits
	} else if max > 0 && len(hits) > max {
		hits = hits[:max]
	}

	res := make([]interface{}, len(hits))
	for i, h := range hits {
		d, err := bot.Bleve.Document(h.ID)
		if err != nil {
			return make([]interface{}, 0), err
		}

		var c xkcd.Comic
		for _, f := range d.Fields {
			switch f.Name() {
			case "title":
				c.Title = string(f.Value())
			case "alt":
				c.Alt = string(f.Value())
			case "img":
				c.Img = string(f.Value())
			}
		}

		res[i] = tgbotapi.InlineQueryResultPhoto{
			Type:      "photo", // must be
			ID:        h.ID,
			URL:       c.Img,
			ThumbURL:  c.Img,
			Title:     c.Title,
			ParseMode: "html",
			Caption: fmt.Sprintf("<a href=\"%s\">#%s:</a> <i>%s</i>",
				xkcd.DefaultBaseURL+h.ID, h.ID, html.EscapeString(c.Alt)),
		}
	}

	return res, nil
}

func (bot *Bot) Update() error {
	lastByte, err := bot.Bleve.GetInternal(lastKey)
	if err != nil {
		return err
	}

	var lastIndex int
	switch cap(lastByte) {
	case 0:
		lastIndex = 0
	case 4:
		lastIndex = int(binary.LittleEndian.Uint32(lastByte))
	default:
		return fmt.Errorf(
			"can't get int from lastByte %q, type %T, len %d, cap %d\n",
			lastByte, lastByte, len(lastByte), cap(lastByte),
		)
	}

	if lastIndex == 0 {
		lastIndex = 1
	}

	lastComic, err := bot.XKCD.GetNum(0)
	if err != nil {
		return err
	}

	batch := bot.Bleve.NewBatch()
	count := 0

	for i := int(lastIndex); i <= lastComic.Num; i++ {
		if i == 404 {
			continue
		}

		// get json, marshal
		c, err := bot.XKCD.GetNum(i)
		if err != nil {
			return err
		}

		// index
		batch.Index(strconv.Itoa(c.Num), c)
		count++

		// exec batch
		if count >= 100 {
			if err := bot.Bleve.Batch(batch); err != nil {
				return err
			}

			batch = bot.Bleve.NewBatch()
			count = 0
		}
	}

	// last batch
	if count >= 0 {
		if err := bot.Bleve.Batch(batch); err != nil {
			return err
		}
	}

	if lastIndex < 0 || lastIndex > 0xffffffff {
		return fmt.Errorf("cannot store %d as uint32\n", lastIndex)
	}

	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(lastComic.Num))
	bot.Bleve.SetInternal(lastKey, bs)

	return nil
}

func (bot *Bot) NewBleve() (bleve.Index, error) {
	// reusable mappings
	english := bleve.NewTextFieldMapping()
	english.Analyzer = en.AnalyzerName

	key := bleve.NewTextFieldMapping()
	key.Analyzer = keyword.Name

	// create mapping
	mapping := bleve.NewDocumentStaticMapping()
	mapping.AddFieldMappingsAt("num", key)
	mapping.AddFieldMappingsAt("title", english)
	mapping.AddFieldMappingsAt("alt", english)
	mapping.AddFieldMappingsAt("transcript", english)

	// create index mapping
	indexMap := bleve.NewIndexMapping()
	indexMap.AddDocumentMapping("xkcd", mapping)

	// create index
	return bleve.New(bot.Config.Bleve.Index, indexMap)
}
