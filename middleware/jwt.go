package middleware

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

type Claims struct {
	Email  string `json:"username"`
	Role   string `json:"role"`
	UserId uint   `json:"userId"`
	jwt.StandardClaims
}

var jwtKey = []byte(os.Getenv("SECRETKEY"))

func JwtToken(c *gin.Context, userId uint, email string, role string) {
	tokenString, err := GenerateToken(userId, email, role)
	if err != nil {
		c.JSON(400, gin.H{
			"status": "Fail",
			"error":  "Failed to generate JWT token",
			"code":   400,
		})
		return
	}

	c.JSON(200, gin.H{
		"status": "Success",
		"code":   200,
		"token":  tokenString,
	})
}

func ValidateAdminToken(c *gin.Context) bool {
	tokenString, err := c.Cookie("jwtTokensAdmin")
	if err != nil || tokenString == "" {
		return false
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	return err == nil && token.Valid && claims.Role == "Admin"
}

func ValidateUserToken(c *gin.Context) bool {
	tokenString, err := c.Cookie("jwtTokensUser")
	if err != nil || tokenString == "" {
		return false
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	return err == nil && token.Valid && claims.Role == "User"
}

func GenerateToken(userId uint, email string, role string) (string, error) {
	claims := Claims{
		Email:  email,
		Role:   role,
		UserId: userId,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 2).Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func GetJwtKey() []byte {
	return jwtKey
}

func AuthMiddleware(requiredRole string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenString, err := c.Cookie("jwtTokens" + requiredRole)
        fmt.Println("TokenString:", tokenString)

        if err != nil || tokenString == "" {
            fmt.Println("No token found in cookie")
            c.JSON(http.StatusUnauthorized, gin.H{
                "status":  "Fail",
                "message": "Please log in to access this resource",
                "code":    401,
            })
            c.Abort()
            return
        }

        claims := &Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return jwtKey, nil
        })
        if err != nil || !token.Valid {
            fmt.Println("Token validation failed:", err)
            c.JSON(http.StatusUnauthorized, gin.H{
                "status":  "Fail",
                "message": "Invalid or expired token",
                "code":    401,
            })
            c.Abort()
            return
        }

        if claims.Role != requiredRole {
            fmt.Println("Role mismatch - Required:", requiredRole, "Found:", claims.Role)
            c.JSON(http.StatusForbidden, gin.H{
                "status": "Fail",
                "message": "Insufficient permissions",
                "code":   403,
            })
            c.Abort()
            return
        }

        c.Set("userid", claims.UserId)
        fmt.Println("UserID set in context:", claims.UserId)
        c.Next()
    }
}