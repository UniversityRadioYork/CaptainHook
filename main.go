package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

import goirc "github.com/fluffle/goirc/client"
import "github.com/BurntSushi/toml"

type MIRCColorType int

const (
	ColorWhite MIRCColorType = iota
	ColorBlack
	ColorBlue
	ColorGreen
	ColorRed
	ColorBrown
	ColorPurple
	ColorOrange
	ColorYellow
	ColorLightGreen
	ColorCyan
	ColorLightCyan
	ColorLightBlue
	ColorPink
	ColorGrey
	ColorLightGrey
)

func MIRCColor(in string, col MIRCColorType) string {
	return string('\u0003') + strconv.Itoa(int(col)) + in + string('\u0003')
}

// Various (partial!) Github object structs, JSON is parsed into these

type User struct {
	Login string
}

type Repo struct {
	Name    string
	HTMLURL string `json:"html_url"`
}

type Issue struct {
	Number  int
	Title   string
	HTMLURL string `json:"html_url"`
}

type PRQ struct {
	Number  int
	Title   string
	HTMLURL string `json:"html_url"`
}

type IssueEvent struct {
	Action     string
	Issue      Issue
	Sender     User
	Repository Repo
}

type PRQEvent struct {
	Action     string
	Sender     User
	PRQ        PRQ
	Repository Repo
}

type RepositoryEvent struct {
	Action     string
	Sender     User
	Repository Repo
}

// CaptainHook's config struct, TOML is decoded into this
type Config struct {
	Channel string
	User    string
	Nick    string
	Server  string
}

// Maps GitHub event strings (e.g. for PRQs, issues) to colors, for make
// benefit beautiful IRC channel times/optical assault
var act2color = map[string]MIRCColorType{
	"opened":   ColorGreen,
	"reopened": ColorGreen,
	"closed":   ColorRed,
	"created":  ColorGreen,
}

// So Github's sweet small urls in their official webhook payloads are
// available for anyone to use. Who knew? Form posts to git.io, gets a short
// URL in a location header back. Cool.
func ShortenGHUrl(url2shorten string) (string, error) {
	resp, err := http.PostForm("http://git.io", url.Values{"url": {url2shorten}})
	if err != nil {
		fmt.Println(err)
	}
	if resp.StatusCode != 201 {
		return "", errors.New("git.io returned non 201 status: " + resp.Status)
	}
	return resp.Header.Get("Location"), nil
}

func main() {
	logger := log.New(os.Stdout, "", log.Lshortfile)
	rawconfig, err := ioutil.ReadFile("config.toml")
	if err != nil {
		logger.Fatal(err)
	}
	var conf Config
	if _, err := toml.Decode(string(rawconfig), &conf); err != nil {
		logger.Fatal(err)
	}
	ircmsgs := make(chan string, 10)

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT)

	ircconf := goirc.NewConfig(conf.Nick)
	ircconf.Server = conf.Server
	irc := goirc.Client(ircconf)

	if err := irc.Connect(); err != nil {
		logger.Println(err)
	}

	irc.HandleFunc("connected", func(conn *goirc.Conn, line *goirc.Line) {
		logger.Println("Connected to " + conf.Server)
		conn.Join(conf.Channel)
	})

	irc.HandleFunc("disconnected", func(conn *goirc.Conn, line *goirc.Line) {
		logger.Println("Disconnected from server! Sleeping for 1 min and retrying")
		time.Sleep(1 * time.Minute)
		if err := irc.Connect(); err != nil {
			logger.Println(err)
		}
	})

	irc.HandleFunc("PRIVMSG", func(conn *goirc.Conn, line *goirc.Line) {
		if strings.HasPrefix(line.Text(), "my") {
			conn.Privmsg(conf.Channel, strings.Replace(line.Text(), "my", MIRCColor("muh", ColorRed), -1))
		} else {
			var output string
			switch strings.Replace(line.Text(), conf.Nick+": ", "", -1) {
			case "colours!!!":
				output = ""
				for i, c := range "RAINBOWS" {
					output += MIRCColor(string(c), MIRCColorType(i+2))
				}
			case "yo", "hi", "sup", "hello", "ohai":
				output = "Salutations " + line.Nick
			}

			conn.Privmsg(conf.Channel, output)
		}
	})

	irc.HandleFunc("KICK", func(conn *goirc.Conn, line *goirc.Line) {
		logger.Println(";_; just got kicked")
		conn.Join(conf.Channel)
		conn.Privmsg(conf.Channel, line.Nick+": Muting notifications for 1 hour, I assume that's what you were trying to achieve. Asshole.")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Println(err)
		}
		if ev := r.Header.Get("X-Github-Event"); ev != "" {
			switch ev {
			case "pull_request":
				var event PRQEvent
				if err := json.Unmarshal(body, &event); err != nil {
					logger.Println(err)
				}
				switch event.Action {
				case "opened", "closed", "reopened":
					logger.Println(event.PRQ.HTMLURL)
					url, err := ShortenGHUrl(event.PRQ.HTMLURL)
					if err != nil {
						logger.Println(err)
					}
					ircmsgs <- fmt.Sprintf("[%s] PRQ #%d %s by %s: %s. %s",
						event.Repository.Name,
						event.PRQ.Number,
						MIRCColor(event.Action, act2color[event.Action]),
						event.Sender.Login,
						event.PRQ.Title,
						url)
				}
			case "issues":
				var event IssueEvent
				if err := json.Unmarshal(body, &event); err != nil {
					logger.Println(err)
				}
				switch event.Action {
				case "opened", "closed", "reopened":
					url, err := ShortenGHUrl(event.Issue.HTMLURL)
					if err != nil {
						logger.Println(err)
					}
					ircmsgs <- fmt.Sprintf("[%s] Issue #%d %s by %s: %s. %s",
						event.Repository.Name,
						event.Issue.Number,
						MIRCColor(event.Action, act2color[event.Action]),
						event.Sender.Login,
						event.Issue.Title,
						url)
				}
			case "repository":
				var event RepositoryEvent
				if err := json.Unmarshal(body, &event); err != nil {
					logger.Println(err)
				}
				switch event.Action {
				case "created":
					url, err := ShortenGHUrl(event.Repository.HTMLURL)
					if err != nil {
						logger.Println(err)
					}
					ircmsgs <- fmt.Sprintf("%s %s %s: %s",
						event.Sender.Login,
						MIRCColor(event.Action, act2color[event.Action]),
						event.Repository.Name,
						url)
				}
			}
		}
	})
	go http.ListenAndServe(":1337", nil)
	for {

		select {
		case msg := <-ircmsgs:
			fmt.Println("Sending: " + msg)
			irc.Privmsg(conf.Channel, msg)
		case <-sigs:
			irc.Quit()
			os.Exit(0)
		}
	}
}
