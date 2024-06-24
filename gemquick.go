package gemquick

import (
	"database/sql"
	"fmt"
	"github.com/jimmitjoo/gemquick/filesystems/miniofilesystem"
	"github.com/jimmitjoo/gemquick/filesystems/s3filesystem"
	"github.com/jimmitjoo/gemquick/sms"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CloudyKit/jet/v6"
	"github.com/alexedwards/scs/v2"
	"github.com/dgraph-io/badger/v3"
	"github.com/go-chi/chi/v5"
	"github.com/gomodule/redigo/redis"
	"github.com/jimmitjoo/gemquick/cache"
	"github.com/jimmitjoo/gemquick/email"
	"github.com/jimmitjoo/gemquick/render"
	"github.com/jimmitjoo/gemquick/session"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

const version = "0.0.1"

var myRedisCache *cache.RedisCache
var myBadgerCache *cache.BadgerCache
var redisPool *redis.Pool
var badgerConn *badger.DB

type Gemquick struct {
	AppName       string
	Debug         bool
	Version       string
	ErrorLog      *log.Logger
	InfoLog       *log.Logger
	RootPath      string
	Routes        *chi.Mux
	Render        *render.Render
	Session       *scs.SessionManager
	DB            Database
	JetViews      *jet.Set
	config        config
	EncryptionKey string
	Cache         cache.Cache
	Scheduler     *cron.Cron
	SMSProvider   sms.SMSProvider
	Mail          email.Mail
	Server        Server
	FileSystems   map[string]interface{}
}

type Server struct {
	ServerName string
	Port       string
	Secure     bool
	URL        string
}

type config struct {
	port        string
	renderer    string
	cookie      cookieConfig
	sessionType string
	database    databaseConfig
	redis       redisConfig
}

func (g *Gemquick) New(rootPath string) error {
	pathConfig := initPaths{
		rootPath:    rootPath,
		folderNames: []string{"handlers", "migrations", "views", "email", "data", "public", "tmp", "logs", "middleware"},
	}

	err := g.Init(pathConfig)

	if err != nil {
		return err
	}

	err = g.checkDotEnv(rootPath)

	if err != nil {
		return err
	}

	// read .env
	err = godotenv.Load(rootPath + "/.env")

	if err != nil {
		return err
	}

	// create loggers
	infoLog, errorLog := g.startLoggers()

	// connect to database
	if os.Getenv("DATABASE_TYPE") != "" {
		db, err := g.OpenDB(os.Getenv("DATABASE_TYPE"), g.BuildDSN())

		if err != nil {
			errorLog.Println(err)
			os.Exit(1)
		}

		g.DB = Database{
			DataType:    os.Getenv("DATABASE_TYPE"),
			Pool:        db,
			TablePrefix: os.Getenv("DATABASE_TABLE_PREFIX"),
		}
	}

	scheduler := cron.New()
	g.Scheduler = scheduler

	// connect to redis
	if os.Getenv("CACHE") == "redis" || os.Getenv("SESSION_TYPE") == "redis" {
		myRedisCache = g.createClientRedisCache()
		g.Cache = myRedisCache

		redisPool = myRedisCache.Conn
	}

	// connect to badger
	if os.Getenv("CACHE") == "badger" || os.Getenv("SESSION_TYPE") == "badger" {
		myBadgerCache = g.createClientBadgerCache()
		g.Cache = myBadgerCache

		badgerConn = myBadgerCache.Conn

		// start badger garbage collector
		_, err := g.Scheduler.AddFunc("@daily", func() {
			_ = myBadgerCache.Conn.RunValueLogGC(0.7)
		})
		if err != nil {
			return err
		}
	}

	g.InfoLog = infoLog
	g.ErrorLog = errorLog
	g.Debug, _ = strconv.ParseBool(os.Getenv("DEBUG"))
	g.Version = version
	g.RootPath = rootPath
	g.Routes = g.routes().(*chi.Mux)

	g.config = config{
		port:     os.Getenv("PORT"),
		renderer: os.Getenv("RENDERER"),
		cookie: cookieConfig{
			name:     os.Getenv("COOKIE_NAME"),
			lifetime: os.Getenv("COOKIE_LIFETIME"),
			persist:  os.Getenv("COOKIE_PERSISTS"),
			secure:   os.Getenv("COOKIE_SECURE"),
			domain:   os.Getenv("COOKIE_DOMAIN"),
		},
		sessionType: os.Getenv("SESSION_TYPE"),
		database: databaseConfig{
			database: os.Getenv("DATABASE_TYPE"),
			dsn:      g.BuildDSN(),
		},
		redis: redisConfig{
			host:     os.Getenv("REDIS_HOST"),
			port:     os.Getenv("REDIS_PORT"),
			password: os.Getenv("REDIS_PASSWORD"),
			prefix:   os.Getenv("REDIS_PREFIX"),
		},
	}

	secure := true
	if strings.ToLower(os.Getenv("SECURE")) == "false" {
		secure = false
	}

	g.Server = Server{
		ServerName: os.Getenv("SERVER_NAME"),
		Port:       os.Getenv("PORT"),
		Secure:     secure,
		URL:        os.Getenv("APP_URL"),
	}

	// create a session
	sess := session.Session{
		CookieLifetime: g.config.cookie.lifetime,
		CookiePersist:  g.config.cookie.persist,
		CookieName:     g.config.cookie.name,
		SessionType:    g.config.sessionType,
		CookieDomain:   g.config.cookie.domain,
		DBPool:         g.DB.Pool,
	}

	switch g.config.sessionType {
	case "redis":
		sess.RedisPool = myRedisCache.Conn
	case "mysql", "postgres", "mariadb", "postgresql", "pgx":
		sess.DBPool = g.DB.Pool
	}

	g.Session = sess.InitSession()
	g.EncryptionKey = os.Getenv("KEY")

	var views *jet.Set
	if g.Debug {
		views = jet.NewSet(
			jet.NewOSFileSystemLoader(fmt.Sprintf("%s/views", rootPath)),

			jet.InDevelopmentMode(),
		)
	} else {
		views = jet.NewSet(
			jet.NewOSFileSystemLoader(fmt.Sprintf("%s/views", rootPath)),
		)
	}

	g.JetViews = views

	g.createRenderer()

	g.FileSystems = g.createFileSystems()

	g.SMSProvider = sms.CreateSMSProvider(os.Getenv("SMS_PROVIDER"))

	g.Mail = g.createMailer()

	go g.Mail.ListenForMail()

	return nil
}

func (g *Gemquick) Init(p initPaths) error {
	root := p.rootPath
	for _, path := range p.folderNames {
		// create folder if it doesnt exist
		err := g.CreateDirIfNotExists(root + "/" + path)

		if err != nil {
			return err
		}
	}

	return nil
}

// ListenAndServe starts the web server
func (g *Gemquick) ListenAndServe() {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", os.Getenv("PORT")),
		ErrorLog:     g.ErrorLog,
		Handler:      g.Routes,
		IdleTimeout:  30 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 600 * time.Second,
	}

	if g.DB.Pool != nil {
		defer func(Pool *sql.DB) {
			err := Pool.Close()
			if err != nil {
				g.ErrorLog.Println(err)
			}
		}(g.DB.Pool)
	}

	if redisPool != nil {
		defer func(redisPool *redis.Pool) {
			err := redisPool.Close()
			if err != nil {
				g.ErrorLog.Println(err)
			}
		}(redisPool)
	}

	if badgerConn != nil {
		defer func(badgerConn *badger.DB) {
			err := badgerConn.Close()
			if err != nil {
				g.ErrorLog.Println(err)
			}
		}(badgerConn)
	}

	g.InfoLog.Printf("Listening on port %s", os.Getenv("PORT"))
	err := srv.ListenAndServe()
	g.ErrorLog.Fatal(err)
}

func (g *Gemquick) checkDotEnv(path string) error {
	err := g.CreateFileIfNotExists(fmt.Sprintf("%s/.env", path))

	if err != nil {
		return err
	}

	return nil
}

func (g *Gemquick) startLoggers() (*log.Logger, *log.Logger) {
	var infoLog *log.Logger
	var errorLog *log.Logger

	infoLog = log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog = log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	return infoLog, errorLog
}

func (g *Gemquick) createRenderer() {
	myRenderer := render.Render{
		Renderer: g.config.renderer,
		RootPath: g.RootPath,
		Port:     g.config.port,
		JetViews: g.JetViews,
		Session:  g.Session,
	}

	g.Render = &myRenderer
}

func (g *Gemquick) createMailer() email.Mail {
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	m := email.Mail{
		Templates: g.RootPath + "/email",

		Host:       os.Getenv("SMTP_HOST"),
		Username:   os.Getenv("SMTP_USERNAME"),
		Password:   os.Getenv("SMTP_PASSWORD"),
		Encryption: os.Getenv("SMTP_ENCRYPTION"),
		Port:       port,

		Domain:   os.Getenv("MAIL_DOMAIN"),
		From:     os.Getenv("MAIL_FROM_ADDRESS"),
		FromName: os.Getenv("MAIL_FROM_NAME"),

		Jobs:    make(chan email.Message, 20),
		Results: make(chan email.Result, 20),

		API:    os.Getenv("MAILER_API"),
		APIKey: os.Getenv("MAILER_KEY"),
		APIUrl: os.Getenv("MAILER_URL"),
	}
	return m
}

func (g *Gemquick) createClientRedisCache() *cache.RedisCache {
	cacheClient := cache.RedisCache{
		Conn:   g.createRedisPool(),
		Prefix: g.config.redis.prefix,
	}
	return &cacheClient
}

func (g *Gemquick) createClientBadgerCache() *cache.BadgerCache {
	cacheClient := cache.BadgerCache{
		Conn: g.createBadgerConn(),
	}
	return &cacheClient
}

func (g *Gemquick) createRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		MaxActive:   10000,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")))

			if err != nil {
				return nil, err
			}

			if os.Getenv("REDIS_PASSWORD") != "" {
				if _, err := c.Do("AUTH", os.Getenv("REDIS_PASSWORD")); err != nil {
					closeError := c.Close()
					if closeError != nil {
						return nil, closeError
					}
					return nil, err
				}
			}

			return c, err
		},

		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func (g *Gemquick) createBadgerConn() *badger.DB {
	db, err := badger.Open(badger.DefaultOptions(fmt.Sprintf("%s/tmp/badger", g.RootPath)))
	if err != nil {
		g.ErrorLog.Fatal(err)
	}

	return db
}

func (g *Gemquick) BuildDSN() string {
	var dsn string

	switch os.Getenv("DATABASE_TYPE") {
	case "postgres", "postgresql":
		dsn = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s timezone=UTC connect_timeout=5",
			os.Getenv("DATABASE_HOST"),
			os.Getenv("DATABASE_PORT"),
			os.Getenv("DATABASE_USER"),
			os.Getenv("DATABASE_NAME"),
			os.Getenv("DATABASE_SSL_MODE"))

		if os.Getenv("DATABASE_PASS") != "" {
			dsn = fmt.Sprintf("%s password=%s", dsn, os.Getenv("DATABASE_PASS"))
		}

	default:
	}

	return dsn
}

func (g *Gemquick) createFileSystems() map[string]interface{} {
	fileSystems := make(map[string]interface{})

	if os.Getenv("MINIO_SECRET") != "" {

		useSSL := false
		if os.Getenv("MINIO_USE_SSL") == "true" {
			useSSL = true
		}

		minio := miniofilesystem.Minio{
			Endpoint:  os.Getenv("MINIO_ENDPOINT"),
			AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
			SecretKey: os.Getenv("MINIO_SECRET"),
			UseSSL:    useSSL,
			Region:    os.Getenv("MINIO_REGION"),
			Bucket:    os.Getenv("MINIO_BUCKET"),
		}

		fileSystems["minio"] = minio
	}

	if os.Getenv("S3_BUCKET") != "" {
		s3 := s3filesystem.S3{
			Key:      os.Getenv("S3_KEY"),
			Secret:   os.Getenv("S3_SECRET"),
			Region:   os.Getenv("S3_REGION"),
			Endpoint: os.Getenv("S3_ENDPOINT"),
			Bucket:   os.Getenv("S3_BUCKET"),
		}

		fileSystems["s3"] = s3
	}

	return fileSystems
}
