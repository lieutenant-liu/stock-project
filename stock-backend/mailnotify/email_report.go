package mailnotify

import (
	"fmt"
	"net/smtp"
	"stock-backend/db"
	"strings"
	"time"
)

func buildRunReportMail(run *db.AutoSyncRun, steps []db.AutoSyncRunStep, subjectPrefix string) (string, string) {
	subject := fmt.Sprintf("%s run#%d %s %s", subjectPrefix, run.ID, run.RunDate, strings.ToUpper(run.Status))

	var b strings.Builder
	b.WriteString("自动任务运行报告\n")
	b.WriteString("====================\n")
	b.WriteString(fmt.Sprintf("Run ID: %d\n", run.ID))
	b.WriteString(fmt.Sprintf("日期: %s\n", run.RunDate))
	b.WriteString(fmt.Sprintf("触发方式: %s\n", run.TriggerType))
	b.WriteString(fmt.Sprintf("状态: %s\n", run.Status))
	b.WriteString(fmt.Sprintf("开始: %s\n", run.StartedAt))
	b.WriteString(fmt.Sprintf("结束: %s\n", run.FinishedAt))
	b.WriteString(fmt.Sprintf("网络失败次数: %d\n", run.NetworkFailures))
	if strings.TrimSpace(run.ErrorMsg) != "" {
		b.WriteString(fmt.Sprintf("错误信息: %s\n", run.ErrorMsg))
	}
	b.WriteString("\n步骤明细:\n")
	for _, s := range steps {
		b.WriteString(fmt.Sprintf("- %s | status=%s targeted=%d success=%d failed=%d skipped=%d\n", s.StepName, s.Status, s.Targeted, s.Success, s.Failed, s.Skipped))
		if strings.TrimSpace(s.ErrorMsg) != "" {
			b.WriteString(fmt.Sprintf("  err: %s\n", s.ErrorMsg))
		}
	}
	b.WriteString("\n原始 summary_json:\n")
	b.WriteString(run.SummaryJSON)
	b.WriteString("\n\nGenerated At: ")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n")
	return subject, b.String()
}

func sendMailSMTP(cfg db.EmailNotifyConfig, recipients []string, subject, body string) error {
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return fmt.Errorf("SMTP Host 不能为空")
	}
	if cfg.SMTPPort <= 0 {
		return fmt.Errorf("SMTP Port 非法")
	}
	if strings.TrimSpace(cfg.SMTPUser) == "" {
		return fmt.Errorf("SMTP 用户名不能为空")
	}
	if strings.TrimSpace(cfg.SMTPPass) == "" {
		return fmt.Errorf("SMTP 密码不能为空")
	}
	if strings.TrimSpace(cfg.SMTPFrom) == "" {
		return fmt.Errorf("发件邮箱不能为空")
	}
	if len(recipients) == 0 {
		return fmt.Errorf("收件人为空")
	}

	host := strings.TrimSpace(cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", host, cfg.SMTPPort)
	auth := smtp.PlainAuth("", strings.TrimSpace(cfg.SMTPUser), cfg.SMTPPass, host)

	msg := strings.Builder{}
	msg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.SMTPFrom))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ",")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return smtp.SendMail(addr, auth, cfg.SMTPFrom, recipients, []byte(msg.String()))
}

func SendRunReport(runID int64, recipientIDs []int64) error {
	cfg, err := db.GetEmailNotifyConfig()
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return fmt.Errorf("邮件推送未启用")
	}

	run, err := db.GetAutoSyncRunByID(runID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("run_id=%d 不存在", runID)
	}

	steps, err := db.ListAutoSyncRunSteps(runID)
	if err != nil {
		return err
	}

	recipients, err := db.GetEnabledRecipientEmailsByIDs(recipientIDs)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return fmt.Errorf("没有可用收件人")
	}

	subjectPrefix := cfg.SubjectPrefix
	if strings.TrimSpace(subjectPrefix) == "" {
		subjectPrefix = "[Stock-AutoSync]"
	}
	subject, body := buildRunReportMail(run, steps, subjectPrefix)
	return sendMailSMTP(cfg, recipients, subject, body)
}

func AutoSendRunReport(runID int64) error {
	cfg, err := db.GetEmailNotifyConfig()
	if err != nil {
		return err
	}
	if !cfg.Enabled || !cfg.AutoSendDaily {
		return nil
	}
	return SendRunReport(runID, nil)
}
