package pkg

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(42, 1)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken() returned empty token")
	}
}

func TestParseToken_Valid(t *testing.T) {
	token, err := GenerateToken(42, 0)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("UserID = %d, want 42", claims.UserID)
	}
	if claims.Role != 0 {
		t.Errorf("Role = %d, want 0", claims.Role)
	}
	if claims.Issuer != "lan-im-server" {
		t.Errorf("Issuer = %s, want lan-im-server", claims.Issuer)
	}
}

func TestParseToken_Expired(t *testing.T) {
	claims := CustomClaims{
		UserID: 1,
		Role:   0,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "lan-im-server",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(jwtSecret())

	_, err := ParseToken(signed)
	if err == nil {
		t.Fatal("ParseToken() expected error for expired token, got nil")
	}
}

func TestParseToken_InvalidSignature(t *testing.T) {
	_, err := ParseToken("invalid.token.here")
	if err == nil {
		t.Fatal("ParseToken() expected error for invalid token, got nil")
	}
}

func TestParseToken_EmptyToken(t *testing.T) {
	_, err := ParseToken("")
	if err == nil {
		t.Fatal("ParseToken() expected error for empty token, got nil")
	}
}

func TestGenerateToken_DifferentUsers(t *testing.T) {
	t1, _ := GenerateToken(1, 0)
	t2, _ := GenerateToken(2, 1)
	if t1 == t2 {
		t.Fatal("tokens for different users should not be equal")
	}

	c1, _ := ParseToken(t1)
	c2, _ := ParseToken(t2)
	if c1.UserID == c2.UserID {
		t.Errorf("UserIDs should differ: got %d == %d", c1.UserID, c2.UserID)
	}
}

func TestCustomClaims_RoleValues(t *testing.T) {
	tests := []struct {
		role int8
	}{
		{role: 0},
		{role: 1},
	}

	for _, tt := range tests {
		token, err := GenerateToken(1, tt.role)
		if err != nil {
			t.Fatalf("GenerateToken(role=%d) error = %v", tt.role, err)
		}
		claims, err := ParseToken(token)
		if err != nil {
			t.Fatalf("ParseToken(role=%d) error = %v", tt.role, err)
		}
		if claims.Role != tt.role {
			t.Errorf("role = %d, want %d", claims.Role, tt.role)
		}
	}
}