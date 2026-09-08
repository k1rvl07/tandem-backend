package token

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tandem/tandem/internal/domain/ports/cache"
)

const versionPrefix = "t:tokver:"

const versionTTL = 7 * 24 * time.Hour

const memTTL = 10 * time.Minute

const jwtAudience = "tandem"

type Claims struct {
	Ver int64 `json:"ver"`
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
	ver, err := m.currentVersion(context.Background(), subject)
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := Claims{
		Ver: ver,
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

func (m *JWTManager) Parse(tokenString string) (string, error) {
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
		return "", err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}
	if claims.Subject == "" {
		return "", errors.New("invalid token")
	}
	if !m.versionValid(context.Background(), claims.Subject, claims.Ver) {
		return "", errors.New("token revoked")
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
