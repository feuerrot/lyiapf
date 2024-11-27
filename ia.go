package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/eduncan911/podcast"
)

const (
	IAMetadataPrefix = "https://archive.org/metadata/"
	IAItemPrefix     = "https://archive.org/details/"
)

type IA struct {
	Created    int        `json:"created"`
	LastUpdate int        `json:"item_last_updated"`
	Files      []IAFiles  `json:"files"`
	Dir        string     `json:"dir"`
	Server     string     `json:"server"`
	Servers    []string   `json:"workable_servers"`
	Metadata   IAMetadata `json:"metadata"`
}

func (ia *IA) OnlyOriginals() {
	var newFiles []IAFiles

	for _, file := range ia.Files {
		if file.Source == IASourceOriginal {
			newFiles = append(newFiles, file)
		}
	}

	ia.Files = newFiles
}

type IAMetadata struct {
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Title       string `json:"title"`
}

type IAFiles struct {
	Name   string   `json:"name"`
	Title  string   `json:"title"`
	Track  string   `json:"track"`
	Artist string   `json:"artist"`
	Album  string   `json:"album"`
	Source IASource `json:"source"`
	MTime  int      `json:"mtime"`
	Size   int      `json:"size"`
	Length float64  `json:"length"`
	SHA1   string   `json:"sha1"`
}

func convertLength(in string) (float64, error) {
	if in == "" {
		return 0, fmt.Errorf("empty length")
	}

	parts := strings.Split(in, ":")
	if len(parts) == 0 {
		return 0, fmt.Errorf("no length")
	}

	if len(parts) == 1 {
		len, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, fmt.Errorf("converting %q: %v", in, err)
		}
		return len, nil
	}

	if len(parts) == 2 {
		lenmin, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, fmt.Errorf("converting %q[0]: %v", in, err)
		}
		len, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, fmt.Errorf("converting %q[1]: %v", in, err)
		}
		return len + 60*lenmin, nil
	}

	if len(parts) == 3 {
		lenhour, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, fmt.Errorf("converting %q[0]: %v", in, err)
		}
		lenmin, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, fmt.Errorf("converting %q[1]: %v", in, err)
		}
		len, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, fmt.Errorf("converting %q[2]: %v", in, err)
		}
		return len + 60*lenmin + 60*60*lenhour, nil
	}

	return 0, nil
}

func (iaf *IAFiles) TitleDescription() (string, string) {
	var title string
	var description string

	if iaf.Title != "" {
		title = iaf.Title
	} else {
		title = iaf.Name
	}

	if iaf.Album != "" {
		description = iaf.Album
	} else {
		description = iaf.Name
	}

	return title, description
}

func (iaf *IAFiles) UnmarshalJSON(data []byte) error {
	var raw map[string]string

	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	iaf.Name = raw["name"]
	iaf.Track = raw["track"]
	iaf.Title = raw["title"]
	iaf.Artist = raw["artist"]
	if raw["artist"] == "" && raw["creator"] != "" {
		iaf.Artist = raw["creator"]
	}
	iaf.Album = raw["album"]
	iaf.Source = IASource(raw["source"])

	mtime, err := strconv.Atoi(raw["mtime"])
	if err == nil {
		iaf.MTime = mtime
	}

	size, err := strconv.Atoi(raw["size"])
	if err == nil {
		iaf.Size = size
	}

	length, err := convertLength(raw["length"])
	if err == nil {
		iaf.Length = length
	}
	iaf.SHA1 = raw["sha1"]

	return nil
}

type IASource string

const (
	IASourceOriginal   = "original"
	IASourceDerivative = "derivative"
)

func GetIA(identifier string) (IA, error) {
	var rtn IA

	resp, err := http.Get(IAMetadataPrefix + identifier)
	if err != nil {
		return rtn, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return rtn, err
	}

	err = json.Unmarshal(body, &rtn)
	if err != nil {
		return rtn, err
	}

	rtn.OnlyOriginals()

	return rtn, nil
}

func (ia IA) Feed() (string, error) {
	pubDate := time.Unix(int64(ia.LastUpdate), 0)

	feed := podcast.New(
		ia.Metadata.Title,
		IAItemPrefix+ia.Metadata.Identifier,
		ia.Metadata.Description,
		&pubDate,
		nil,
	)

	for _, entry := range ia.Files {
		if strings.HasSuffix(entry.Name, ".xml") || strings.HasSuffix(entry.Name, ".jpg") || strings.HasSuffix(entry.Name, ".sqlite") {
			continue
		}

		creationtime := time.Unix(int64(entry.MTime), 0)

		title, description := entry.TitleDescription()
		item := podcast.Item{
			Title:       title,
			Description: description,
			Author: &podcast.Author{
				Email: entry.Artist,
			},
			Link:    "https://" + ia.Server + ia.Dir + "/" + url.PathEscape(entry.Name),
			PubDate: &creationtime,
			GUID:    entry.SHA1,
		}

		item.AddEnclosure("https://"+ia.Server+ia.Dir+"/"+url.PathEscape(entry.Name), 0, int64(entry.Size))

		if _, err := feed.AddItem(item); err != nil {
			return "", fmt.Errorf("feed.AddItem: %w", err)
		}
	}

	buffer := new(bytes.Buffer)
	if err := feed.Encode(buffer); err != nil {
		return "", fmt.Errorf("feed.Encode: %w", err)
	}

	return buffer.String(), nil
}
