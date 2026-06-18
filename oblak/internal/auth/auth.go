package auth

import (
    "errors"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/crypto/bcrypt"
)

var secretKey []byte

func Init() {
    key := os.Getenv("SECRET_KEY")
    if key == "" {
        panic("SECRET_KEY env varijabla nije postavljena")
    }
    secretKey = []byte(key)
}

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func CheckPassword(password, hash string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type Claims struct {
    Username string `json:"username"`
    jwt.RegisteredClaims
}

func CreateToken(username string) (string, error) {
    expireMinutes := 60
    claims := Claims{
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireMinutes) * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(secretKey)
}

func ParseToken(tokenStr string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, errors.New("neočekivana metoda potpisivanja")
        }
        return secretKey, nil
    })
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, errors.New("nevalidan token")
    }
    return claims, nil
}

// Gin middleware za autentikaciju
func Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        header := c.GetHeader("Authorization")
        if header == "" || !strings.HasPrefix(header, "Bearer ") {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Token nije prosleđen"})
            return
        }

        tokenStr := strings.TrimPrefix(header, "Bearer ")
        claims, err := ParseToken(tokenStr)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Nevalidan token"})
            return
        }

        c.Set("username", claims.Username)
        c.Next()
    }
}

func GetUsername(c *gin.Context) string {
    username, _ := c.Get("username")
    return username.(string)
}