package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/analysis/lang/en"

	"git.sr.ht/~mendelmaleh/tgbotapi"
)

// FTS queries the bleve index and returns a []interface, its
// elements are tgbotapi.InlineQueryResultPhoto.
func (bot *Bot) FTS(query string, n int) ([]interface{}, error) {
	search := bleve.NewSearchRequest(bleve.NewQueryStringQuery(query))
	search.Fields = []string{"num", "alt", "img"}
	results, err := bot.Bleve.Search(search)
	if err != nil {
		return make([]interface{}, 0), err
	}

	hits := results.Hits
	if len(hits) > n {
		hits = hits[:n]
	}

	res := make([]interface{}, len(hits))
	for i, h := range hits {
		r := &tgbotapi.InlineQueryResultPhoto{
			Type: "photo", // must be
		}

		if v, ok := h.Fields["num"].(float64); ok {
			r.ID = strconv.Itoa(int(v))
		}

		if v, ok := h.Fields["alt"].(string); ok {
			r.Caption = v
		}

		if v, ok := h.Fields["img"].(string); ok {
			r.ThumbURL = v
			r.URL = v
		}

		res[i] = r
	}

	return res, nil
}

func (bot *Bot) IndexData() error {
	files, err := ioutil.ReadDir(bot.Config.Bleve.Data)
	if err != nil {
		return err
	}

	batch := bot.Bleve.NewBatch()
	count := 0

	for _, f := range files {
		n := f.Name()

		// bytes
		jsonBytes, err := ioutil.ReadFile(bot.Config.Bleve.Data + "/" + n)
		if err != nil {
			return err
		}

		// json
		var jsonDoc interface{}
		if err := json.Unmarshal(jsonBytes, &jsonDoc); err != nil {
			return err
		}

		// index
		id := n[:len(n)-len(filepath.Ext(n))]
		batch.Index(id, jsonDoc)
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
