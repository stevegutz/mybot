package main

import (
	"log"
	"sort"
	"strings"

	"golang.org/x/net/websocket"
	"golang.org/x/time/rate"
)

type Robot struct {
	Id            string
	Conn          *websocket.Conn
	CommandPrefix string
	postLimiter   *rate.Limiter
	actions       map[string]Action
}

type Action struct {
	Keyword string
	Desc    string
	Run     func(*Command) error
}

func NewRobot(commandPrefix, token string) (*Robot, error) {
	conn, id, err := slackConnect(token)
	if err != nil {
		return nil, err
	}
	r := Robot{
		Id:            id,
		Conn:          conn,
		CommandPrefix: normalizeDirective(commandPrefix),
		// 1 / sec with bursts of 50 (see docs; burst is arbitrary)
		postLimiter: rate.NewLimiter(rate.Limit(1.0), 50),
	}

	// TODO: There's definitely a nicer way to define this
	actions := []Action{
		Action{"echo", "<text> - reply with <text>", r.echo},
		Action{"help", "- display all commands", r.help},
		Action{"love", "- approximate emotion", r.love},
		Action{"ping", "- reply with pong", r.ping},
	}
	r.actions = make(map[string]Action, len(actions))
	for _, a := range actions {
		r.actions[normalizeDirective(a.Keyword)] = a
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
	if a, ok := r.actions[c.Command]; ok {
		return a.Run(c)
	} else {
		return nil
	}
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

func (r *Robot) echo(c *Command) error {
	return r.SendMessage(c.M.Channel, c.Args)
}

func (r *Robot) help(c *Command) error {
	keys := make([]string, 0, len(r.actions))
	for k, _ := range r.actions {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var msg string
	for i, k := range keys {
		if i != 0 {
			msg += "\n"
		}
		msg += k + " " + r.actions[k].Desc
	}

	return r.SendMessage(c.M.Channel, msg)
}

func (r *Robot) love(c *Command) error {
	return r.SendMessage(c.M.Channel,
		"http://stream1.gifsoup.com/view3/1783565/wall-e-and-eve-o.gif")
}

func (r *Robot) ping(c *Command) error {
	return r.SendMessage(c.M.Channel, "pong")
}

func normalizeDirective(s string) string {
	return strings.ToLower(s)
}
