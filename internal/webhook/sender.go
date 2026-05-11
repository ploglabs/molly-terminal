package webhook

type Sender struct {
	webhookURL string
	username   string
}

func New(webhookURL, username string) *Sender {
	return &Sender{
		webhookURL: webhookURL,
		username:   username,
	}
}

func (s *Sender) Send(content string) error {
	return nil
}
