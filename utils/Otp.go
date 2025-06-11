package utils

import (
	"bytes"
	"crypto/rand"
	"ecommerce/config"
	"ecommerce/models"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"time"
)

func GenerateOTP(length int) (string, error) {
	const charset = "0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

func StoreOTP(email, otp string, expireDuration time.Duration) error {
	expireTime := time.Now().Add(expireDuration)
	otpRecord := models.Otp{
		Email:      email,
		Otp:        otp,
		ExpireTime: expireTime,
	}
	return config.DB.Create(&otpRecord).Error
}

func SendOTPEmail(email, otp string) error {
	url := "https://api.brevo.com/v3/smtp/email"

	apiKey := os.Getenv("BREVO_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("BREVO_API_KEY environment variable is not set")
	}

	payload := map[string]interface{}{
		"sender": map[string]string{
			"name":  "Vogue Luxe",
			"email": "vogueluxe@outlook.com",
		},
		"to": []map[string]string{
			{
				"email": email,
				"name":  email,
			},
		},
		"subject": "Your OTP for VogueLuxe Signup",
		"htmlContent": fmt.Sprintf(`
			<!DOCTYPE html>
			<html>
			<head>
				<title>Your OTP for VogueLuxe Signup</title>
			</head>
			<body>
				<h1>Welcome to VogueLuxe!</h1>
				<p>Your OTP is <strong>%s</strong>. It expires in 5 minutes.</p>
				<p>If you didn't request this, please ignore this email.</p>
				<p>Best regards,<br>The VogueLuxe Team</p>
			</body>
			</html>
		`, otp),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send OTP email via Brevo: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Brevo API Response Status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to send OTP email, status code: %d, unable to read response body: %v", resp.StatusCode, readErr)
		}
		return fmt.Errorf("failed to send OTP email, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Successfully sent OTP email to %s via Brevo API. Response: %s\n", email, string(body))
	return nil
}

func IsNumeric(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}