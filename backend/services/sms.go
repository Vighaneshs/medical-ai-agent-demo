package services

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"kyron-medical/models"
)

func SendConfirmationSMS(appt *models.Appointment) {
	msg := fmt.Sprintf(
		"Kyron Medical: Appt confirmed! Dr. %s on %s at %s. Location: 1250 Healthcare Blvd, SF. Confirmation #%s. Reply STOP to unsubscribe.",
		appt.Doctor.Name,
		FormatDateReadable(appt.Slot.Date),
		appt.Slot.StartTime,
		appt.ID[:8],
	)
	sendSMS(appt.Patient.Phone, msg)
}

func SendReminderSMS(appt *models.Appointment) {
	msg := fmt.Sprintf(
		"Kyron Medical reminder: Your appt with Dr. %s is tomorrow at %s. 1250 Healthcare Blvd, SF. See you then!",
		appt.Doctor.Name,
		appt.Slot.StartTime,
	)
	sendSMS(appt.Patient.Phone, msg)
}

func sendSMS(to, body string) {
	sid := os.Getenv("TWILIO_ACCOUNT_SID")
	token := os.Getenv("TWILIO_AUTH_TOKEN")
	from := os.Getenv("TWILIO_PHONE_NUMBER")

	if sid == "" || token == "" || from == "" {
		log.Printf("SMS skipped (no credentials): to=%s", to)
		return
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", sid)
	data := url.Values{
		"From": {from},
		"To":   {to},
		"Body": {body},
	}

	req, _ := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	req.SetBasicAuth(sid, token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("SMS send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("SMS send failed: status %d", resp.StatusCode)
	} else {
		log.Printf("SMS sent: to=%s", to)
	}
}
