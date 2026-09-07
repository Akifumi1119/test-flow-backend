package email

import (
	"fmt"

	"github.com/resend/resend-go/v2"
)

type Client struct {
	client *resend.Client
	from   string
}

func NewClient(apiKey, from string) *Client {
	return &Client{
		client: resend.NewClient(apiKey),
		from:   from,
	}
}

func (c *Client) SendVerification(toEmail, toName, code string) error {
	params := &resend.SendEmailRequest{
		From:    c.from,
		To:      []string{toEmail},
		Subject: "【タスク管理】メールアドレスの確認",
		Html: fmt.Sprintf(`
<p>%s さん、ご登録ありがとうございます。</p>
<p>以下の確認コードを入力してください：</p>
<h2 style="letter-spacing: 4px;">%s</h2>
<p>このコードは10分間有効です。</p>
`, toName, code),
	}
	_, err := c.client.Emails.Send(params)
	return err
}
