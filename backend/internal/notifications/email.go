package notifications

import (
	"log/slog"
	"strings"
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

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	name := parts[0]
	if len(name) <= 2 {
		name = string(name[0]) + "***"
	} else {
		name = string(name[0]) + "***" + string(name[len(name)-1])
	}
	return name + "@" + parts[1]
}

func (s *DevEmailSender) SendSellerInvitationEmail(email, temporaryPassword string) error {
	s.logger.Info("📧 [DEV EMAIL] Seller Invitation",
		slog.String("to", maskEmail(email)),
		slog.String("status", "invitation_created"))
	return nil
}
