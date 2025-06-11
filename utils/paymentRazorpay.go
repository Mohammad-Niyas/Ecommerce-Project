	package utils

	import (
		"crypto/hmac"
		"crypto/sha256"
		"ecommerce/models"
		"encoding/base64"
		"encoding/hex"
		"encoding/json"
		"errors"
		"strings"
	)

	type PaymentData struct {
		Address          models.Address
		ExpectedDelivery string
		Subtotal         float64
		Discount         float64
		Tax              float64
		Delivery         float64
		Total            float64
		ItemCount        int
	}

	func SignPaymentData(paymentData PaymentData, secretKey string) (string, error) {
		data, err := json.Marshal(paymentData)
		if err != nil {
			return "", err
		}

		encodedData := base64.StdEncoding.EncodeToString(data)
		h := hmac.New(sha256.New, []byte(secretKey))
		h.Write([]byte(encodedData))
		signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

		return encodedData + "." + signature, nil
	}

	func VerifyAndDecodePaymentData(token string, secretKey string) (PaymentData, error) {
		var paymentData PaymentData

		parts := strings.Split(token, ".")
		if len(parts) != 2 {
			return paymentData, errors.New("invalid token format")
		}

		encodedData, signature := parts[0], parts[1]
		h := hmac.New(sha256.New, []byte(secretKey))
		h.Write([]byte(encodedData))
		expectedSignature := base64.StdEncoding.EncodeToString(h.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			return paymentData, errors.New("invalid signature")
		}

		data, err := base64.StdEncoding.DecodeString(encodedData)
		if err != nil {
			return paymentData, err
		}

		if err := json.Unmarshal(data, &paymentData); err != nil {
			return paymentData, err
		}

		return paymentData, nil
	}

	func HmacSha256(data string, secret string) string {
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(data))
		return hex.EncodeToString(h.Sum(nil))
	}