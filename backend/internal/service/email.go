package service

import (
	"fmt"

	"github.com/resend/resend-go/v2"
)

type EmailService struct {
	client      *resend.Client
	fromEmail   string
	frontendURL string
}

func NewEmailService(apiKey, fromEmail, frontendURL string) *EmailService {
	client := resend.NewClient(apiKey)
	return &EmailService{
		client:      client,
		fromEmail:   fromEmail,
		frontendURL: frontendURL,
	}
}

func (es *EmailService) SendVerificationEmail(to, token string) error {
	verificationURL := fmt.Sprintf("%s/verify-email?token=%s", es.frontendURL, token)

	htmlContent := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8">
		<title>Verify Your Email</title>
	</head>
	<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
		<h2 style="color: #2c3e50;">Verify Your Email Address</h2>
		<p>Thank you for registering with PaperTrader!</p>
		<p>Please click the button below to verify your email address:</p>
		<div style="text-align: center; margin: 30px 0;">
			<a href="%s" style="background-color: #3498db; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Verify Email</a>
		</div>
		<p>Or copy and paste this link into your browser:</p>
		<p style="word-break: break-all; color: #7f8c8d;">%s</p>
		<p style="margin-top: 30px; font-size: 12px; color: #95a5a6;">This link will expire in 24 hours.</p>
	</body>
	</html>
	`, verificationURL, verificationURL)

	params := &resend.SendEmailRequest{
		From:    es.fromEmail,
		To:      []string{to},
		Subject: "Verify Your Email Address - PaperTrader",
		Html:    htmlContent,
	}

	_, err := es.client.Emails.Send(params)
	return err
}

