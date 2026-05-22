package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agro-cost-app/pkg/models"
	"agro-cost-app/pkg/repository"
)

type AuthService struct {
	Repo          *repository.Repository
	SessionSecret string
}

func NewAuthService(repo *repository.Repository, secret string) *AuthService {
	return &AuthService{Repo: repo, SessionSecret: secret}
}

func (s *AuthService) Register(ctx context.Context, fullName, email, password string) (models.User, error) {
	fullName = strings.TrimSpace(fullName)
	email = strings.ToLower(strings.TrimSpace(email))
	if fullName == "" {
		return models.User{}, fmt.Errorf("укажите ФИО")
	}
	if !strings.Contains(email, "@") {
		return models.User{}, fmt.Errorf("укажите корректный email")
	}
	if len(password) < 6 {
		return models.User{}, fmt.Errorf("пароль должен содержать не менее 6 символов")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return models.User{}, err
	}
	return s.Repo.CreateUser(ctx, fullName, email, hash)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.Repo.UserByEmail(ctx, email)
	if err != nil {
		return models.User{}, err
	}
	if user.ID == 0 || !checkPassword(password, user.PasswordHash) {
		return models.User{}, fmt.Errorf("неверный email или пароль")
	}
	return user, nil
}

func (s *AuthService) SetSessionCookie(w http.ResponseWriter, userID int64) {
	payload := strconv.FormatInt(userID, 10)
	sig := s.sign(payload)
	http.SetCookie(w, &http.Cookie{Name: "agro_session", Value: payload + "." + sig, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(14 * 24 * time.Hour)})
}

func (s *AuthService) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "agro_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Unix(0, 0), MaxAge: -1})
}

func (s *AuthService) CurrentUser(ctx context.Context, r *http.Request) (models.User, bool) {
	c, err := r.Cookie("agro_session")
	if err != nil || c.Value == "" {
		return models.User{}, false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(s.sign(parts[0]))) {
		return models.User{}, false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return models.User{}, false
	}
	user, err := s.Repo.UserByID(ctx, id)
	return user, err == nil && user.ID > 0
}

func (s *AuthService) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.SessionSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return base64.RawStdEncoding.EncodeToString(salt) + "$" + hex.EncodeToString(sum[:]), nil
}

func checkPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return hmac.Equal([]byte(parts[1]), []byte(hex.EncodeToString(sum[:])))
}
