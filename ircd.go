package main

import (
	"strings"
	"net"
	"log"
	"bufio"
	"fmt"
	"./twistmoe"
	"os/exec"
	"strconv"
)

type IrcMessage struct {
	Prefix  string
	Command string
	Params  []string
}

func (m IrcMessage) String() string {
	s := ""
	if len(m.Prefix) > 0 {
		if m.Prefix[0] != ':' {
			s += ":"
		}
		s += m.Prefix
		s += " "
	}
	s += m.Command
	for _, p := range m.Params {
		s += " "
		if strings.Index(p, " ") != -1 && p[0] != ':' {
			s += ":"
			//TODO: consider validating
		}
		s += p
	}
	return s
}

func SplitIrcParams(params string) []string {
	p := []string{}
	for len(params) > 0 {
		params = strings.TrimLeft(params, " ")
		if s := strings.Index(params, " "); params[0] == ':' || s == -1 {
			p = append(p, params)
			params = ""
		} else {
			p = append(p, params[:s])
			params = strings.TrimLeft(params[s:], " ")
		}
	}
	return p
}

func ParseIrcMessage(message string) IrcMessage {
	log.Print(message)
	m := strings.Trim(message, "\r\n")
	msg := IrcMessage{}
	if strings.HasPrefix(m, ":") {
		s := strings.Index(m, " ")
		msg.Prefix = m[:s]
		m = strings.TrimLeft(m[s:], " ")
	}
	s := strings.Index(m, " ")
	if s == -1 {
		msg.Command = m
		msg.Params = []string{}
	} else {
		msg.Command = m[:s]
		msg.Params = SplitIrcParams(strings.TrimLeft(m[s:], " "))
	}
	return msg
}

func main() {
	listener, err := net.Listen("tcp", ":6667")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
		}
		go handleConnection(conn)
	}
}
func handleConnection(conn net.Conn) {
	rd := bufio.NewReader(conn)
	defer conn.Close()
	incoming := make(chan IrcMessage, 20)
	outcoming := make(chan IrcMessage, 20)
	go push(conn, outcoming)
	go serve(incoming, outcoming)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			log.Fatal(err)
			// TODO: consider cleaner error handling
			return
		}
		msg := ParseIrcMessage(line)
		incoming <- msg
	}
}

func serve(incoming <-chan IrcMessage, outcoming chan<- IrcMessage) {
	nickname := "anon"
	for {
		msg := <-incoming
		switch msg.Command {
		case "NICK":
			nickname = msg.Params[0]
		case "USER":
			outcoming <- IrcMessage{
				Prefix:  ":ircd.twist.moe",
				Command: "001",
				Params:  []string{nickname, "Welcome to PlotTwist IRCD"},
			}
			outcoming <- IrcMessage{
				Prefix:  ":ircd.twist.moe",
				Command: "002",
				Params:  []string{nickname, "Your host is ircd.twist.moe, running in the 90s"},
			}
			outcoming <- IrcMessage{
				Prefix:  ":ircd.twist.moe",
				Command: "003",
				Params:  []string{nickname, "This server was created right now tbh"},
			}
			outcoming <- IrcMessage{
				Prefix:  ":ircd.twist.moe",
				Command: "004",
				Params:  []string{nickname, "ircd.twist.moe", "PlotTwist", "o", "vo"},
			}
		case "LIST":
			series, err := twistmoe.FetchSeriesList()
			if err != nil {
				log.Print("error while LISTing", err)
			} else {
				for _, serie := range series {
					outcoming <- IrcMessage{
						Prefix:  ":ircd.twist.moe",
						Command: "322",
						Params:  []string{nickname, "#" + serie.Name, "10", ":" + serie.Topic},
					}
				}
				outcoming <- IrcMessage{
					Prefix:  ":ircd.twist.moe",
					Command: "323",
					Params:  []string{nickname, "End of LIST"},
				}
			}
		case "JOIN":
			if msg.Params[0] == "0" {
				// TODO: don't screw RFC2812
			}
			channels := strings.Split(msg.Params[0], ",")
			for _, channel := range channels {
				channelTrim := channel
				if channel[0] == '#' {
					channelTrim = channel[1:]
				}
				episodes, err := twistmoe.FetchEpisodesList(channelTrim)
				if err != nil {
					outcoming <- IrcMessage{
						Prefix:  ":ircd.twist.moe",
						Command: "403",
						Params:  []string{nickname, channelTrim},
					}
				} else {
					outcoming <- IrcMessage{
						Prefix:  nickname,
						Command: "JOIN",
						Params:  []string{channel},
					}
					outcoming <- IrcMessage{
						Prefix:  ":ircd.twist.moe",
						Command: "332",
						Params:  []string{nickname, channel, ":" + episodes.Topic()},
					}
					names := []string{"@" + nickname}
					for _, episode := range episodes.Episodes {
						names = append(names, episode.Username())
						// TODO: handle voiced episode
					}
					outcoming <- IrcMessage{
						":ircd.twist.moe",
						"353",
						[]string{nickname, "=", channel, ":" + strings.Join(names, " ")},
					}
					outcoming <- IrcMessage{
						":ircd.twist.moe",
						"366",
						[]string{nickname, channel, ":End of names"},
					}
				}
			}
		case "WHO":
			channel := msg.Params[0][1:]
			episodes, err := twistmoe.FetchEpisodesList(channel)
			if err != nil {
				outcoming <- IrcMessage{
					Prefix:  ":ircd.twist.moe",
					Command: "403",
					Params:  []string{nickname, "#" + channel},
				}
			} else {
				for _, episode := range episodes.Episodes {
					outcoming <- IrcMessage{
						":ircd.twist.moe",
						"352",
						[]string{nickname, "#" + channel, episode.Username(), "twist.moe", "twist.moe", episode.Username(), "H", ":0 Episode"},
					}
					// TODO: handle voiced episode
				}
				outcoming <- IrcMessage{
					":ircd.twist.moe",
					"315",
					[]string{nickname, "#" + channel, ":End of who"},
				}
			}
		case "PART":
			// TODO: unbodge
			outcoming <- IrcMessage{
				Prefix:  nickname,
				Command: "PART",
				Params:  []string{msg.Params[0]},
			}
		case "MODE":
			// TODO: do it right, handle multiple
			channel := msg.Params[0]
			if len(msg.Params) < 2 {
				outcoming <- IrcMessage{
					":ircd.twist.moe",
					"324",
					[]string{nickname, channel, "+"},
				}
				continue
			}
			mode := msg.Params[1]
			target := msg.Params[2]
			if mode != "+v" {
				outcoming <- IrcMessage{
					":ircd.twist.moe",
					"472",
					[]string{nickname, mode[1:], "is unknown mode"},
				}
				continue
			}
			if channel[1:] != target[:strings.Index(target, "--")] {
				outcoming <- IrcMessage{
					":ircd.twist.moe",
					"441",
					[]string{nickname, target[:strings.Index(target, "--")], channel, "not gonna happen"},
				}
				continue
			}
			outcoming <- IrcMessage{
				nickname,
				"MODE",
				msg.Params,
			}
			targetSplit := strings.Split(target, "--")
			series, epnums := targetSplit[0], targetSplit[1]
			epnum, err := strconv.Atoi(epnums)
			if err != nil {
				// TODO: handle bad user input
			}
			go log.Print(exec.Command("mpv", fmt.Sprintf("https://twist.moe/a/%v/%v", series, epnum)).Run())
			outcoming <- IrcMessage{
				":ircd.twist.moe",
				"MODE",
				[]string{channel, "-v", target},
			}
		case "CAP":
			continue
		default:
			log.Print("unknown message", msg)
		}
	}
}

func push(conn net.Conn, messages chan IrcMessage) {
	for {
		outmsg := <-messages
		if outmsg.Command == ":::" {
			return
		}
		log.Print(outmsg.String())
		_, err := fmt.Fprint(conn, outmsg.String()+"\r\n")
		if err != nil {
			log.Fatal(err)
			// TODO: consider cleaner error handling
			return
		}
	}
}
