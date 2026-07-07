package notifications

import (
	"log/slog"
)

// EmailSender defines the interface for sending system emails.
type EmailSender interface {
	SendSellerInvitationEmail(email, temporaryPassword string) error
}

// DevEmailSender is a development implementation that logs emails instead of sending them.
type DevEmailSender struct {
	logger *slog.Logger
}

func NewDevEmailSender(logger *slog.Logger) *DevEmailSender {
	return &DevEmailSender{logger: logger}
}

func (s *DevEmailSender) SendSellerInvitationEmail(email, temporaryPassword string) error {
	// WARNING: In production, never log passwords.
	// This is exclusively for local development testing to avoid real SMTP.
	s.logger.Info("📧 [DEV EMAIL] Seller Invitation",
		"to", email,
		"temporaryPassword", temporaryPassword,
		"body", "Добро пожаловать на ZAMK! Ваш временный пароль для входа: "+temporaryPassword,
		"note", "Письмо сформировано в тестовом режиме разработки.",
	)
	return nil
}
