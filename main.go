package main

import "os"
import "os/signal"
import "syscall"
import "fmt"
import "time"

import ircevent "github.com/thoj/go-ircevent"

func main() {
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT)
	irc := ircevent.IRC("URY-Github", "URY-Github")
	irc.Connect("chat.freenode.net:6667")
	time.Sleep(5 * time.Second)
	irc.Join("#ury")

	irc.AddCallback("MSG", func(event *ircevent.Event) {
		fmt.Println(event.Message())
	})
	select {
	case <-sigs:
		irc.Quit()
		os.Exit(0)
	}
}
