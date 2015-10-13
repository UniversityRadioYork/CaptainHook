package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/koding/multiconfig"
	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
)

// These are in string format as not having a leading zero can mess
// up some clients when the string to colorize starts with a number.
type MIRCColor string

const (
	ColorWhite      MIRCColor = "00"
	ColorBlack                = "01"
	ColorBlue                 = "02"
	ColorGreen                = "03"
	ColorRed                  = "04"
	ColorBrown                = "05"
	ColorPurple               = "06"
	ColorOrange               = "07"
	ColorYellow               = "08"
	ColorLightGreen           = "09"
	ColorCyan                 = "10"
	ColorLightCyan            = "11"
	ColorLightBlue            = "12"
	ColorPink                 = "13"
	ColorGrey                 = "14"
	ColorLightGrey            = "15"
)

// Take a string and insert irc formatting codes around it.
func IrcColorize(in string, fg MIRCColor) string {
	return string('\x03') + string(fg) + in + string('\x0F')
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
	Merged  bool
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
	PRQ        PRQ `json:"pull_request"`
	Repository Repo
}

type RepositoryEvent struct {
	Action     string
	Sender     User
	Repository Repo
}

// CaptainHook's config struct
type Config struct {
	Channels string `required`
	Server   string `default:"chat.freenode.net:6667"`
	Nick     string `default:"CaptHook"`
	Ident    string `default:"capthook"`
	Name     string `default:"The Captain"`
	HostPort string `default:":4665"` // HTTP listen host and port
	Join     bool   `default:"true"`
	GHSecret string `required` // The Github webhook secret
}

// Maps GitHub event strings (e.g. for PRQs, issues) to colors, for make
// benefit beautiful IRC channel times/optical assault
var act2color = map[string]MIRCColor{
	"opened":   ColorGreen,
	"reopened": ColorGreen,
	"closed":   ColorRed,
	"created":  ColorGreen,
}

// So Github's sweet small urls in their official webhook payloads are
// available for anyone to use. Who knew? Form posts to git.io, gets a short
// URL in a location header back. Cool.
func ShortenGHUrl(url2shorten string) (shorturl string, err error) {
	shorturl = url2shorten // Initially set the return url to this, in case of error
	resp, err := http.PostForm("http://git.io", url.Values{"url": {url2shorten}})
	if err != nil {
		return
	}
	if resp.StatusCode != 201 {
		err = errors.New("git.io returned non 201 status: " + resp.Status)
		return
	}
	shorturl = resp.Header.Get("Location")
	return
}

func CheckHMAC(message, reqMAC, key []byte) bool {
	mac := hmac.New(sha1.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(reqMAC, expectedMAC) // It's the return of the mac
}

var conf *Config

func HandleConnected(s ircx.Sender, m *irc.Message, logger *log.Logger) {
	logger.Println("Connected to " + conf.Server)
	if conf.Join {
		logger.Println("Joining " + conf.Channels)
		for _, c := range strings.Split(conf.Channels, ",") {
			s.Send(&irc.Message{
				Command: irc.JOIN,
				Params:  []string{c},
			})
		}
	}

}

func HandlePrivMsg(s ircx.Sender, m *irc.Message, logger *log.Logger) {
	logger.Println(m)
	/*
		if strings.HasPrefix(m), conf.Nick+":") { // Someone mentioned us
			var output string
			mention := strings.TrimSpace(
				strings.TrimPrefix(line.Text(), conf.Nick+":"))
			switch mention {
			case "yo", "hi", "sup", "hello", "ohai", "wb", "evening", "morning", "afternoon":
				output = "Well met, " + line.Nick
			case "reload", "restart", "reboot", "eat toml":
				// Reload config
			}
			channel := line.Args[0]
			logger.Println("Sending " + output + " to " + channel)
			conn.Privmsg(channel, output)
		}
	*/
}

func main() {
	logger := log.New(os.Stdout, "", log.Lshortfile)
	conf = new(Config)
	var m *multiconfig.DefaultLoader
	if _, err := os.Stat("config.toml"); os.IsNotExist(err) {
		m = multiconfig.New()
	} else {
		m = multiconfig.NewWithPath("config.toml")
	}
	if err := m.Load(conf); err != nil {
		logger.Fatal("Config load failed!" + err.Error())
	}
	broadcastmsgs := make(chan string, 10)

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT)

	bot := ircx.Classic(conf.Server, conf.Nick)
	if err := bot.Connect(); err != nil {
		logger.Fatalln("Unable to dial IRC Server ", err)
	}

	bot.HandleFunc(irc.RPL_WELCOME, func(s ircx.Sender, m *irc.Message) {
		HandleConnected(s, m, logger)
	})

	bot.HandleFunc(irc.PING, func(s ircx.Sender, m *irc.Message) {
		s.Send(&irc.Message{
			Command:  irc.PONG,
			Params:   m.Params,
			Trailing: m.Trailing,
		})
	})

	go bot.HandleLoop()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Println("Error reading response body: " + err.Error())
		}
		reqMAC, err := hex.DecodeString(strings.Split(r.Header.Get("X-Hub-Signature"), "=")[1])
		if err != nil {
			logger.Println("Error decoding HMAC header: " + err.Error())
		}
		if CheckHMAC(body, reqMAC, []byte(conf.GHSecret)) {
			if ev := r.Header.Get("X-Github-Event"); ev != "" {
				switch ev {
				case "pull_request":
					var event PRQEvent
					if err := json.Unmarshal(body, &event); err != nil {
						logger.Println("Error unmarshalling JSON: " + err.Error())
					}
					switch event.Action {
					case "opened", "closed", "reopened":
						logger.Println(event.PRQ.HTMLURL)
						url, err := ShortenGHUrl(event.PRQ.HTMLURL)
						if err != nil {
							logger.Println("Error shortening URL: " + err.Error())
						}
						// PRQs are a bit special -_-
						// The PRQ has a 'merged' key instead of a merged
						// event, so we explicitly check for that.
						action := IrcColorize(event.Action, act2color[event.Action])
						if event.PRQ.Merged {
							action = IrcColorize("Merged", ColorBlue)
						}
						broadcastmsgs <- fmt.Sprintf("[%s] PRQ #%d %s by %s: %s. %s",
							IrcColorize(event.Repository.Name, ColorPurple),
							event.PRQ.Number,
							action,
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
							logger.Println("Error shortening URL: " + err.Error())
						}
						broadcastmsgs <- fmt.Sprintf("[%s] Issue #%d %s by %s: %s. %s",
							IrcColorize(event.Repository.Name, ColorPurple),
							event.Issue.Number,
							IrcColorize(event.Action, act2color[event.Action]),
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
							logger.Println("Error shortening URL: " + err.Error())
						}
						broadcastmsgs <- fmt.Sprintf("%s %s %s: %s",
							event.Sender.Login,
							IrcColorize(event.Action, act2color[event.Action]),
							IrcColorize(event.Repository.Name, ColorPurple),
							url)
					}
				}
			}
		} else {
			logger.Println("Invalid/missing HMAC in request")
		}
	})
	go http.ListenAndServe(conf.HostPort, nil)
	for {
		select {
		case msg := <-broadcastmsgs:
			fmt.Println("Sending: " + msg)
			for _, c := range strings.Split(conf.Channels, ",") {
				bot.Sender.Send(&irc.Message{
					Command:  irc.NOTICE,
					Params:   []string{c},
					Trailing: msg,
				})
			}
		case <-sigs:
			logger.Println("Sending quit")
			bot.Sender.Send(&irc.Message{
				Command:  irc.QUIT,
				Trailing: "RIP in pepparoni",
			})
		}
	}
}
