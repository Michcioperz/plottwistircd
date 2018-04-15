package twistmoe

import (
	"errors"
	"strings"
	"golang.org/x/net/html"
	"fmt"
	"regexp"
	"net/http"
	"io/ioutil"
	"encoding/json"
)

type Series struct {
	Name string
	Topic string
}

var SeriesListRegexp = regexp.MustCompile(`<a href="/a/[a-zA-Z0-9-]+?" class="series-title" data-title="[^"]*?"(?: data-alt="[^"]*?")?>[^<]+`)
const SeriesListAppendix = `</a>`

func FetchSeriesList() ([]Series, error) {
	body, err := FetchPageContents("https://twist.moe")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(body, "\n")
	series := make([]Series, 0)
	for _, linee := range lines {
		line := strings.TrimSpace(linee)
		if SeriesListRegexp.MatchString(line) {
			htmlLine := strings.NewReader(line + SeriesListAppendix)
			htmlElem, err := html.Parse(htmlLine)
			if err != nil {
				return nil, err
			}
			for (htmlElem.Type != html.ElementNode || htmlElem.Data != "a") && htmlElem.FirstChild != nil {
				htmlElem = htmlElem.FirstChild
				if htmlElem.Type == html.ElementNode && htmlElem.Data == "head" && htmlElem.NextSibling != nil {
					htmlElem = htmlElem.NextSibling
				}
			}
			if !(htmlElem.Type == html.ElementNode && htmlElem.Data == "a") {
				return nil, fmt.Errorf("parsing error: instead of <a> tag we found %#v <%v>", htmlElem.Type, htmlElem.Data)
			}
			var htmlHref string = ""
			var htmlTitle string = ""
			var htmlAlt string = ""
			for _, attr := range htmlElem.Attr {
				switch attr.Key {
				case "href":
					htmlHref = attr.Val
				case "data-title":
					htmlTitle = attr.Val
				case "data-alt":
					htmlAlt = attr.Val
				}
			}
			if htmlHref == "" || htmlTitle == "" {
				// silently ignore completely nonsense error
				continue
			}
			niceTitle := htmlTitle
			if htmlAlt != "" {
				niceTitle += " (" + htmlAlt + ")"
			}
			serie := Series{Topic: niceTitle, Name: htmlHref[3:]}
			series = append(series, serie)
		}
	}
	return series, nil
}
func FetchPageContents(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}


type Episode struct {
	Source string
	Number int
	Slug string
}

func (episode Episode) Username() string {
	return fmt.Sprintf("%v--%03d", episode.Slug, episode.Number)
}

type SeriesDetail struct {
	Title    string
	AltTitle string
	Episodes []Episode
}

func (detail *SeriesDetail) Topic() string {
	if detail.AltTitle != "" {
		return detail.Title + " (" + detail.AltTitle + ")"
	}
	return detail.Title
}

const JsonPrefixLine = `<script id="series-object" type="application/json">`

func FetchEpisodesList(series string) (*SeriesDetail, error) {
	body, err := FetchPageContents("https://twist.moe/a/" + series)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(body, "\n")
	found := false
	var i int
	var line string
	for i, line = range lines {
		if strings.TrimSpace(line) == JsonPrefixLine {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("series-object not found in series page")
	}
	seriesObjectBody := strings.TrimSpace(lines[i+1])
	var s SeriesDetail
	err = json.Unmarshal([]byte(seriesObjectBody), &s)
	if err != nil {
		return nil, err
	}
	for i, e := range s.Episodes {
		s.Episodes[i].Source = "https://twist.moe" + strings.TrimSpace(e.Source)
		s.Episodes[i].Slug = series
	}
	return &s, nil
}
