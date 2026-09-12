package token

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

const versionPrefix = "t:tokver:"

const refreshPrefix = "t:refresh:"

const versionTTL = 7 * 24 * time.Hour

const memTTL = 10 * time.Minute

const jwtAudience = "tandem"

const tokenTypeAccess = "access"

const tokenTypeRefresh = "refresh"

func randomJTI() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type Claims struct {
	Ver int64  `json:"ver"`
	Typ string `json:"typ"`
	JTI string `json:"jti,omitempty"`
	jwt.RegisteredClaims
}

type memVersion struct {
	ver int64
	at  time.Time
}

type JWTManager struct {
	secret []byte
	issuer string
	cache  cache.Cache
	mu     sync.Mutex
	mem    map[string]memVersion
}

func NewJWTManager(secret string, c cache.Cache) *JWTManager {
	return &JWTManager{secret: []byte(secret), issuer: "tandem", cache: c, mem: make(map[string]memVersion)}
}

func (m *JWTManager) Generate(subject string, ttl time.Duration) (string, error) {
	return m.generate(subject, tokenTypeAccess, ttl, "")
}

func (m *JWTManager) GenerateRefresh(subject string, ttl time.Duration) (string, error) {
	if m.cache == nil {
		return m.generate(subject, tokenTypeRefresh, ttl, "")
	}
	jti, err := randomJTI()
	if err != nil {
		return "", err
	}
	if err := m.cache.Set(context.Background(), refreshPrefix+jti, subject, ttl); err != nil {
		return "", err
	}
	return m.generate(subject, tokenTypeRefresh, ttl, jti)
}

func (m *JWTManager) RotateRefresh(ctx context.Context, refreshToken string, accessTTL, refreshTTL time.Duration) (string, string, error) {
	claims, err := m.parse(refreshToken)
	if err != nil || claims.Typ != tokenTypeRefresh || claims.JTI == "" {
		return "", "", pkgerrors.ErrUnauthorized
	}
	if m.cache == nil {
		return m.mintPair(claims.Subject, accessTTL, refreshTTL)
	}
	gone, err := m.cache.GetDel(ctx, refreshPrefix+claims.JTI)
	if err != nil {
		return "", "", err
	}
	if gone != claims.Subject {
		return "", "", pkgerrors.ErrUnauthorized
	}
	return m.mintPair(claims.Subject, accessTTL, refreshTTL)
}

func (m *JWTManager) mintPair(subject string, accessTTL, refreshTTL time.Duration) (string, string, error) {
	access, err := m.generate(subject, tokenTypeAccess, accessTTL, "")
	if err != nil {
		return "", "", err
	}
	refresh, err := m.GenerateRefresh(subject, refreshTTL)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func (m *JWTManager) generate(subject string, typ string, ttl time.Duration, jti string) (string, error) {
	ver, err := m.currentVersion(context.Background(), subject)
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := Claims{
		Ver: ver,
		Typ: typ,
		JTI: jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(jwtAudience),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Subject == "" {
		return nil, errors.New("invalid token")
	}
	if !m.versionValid(context.Background(), claims.Subject, claims.Ver) {
		return nil, errors.New("token revoked")
	}
	return claims, nil
}

func (m *JWTManager) Parse(tokenString string) (string, error) {
	claims, err := m.parse(tokenString)
	if err != nil {
		return "", err
	}
	if claims.Typ != "" && claims.Typ != tokenTypeAccess {
		return "", errors.New("invalid token type")
	}
	return claims.Subject, nil
}

func (m *JWTManager) ParseRefresh(tokenString string) (string, error) {
	claims, err := m.parse(tokenString)
	if err != nil {
		return "", err
	}
	if claims.Typ != tokenTypeRefresh {
		return "", errors.New("invalid token type")
	}
	return claims.Subject, nil
}

func (m *JWTManager) Revoke(ctx context.Context, subject string) error {
	if m.cache == nil {
		return nil
	}
	key := versionPrefix + subject
	current, err := m.cache.Get(ctx, key)
	if err != nil {
		m.memPut(subject, m.memBump(subject))
		return nil
	}
	if current == "" {
		if err := m.cache.Set(ctx, key, "2", versionTTL); err != nil {
			m.memPut(subject, m.memBump(subject))
			return err
		}
		m.memPut(subject, 2)
		return nil
	}
	next, err := m.cache.Incr(ctx, key)
	if err != nil {
		m.memPut(subject, m.memBump(subject))
		return err
	}
	_ = m.cache.Expire(ctx, key, versionTTL)
	m.memPut(subject, next)
	return nil
}

func (m *JWTManager) currentVersion(ctx context.Context, subject string) (int64, error) {
	if m.cache == nil {
		return 1, nil
	}
	key := versionPrefix + subject
	v, err := m.cache.Get(ctx, key)
	if err != nil {
		if memVer, ok := m.memGet(subject); ok {
			return memVer, nil
		}
		return 0, err
	}
	ver := int64(1)
	if v != "" {
		if parsed, perr := strconv.ParseInt(v, 10, 64); perr == nil && parsed >= 1 {
			ver = parsed
		}
	}
	if err := m.cache.Set(ctx, key, strconv.FormatInt(ver, 10), versionTTL); err != nil {
		return 0, err
	}
	m.memPut(subject, ver)
	return ver, nil
}

func (m *JWTManager) versionValid(ctx context.Context, subject string, ver int64) bool {
	if m.cache == nil {
		return true
	}
	v, err := m.cache.Get(ctx, versionPrefix+subject)
	if err != nil {
		if memVer, ok := m.memGet(subject); ok {
			return memVer == ver
		}
		return false
	}
	if v == "" {
		return false
	}
	stored, perr := strconv.ParseInt(v, 10, 64)
	if perr != nil {
		return false
	}
	m.memPut(subject, stored)
	return stored == ver
}

func (m *JWTManager) memGet(subject string) (int64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.mem[subject]
	if !ok || time.Since(e.at) > memTTL {
		return 0, false
	}
	return e.ver, true
}

func (m *JWTManager) memPut(subject string, ver int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mem[subject] = memVersion{ver: ver, at: time.Now()}
}

func (m *JWTManager) memBump(subject string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	ver := int64(1)
	if e, ok := m.mem[subject]; ok && time.Since(e.at) <= memTTL && e.ver >= 1 {
		ver = e.ver
	}
	next := ver + 1
	m.mem[subject] = memVersion{ver: next, at: time.Now()}
	return next
}
