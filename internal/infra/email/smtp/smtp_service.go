package smtp

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/moura95/backend-challenge/internal/domain/email"
)

type SMTPService struct {
	config email.SMTPConfig
}

func NewSMTPService(config email.SMTPConfig) *SMTPService {
	return &SMTPService{
		config: config,
	}
}

func (s *SMTPService) SendEmail(ctx context.Context, emailEntity *email.Email) error {
	// Preparar dados do email
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Construir headers
	headers := make(map[string]string)
	headers["From"] = s.config.From
	headers["To"] = emailEntity.To
	headers["Subject"] = emailEntity.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"utf-8\""

	// Construir mensagem
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + emailEntity.Body

	// Endereço do servidor
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Enviar email
	err := smtp.SendMail(
		addr,
		auth,
		s.config.From,
		[]string{emailEntity.To},
		[]byte(message),
	)

	if err != nil {
		return fmt.Errorf("smtp: failed to send email: %w", err)
	}

	fmt.Printf("Email sent successfully to %s\n", emailEntity.To)
	return nil
}

// Versão sem autenticação para desenvolvimento (MailCatcher)
func NewSMTPServiceDev(host string, port int, from string) *SMTPService {
	return &SMTPService{
		config: email.SMTPConfig{
			Host:     host,
			Port:     port,
			Username: "", // Sem auth para dev
			Password: "", // Sem auth para dev
			From:     from,
		},
	}
}

func (s *SMTPService) SendEmailDev(ctx context.Context, emailEntity *email.Email) error {

	// Construir headers
	headers := make(map[string]string)
	headers["From"] = s.config.From
	headers["To"] = emailEntity.To
	headers["Subject"] = emailEntity.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"utf-8\""

	// Construir mensagem
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + emailEntity.Body

	// Endereço do servidor
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Conectar sem autenticação
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dev: failed to connect: %w", err)
	}
	defer client.Close()

	// Configurar remetente
	if err = client.Mail(s.config.From); err != nil {
		return fmt.Errorf("smtp dev: failed to set sender: %w", err)
	}

	// Configurar destinatário
	if err = client.Rcpt(emailEntity.To); err != nil {
		return fmt.Errorf("smtp dev: failed to set recipient: %w", err)
	}

	// Enviar dados
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp dev: failed to get data writer: %w", err)
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("smtp dev: failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("smtp dev: failed to close writer: %w", err)
	}

	fmt.Printf("Email sent successfully to %s (dev mode)\n", emailEntity.To)
	return nil
}

func (s *SMTPService) SendEmailAuto(ctx context.Context, emailEntity *email.Email) error {
	// Se não tem username/password, usar modo dev
	if s.config.Username == "" && s.config.Password == "" {
		return s.SendEmailDev(ctx, emailEntity)
	}

	// Senão usar modo com autenticação
	return s.SendEmail(ctx, emailEntity)
}
