package main

import "os"
import "os/signal"
import "syscall"
import "fmt"
import "strings"
import "strconv"
import "io/ioutil"
import "net/http"

//import "time"

import ircevent "github.com/thoj/go-ircevent"

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

type PRQEvent struct {
	action string `json:"action"`
	number int    `json:"number"`
}

func main() {
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT)

	irc := ircevent.IRC("URY-Github", "URY-Github")
	irc.Connect("chat.freenode.net:6667")
	irc.Join("#ury-dev")

	irc.AddCallback("PRIVMSG", func(event *ircevent.Event) {
		fmt.Println(event.Message())
		if strings.HasPrefix(event.Message(), "my") {
			irc.Privmsg("#ury-dev", strings.Replace(event.Message(), "my", MIRCColor("muh", ColorRed), -1))
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(body)
	})
	go http.ListenAndServe(":1337", nil)

	select {
	case <-sigs:
		irc.Quit()
		os.Exit(0)
	}
}
