package dto

import "buildadmin-go/internal/admin/validate"

type MailParam struct {
	SmtpServer       string `json:"smtp_server" binding:"required"`
	SmtpPort         string `json:"smtp_port" binding:"required"`
	SmtpUser         string `json:"smtp_user" binding:"required"`
	SmtpPass         string `json:"smtp_pass" binding:"required"`
	SmtpVerification string `json:"smtp_verification" binding:"required"`
	SmtpSenderMail   string `json:"smtp_sender_mail" binding:"required"`
	TestMail         string `json:"testMail" binding:"required"`
}

func (v MailParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}
