package main

import (
	"log"
	"strings"

	"golang.org/x/net/websocket"
	"golang.org/x/time/rate"
)

type Robot struct {
	Id            string
	Conn          *websocket.Conn
	CommandPrefix string
	postLimiter   *rate.Limiter
}

func NewRobot(commandPrefix, token string) (*Robot, error) {
	conn, id, err := slackConnect(token)
	if err != nil {
		return nil, err
	}
	r := Robot{
		id,
		conn,
		normalizeDirective(commandPrefix),
		// 1 / sec with bursts of 50 (see docs; burst is arbitrary)
		rate.NewLimiter(rate.Limit(1.0), 50),
	}
	return &r, nil
}

func (r *Robot) GetMessage() (Message, error) {
	return getMessage(r.Conn)
}

func (r *Robot) PostMessage(m Message) error {
	if !r.postLimiter.Allow() {
		// This is probably fine for most use cases
		log.Println("rate limiting message ", m)
		return nil
	}
	return postMessage(r.Conn, m)
}

type Command struct {
	Command string
	Args    string
	M       Message
}

func (r *Robot) ParseCommand(m Message) *Command {
	if m.Type != typeMessage {
		return nil
	}
	parts := strings.SplitN(m.Text, " ", 3)
	if len(parts) < 2 || normalizeDirective(parts[0]) != r.CommandPrefix {
		return nil
	}

	c := &Command{
		Command: normalizeDirective(parts[1]),
		M:       m,
	}
	if len(parts) == 3 {
		c.Args = parts[2]
	}
	return c
}

func (r *Robot) RunCommand(c *Command) error {
	var text string
	switch c.Command {
	case "echo":
		text = c.Args
	case "love":
		text = "http://stream1.gifsoup.com/view3/1783565/wall-e-and-eve-o.gif"
	case "ping":
		text = "pong"
	default:
		return nil
	}
	return r.SendMessage(c.M.Channel, text)
}

func (r *Robot) SendMessage(channel interface{}, text string) error {
	return r.PostMessage(Message{
		Type:    typeMessage,
		Channel: channel,
		Text:    text,
	})
}

func (r *Robot) Run() error {
	for {
		m, err := r.GetMessage()
		if err != nil {
			return err
		}
		c := r.ParseCommand(m)
		if c == nil {
			continue
		}
		if err := r.RunCommand(c); err != nil {
			return err
		}
	}
}

func normalizeDirective(s string) string {
	return strings.ToLower(s)
}
