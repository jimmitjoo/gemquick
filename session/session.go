package session

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
)

type Session struct {
	CookieLifetime string
	CookiePersist  string
	CookieName     string
	CookieDomain   string
	SessionType    string
	CookieSecure   string
	DBPool         *sql.DB
	RedisPool      *redis.Pool
}

func (g *Session) InitSession() *scs.SessionManager {
	var persist, secure bool

	// how long should sessions last?
	minutes, err := strconv.Atoi(g.CookieLifetime)

	if err != nil {
		minutes = 60
	}

	// should cookies persist?
	if strings.ToLower(g.CookiePersist) == "true" {
		persist = true
	}

	// must cookies be secure?
	if strings.ToLower(g.CookieSecure) == "true" {
		secure = true
	}

	// create session
	session := scs.New()
	session.Lifetime = time.Duration(minutes) * time.Minute
	session.Cookie.Persist = persist
	session.Cookie.Name = g.CookieName
	session.Cookie.Secure = secure
	session.Cookie.Domain = g.CookieDomain
	session.Cookie.SameSite = http.SameSiteLaxMode

	// which session store?
	switch strings.ToLower(g.SessionType) {
	case "redis":
		session.Store = redisstore.New(g.RedisPool)
	case "mysql", "mariadb":
		session.Store = mysqlstore.New(g.DBPool)
	case "postgres", "postgresql":
		session.Store = postgresstore.New(g.DBPool)
	default:
		// cookie
	}

	return session
}
