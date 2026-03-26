package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"kyron-medical/models"
)

func SendConfirmationEmail(appt *models.Appointment) {
	html := buildConfirmationHTML(appt)
	sendEmail(
		appt.Patient.Email,
		fmt.Sprintf("Appointment Confirmed — Dr. %s at Kyron Medical", appt.Doctor.Name),
		html,
	)
}

func SendReminderEmail(appt *models.Appointment) {
	html := buildReminderHTML(appt)
	sendEmail(
		appt.Patient.Email,
		fmt.Sprintf("Reminder: Your appointment tomorrow with Dr. %s", appt.Doctor.Name),
		html,
	)
}

func ScheduleReminder(appt *models.Appointment) {
	apptDate, err := time.Parse("2006-01-02", appt.Slot.Date)
	if err != nil {
		log.Printf("warn: ScheduleReminder: could not parse slot date %q for appt %s: %v", appt.Slot.Date, appt.ID[:8], err)
		return
	}

	hour, err := time.Parse("15:04", appt.Slot.StartTime)
	if err != nil {
		log.Printf("warn: ScheduleReminder: could not parse slot time %q for appt %s: %v", appt.Slot.StartTime, appt.ID[:8], err)
		return
	}

	apptTime := time.Date(apptDate.Year(), apptDate.Month(), apptDate.Day(),
		hour.Hour(), hour.Minute(), 0, 0, clinicLoc)

	reminderTime := apptTime.Add(-24 * time.Hour)
	delay := time.Until(reminderTime)
	if delay <= 0 {
		log.Printf("info: ScheduleReminder: appt %s is in the past or within 24h, skipping reminder", appt.ID[:8])
		return
	}

	time.AfterFunc(delay, func() {
		SendReminderEmail(appt)
		if appt.Patient.SMSOptIn {
			SendReminderSMS(appt)
		}
	})

	log.Printf("reminder scheduled for %s (in %.1f hours)", appt.Patient.Email, delay.Hours())
}

func sendEmail(to, subject, html string) {
	apiKey := os.Getenv("RESEND_API_KEY")
	fromEmail := os.Getenv("RESEND_FROM_EMAIL")
	if apiKey == "" || fromEmail == "" {
		log.Printf("email skipped (no credentials): to=%s subject=%s", to, subject)
		return
	}

	payload := map[string]interface{}{
		"from":    fromEmail,
		"to":      []string{to},
		"subject": subject,
		"html":    html,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("email send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("email send failed: status %d", resp.StatusCode)
	} else {
		log.Printf("email sent: to=%s", to)
	}
}

func buildConfirmationHTML(appt *models.Appointment) string {
	calDate := appt.Slot.Date
	calTime := appt.Slot.StartTime
	ctz := url.QueryEscape(clinicLoc.String())

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:#121723;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#121723;padding:40px 20px;">
    <tr><td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:rgba(87,125,232,0.08);border:1px solid rgba(72,172,240,0.25);border-radius:16px;overflow:hidden;max-width:600px;width:100%%;">
        <!-- Header -->
        <tr><td style="background:linear-gradient(135deg,#577DE8,#48ACF0);padding:32px 40px;text-align:center;">
          <div style="font-size:24px;font-weight:700;color:#fff;letter-spacing:-0.5px;">Kyron Medical</div>
          <div style="font-size:14px;color:rgba(255,255,255,0.8);margin-top:4px;">Appointment Confirmed ✓</div>
        </td></tr>
        <!-- Body -->
        <tr><td style="padding:40px;">
          <p style="color:#fff;font-size:18px;margin:0 0 24px;">Hi %s,</p>
          <p style="color:rgba(255,255,255,0.8);font-size:15px;margin:0 0 32px;">Your appointment has been confirmed. Here are the details:</p>
          <!-- Detail card -->
          <table width="100%%" cellpadding="0" cellspacing="0" style="background:rgba(255,255,255,0.05);border:1px solid rgba(255,255,255,0.1);border-radius:12px;margin-bottom:32px;">
            <tr><td style="padding:24px;">
              <table width="100%%" cellpadding="0" cellspacing="8">
                <tr><td style="color:#8892a4;font-size:13px;padding-bottom:4px;">DOCTOR</td></tr>
                <tr><td style="color:#fff;font-size:16px;font-weight:600;padding-bottom:16px;">%s<br><span style="color:#48ACF0;font-size:14px;font-weight:400;">%s</span></td></tr>
                <tr><td style="color:#8892a4;font-size:13px;padding-bottom:4px;">DATE &amp; TIME</td></tr>
                <tr><td style="color:#fff;font-size:16px;font-weight:600;padding-bottom:16px;">%s<br><span style="color:#48ACF0;font-size:14px;font-weight:400;">%s – %s</span></td></tr>
                <tr><td style="color:#8892a4;font-size:13px;padding-bottom:4px;">LOCATION</td></tr>
                <tr><td style="color:#fff;font-size:15px;padding-bottom:16px;">1250 Healthcare Blvd, Suite 400<br>San Francisco, CA 94105</td></tr>
                <tr><td style="color:#8892a4;font-size:13px;padding-bottom:4px;">CONFIRMATION #</td></tr>
                <tr><td style="color:#2EA84A;font-size:15px;font-weight:600;font-family:monospace;">%s</td></tr>
              </table>
            </td></tr>
          </table>
          <!-- Calendar links -->
          <p style="color:rgba(255,255,255,0.6);font-size:13px;margin:0 0 8px;">Add to calendar:</p>
          <a href="https://calendar.google.com/calendar/r/eventedit?text=Kyron+Medical+Appointment&dates=%sT%s00/%sT%s00&ctz=%s&location=1250+Healthcare+Blvd,+San+Francisco,+CA&details=Appointment+with+%s" style="display:inline-block;margin-right:8px;padding:8px 16px;background:#577DE8;color:#fff;text-decoration:none;border-radius:8px;font-size:13px;">Google Calendar</a>
        </td></tr>
        <tr><td style="padding:24px 40px;border-top:1px solid rgba(255,255,255,0.08);text-align:center;">
          <p style="color:#8892a4;font-size:13px;margin:0;">Kyron Medical · (555) 201-0000 · kyronmedical.com</p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`,
		appt.Patient.FirstName,
		appt.Doctor.Name, appt.Doctor.Specialty,
		FormatDateReadable(appt.Slot.Date), FormatTimeReadable(appt.Slot.StartTime), FormatTimeReadable(appt.Slot.EndTime),
		appt.ID[:8],
		// Google Calendar date params (basic — removes dashes/colons)
		removeChars(calDate, "-"), removeChars(calTime, ":")+":00",
		removeChars(calDate, "-"), removeChars(appt.Slot.EndTime, ":")+":00",
		ctz,
		appt.Doctor.Name,
	)
}

func buildReminderHTML(appt *models.Appointment) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background:#121723;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#121723;padding:40px 20px;">
    <tr><td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:rgba(87,125,232,0.08);border:1px solid rgba(72,172,240,0.25);border-radius:16px;overflow:hidden;max-width:600px;width:100%%;">
        <tr><td style="background:linear-gradient(135deg,#577DE8,#48ACF0);padding:32px 40px;text-align:center;">
          <div style="font-size:24px;font-weight:700;color:#fff;">Kyron Medical</div>
          <div style="font-size:14px;color:rgba(255,255,255,0.8);margin-top:4px;">Appointment Reminder 🗓</div>
        </td></tr>
        <tr><td style="padding:40px;">
          <p style="color:#fff;font-size:18px;margin:0 0 16px;">Hi %s, your appointment is tomorrow!</p>
          <p style="color:rgba(255,255,255,0.8);font-size:15px;"><strong style="color:#48ACF0;">%s</strong> · %s · %s – %s</p>
          <p style="color:#8892a4;font-size:14px;">1250 Healthcare Blvd, Suite 400, San Francisco, CA 94105</p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`,
		appt.Patient.FirstName,
		appt.Doctor.Name,
		FormatDateReadable(appt.Slot.Date),
		FormatTimeReadable(appt.Slot.StartTime), FormatTimeReadable(appt.Slot.EndTime),
	)
}

func removeChars(s, chars string) string {
	result := s
	for _, c := range chars {
		result = replaceAll(result, string(c), "")
	}
	return result
}

func replaceAll(s, old, new string) string {
	for {
		n := ""
		for i := 0; i < len(s); {
			if i+len(old) <= len(s) && s[i:i+len(old)] == old {
				n += new
				i += len(old)
			} else {
				n += string(s[i])
				i++
			}
		}
		if n == s {
			break
		}
		s = n
	}
	return s
}
