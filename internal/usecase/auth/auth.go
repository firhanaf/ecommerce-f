package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"strings"

	"ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"ecommerce-api/pkg/jwt"
	pkgotp "ecommerce-api/pkg/otp"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ─── DTOs ────────────────────────────────────────────────────────────────────

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Phone    string
}

type LoginInput struct {
	Identifier string // email atau nomor HP
	Password   string
}

type AuthResult struct {
	User         *domain.User
	AccessToken  string
	RefreshToken string
}

type RegisterResult struct {
	UserID  uuid.UUID
	Message string
}

// ─── Errors ──────────────────────────────────────────────────────────────────

const maxOTPAttempts = 5

var (
	ErrEmailAlreadyExists    = errors.New("email already registered")
	ErrPhoneAlreadyExists    = errors.New("phone number already registered")
	ErrInvalidCredentials    = errors.New("invalid email/phone or password")
	ErrUserInactive          = errors.New("user account is inactive")
	ErrPhoneNotVerified      = errors.New("phone number not verified")
	ErrInvalidOTP            = errors.New("invalid or expired OTP code")
	ErrOTPMaxAttempts        = errors.New("OTP has been invalidated due to too many failed attempts")
	ErrOTPRateLimited        = errors.New("too many OTP requests, please wait before retrying")
	ErrPhoneRequired         = errors.New("phone number is required for verification")
	ErrUserNotFound          = errors.New("user not found")
	ErrPasswordTooShort      = errors.New("password must be at least 8 characters")
)

// ─── Interface ───────────────────────────────────────────────────────────────

type UseCase interface {
	Register(ctx context.Context, input RegisterInput) (*RegisterResult, error)
	Login(ctx context.Context, input LoginInput) (*AuthResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (*AuthResult, error)
	VerifyOTP(ctx context.Context, userID uuid.UUID, code string) (*AuthResult, error)
	ResendOTP(ctx context.Context, userID uuid.UUID) error
	ForgotPassword(ctx context.Context, identifier string) (userID uuid.UUID, err error)
	ResetPassword(ctx context.Context, userID uuid.UUID, code, newPassword string) error
}

// ─── Implementation ──────────────────────────────────────────────────────────

type useCase struct {
	userRepo  repository.UserRepository
	otpRepo   repository.OTPRepository
	tokenSvc  jwt.TokenService
	otpSender pkgotp.Sender
	auditRepo repository.AuditLogRepository
}

func NewUseCase(
	userRepo repository.UserRepository,
	otpRepo repository.OTPRepository,
	tokenSvc jwt.TokenService,
	otpSender pkgotp.Sender,
	auditRepo repository.AuditLogRepository,
) UseCase {
	return &useCase{
		userRepo:  userRepo,
		otpRepo:   otpRepo,
		tokenSvc:  tokenSvc,
		otpSender: otpSender,
		auditRepo: auditRepo,
	}
}

// Register membuat user baru dan langsung kirim OTP ke WA
// Tidak return token — user harus verifikasi OTP dulu
func (uc *useCase) Register(ctx context.Context, input RegisterInput) (*RegisterResult, error) {
	if input.Phone == "" {
		return nil, ErrPhoneRequired
	}

	// Normalisasi phone ke format 62xxx sebelum disimpan
	normalizedPhone := pkgotp.NormalizePhone(input.Phone)

	// Cek duplikat email
	existing, _ := uc.userRepo.FindByEmail(ctx, input.Email)
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	// Cek duplikat phone
	existingPhone, _ := uc.userRepo.FindByPhone(ctx, normalizedPhone)
	if existingPhone != nil {
		return nil, ErrPhoneAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &domain.User{
		ID:            uuid.New(),
		Name:          input.Name,
		Email:         input.Email,
		PasswordHash:  string(hash),
		Role:          domain.RoleBuyer,
		Phone:         normalizedPhone,
		IsActive:      true,
		PhoneVerified: false, // blocked sampai OTP verified
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &user.ID,
		ActorRole:  string(user.Role),
		Action:     domain.AuditCreate,
		EntityType: "users",
		EntityID:   &user.ID,
		NewData:    map[string]any{"email": user.Email, "name": user.Name},
		CreatedAt:  now,
	})

	return &RegisterResult{
		UserID:  user.ID,
		Message: "OTP telah dikirim ke nomor WhatsApp Anda",
	}, nil
}

// Login blocked jika phone belum diverifikasi
func (uc *useCase) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	var user *domain.User
	var err error

	if strings.Contains(input.Identifier, "@") {
		user, err = uc.userRepo.FindByEmail(ctx, input.Identifier)
	} else {
		user, err = uc.userRepo.FindByPhone(ctx, pkgotp.NormalizePhone(input.Identifier))
	}

	if err != nil || user == nil {
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Blocked kalau nomor HP belum diverifikasi
	if !user.PhoneVerified {
		return nil, fmt.Errorf("%w:%s", ErrPhoneNotVerified, user.ID.String())
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &user.ID,
		ActorRole:  string(user.Role),
		Action:     domain.AuditLogin,
		EntityType: "users",
		EntityID:   &user.ID,
		CreatedAt:  time.Now(),
	})

	return uc.buildAuthResult(user)
}

func (uc *useCase) RefreshToken(ctx context.Context, refreshToken string) (*AuthResult, error) {
	claims, err := uc.tokenSvc.ValidateToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	user, err := uc.userRepo.FindByID(ctx, claims.UserID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}

	return uc.buildAuthResult(user)
}

// VerifyOTP validasi kode OTP, jika benar set phone_verified = true dan return tokens
func (uc *useCase) VerifyOTP(ctx context.Context, userID uuid.UUID, code string) (*AuthResult, error) {
	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}

	if user.PhoneVerified {
		// Sudah verified — langsung return tokens
		return uc.buildAuthResult(user)
	}

	// Ambil OTP terbaru yang belum dipakai
	otp, err := uc.otpRepo.FindLatest(ctx, userID, domain.OTPTypePhoneVerification)
	if err != nil {
		return nil, err
	}
	if otp == nil {
		return nil, ErrInvalidOTP
	}

	// Cek expired
	if time.Now().After(otp.ExpiresAt) {
		return nil, ErrInvalidOTP
	}

	// Cek apakah sudah melebihi max attempts
	if otp.Attempts >= maxOTPAttempts {
		return nil, ErrOTPMaxAttempts
	}

	// Cek kode — salah: increment attempts, kalau sudah mentok invalidate OTP
	if otp.Code != code {
		_ = uc.otpRepo.IncrementAttempts(ctx, otp.ID)
		if otp.Attempts+1 >= maxOTPAttempts {
			_ = uc.otpRepo.MarkUsed(ctx, otp.ID)
			return nil, ErrOTPMaxAttempts
		}
		return nil, ErrInvalidOTP
	}

	// Tandai OTP sudah dipakai
	if err := uc.otpRepo.MarkUsed(ctx, otp.ID); err != nil {
		return nil, err
	}

	// Set phone_verified = true
	if err := uc.userRepo.UpdatePhoneVerified(ctx, userID); err != nil {
		return nil, err
	}
	user.PhoneVerified = true

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &user.ID,
		ActorRole:  string(user.Role),
		Action:     domain.AuditUpdate,
		EntityType: "users",
		EntityID:   &user.ID,
		NewData:    map[string]any{"phone_verified": true},
		CreatedAt:  time.Now(),
	})

	return uc.buildAuthResult(user)
}

// ResendOTP kirim ulang OTP dengan rate limiting: max 3x per 10 menit
func (uc *useCase) ResendOTP(ctx context.Context, userID uuid.UUID) error {
	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	if user.PhoneVerified {
		return errors.New("phone already verified")
	}

	// Rate limit: max 3 OTP dalam 10 menit terakhir
	since := time.Now().Add(-10 * time.Minute)
	count, err := uc.otpRepo.CountRecent(ctx, userID, domain.OTPTypePhoneVerification, since)
	if err != nil {
		return err
	}
	if count >= 3 {
		return ErrOTPRateLimited
	}

	return uc.sendOTP(ctx, user.ID, user.Phone, domain.OTPTypePhoneVerification)
}

// ForgotPassword mencari user berdasarkan email/phone, lalu kirim OTP reset password.
// Rate limit: maks 1 OTP reset_password per 10 menit per user.
func (uc *useCase) ForgotPassword(ctx context.Context, identifier string) (uuid.UUID, error) {
	var user *domain.User
	var err error

	if strings.Contains(identifier, "@") {
		user, err = uc.userRepo.FindByEmail(ctx, identifier)
	} else {
		user, err = uc.userRepo.FindByPhone(ctx, pkgotp.NormalizePhone(identifier))
	}
	if err != nil || user == nil {
		return uuid.Nil, ErrUserNotFound
	}
	if !user.IsActive {
		return uuid.Nil, ErrUserInactive
	}

	since := time.Now().Add(-10 * time.Minute)
	count, err := uc.otpRepo.CountRecent(ctx, user.ID, domain.OTPTypeResetPassword, since)
	if err != nil {
		return uuid.Nil, err
	}
	if count >= 1 {
		return uuid.Nil, ErrOTPRateLimited
	}

	if err := uc.sendOTP(ctx, user.ID, user.Phone, domain.OTPTypeResetPassword); err != nil {
		return uuid.Nil, err
	}

	return user.ID, nil
}

// ResetPassword memvalidasi OTP reset password lalu update password hash.
func (uc *useCase) ResetPassword(ctx context.Context, userID uuid.UUID, code, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}

	otp, err := uc.otpRepo.FindLatest(ctx, userID, domain.OTPTypeResetPassword)
	if err != nil {
		return err
	}
	if otp == nil || time.Now().After(otp.ExpiresAt) {
		return ErrInvalidOTP
	}
	if otp.Attempts >= maxOTPAttempts {
		return ErrOTPMaxAttempts
	}
	if otp.Code != code {
		_ = uc.otpRepo.IncrementAttempts(ctx, otp.ID)
		if otp.Attempts+1 >= maxOTPAttempts {
			_ = uc.otpRepo.MarkUsed(ctx, otp.ID)
			return ErrOTPMaxAttempts
		}
		return ErrInvalidOTP
	}

	if err := uc.otpRepo.MarkUsed(ctx, otp.ID); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := uc.userRepo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return err
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &userID,
		Action:     domain.AuditUpdate,
		EntityType: "users",
		EntityID:   &userID,
		NewData:    map[string]any{"password_reset": true},
		CreatedAt:  time.Now(),
	})

	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (uc *useCase) sendOTP(ctx context.Context, userID uuid.UUID, phone, otpType string) error {
	code, err := generateOTPCode()
	if err != nil {
		return fmt.Errorf("generate otp code: %w", err)
	}

	now := time.Now()
	otp := &domain.OTPToken{
		ID:        uuid.New(),
		UserID:    userID,
		Code:      code,
		Type:      otpType,
		ExpiresAt: now.Add(5 * time.Minute),
		CreatedAt: now,
	}

	if err := uc.otpRepo.Create(ctx, otp); err != nil {
		return fmt.Errorf("save otp: %w", err)
	}

	if err := uc.otpSender.SendOTP(ctx, phone, code); err != nil {
		return fmt.Errorf("send otp: %w", err)
	}

	return nil
}

func (uc *useCase) buildAuthResult(user *domain.User) (*AuthResult, error) {
	accessToken, err := uc.tokenSvc.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		return nil, err
	}

	refreshToken, err := uc.tokenSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateOTPCode menghasilkan kode OTP 6 digit menggunakan crypto/rand (aman)
func generateOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
