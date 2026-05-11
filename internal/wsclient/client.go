package wsclient

import "github.com/ploglabs/molly-terminal/internal/model"

type Client struct{}

func New(_ string) *Client {
	return &Client{}
}

func (c *Client) Connect() error {
	return nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) Messages() <-chan model.Message {
	ch := make(chan model.Message)
	close(ch)
	return ch
}
