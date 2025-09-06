# Gemquick

![alt gemquick](https://raw.githubusercontent.com/jimmitjoo/gemquick-bare/main/public/images/gemquick-logo.png)

Gemquick is a modern, full-featured web application framework for Go that provides everything you need to build scalable web applications quickly and securely.

## Features

- 🚀 **Chi Router** - Fast and lightweight HTTP router
- 🗄️ **Multi-Database Support** - PostgreSQL, MySQL, MariaDB, SQLite
- 🔐 **Security First** - CSRF protection, rate limiting, input validation, XSS prevention
- 📧 **Email System** - Multiple provider support with templates
- 💾 **Caching** - Redis and Badger cache implementations
- 🔄 **Background Jobs** - Job queue with cron scheduler
- 🌐 **WebSocket Support** - Real-time communication with hub pattern
- 📁 **File Storage** - S3 and MinIO filesystem integrations
- 📱 **SMS Integration** - Multiple SMS provider support
- 🎨 **Template Engine** - Jet template engine for dynamic views
- 📊 **Logging & Metrics** - Structured logging with health monitoring
- 🔑 **Session Management** - Secure session handling with multiple stores
- 🛠️ **CLI Tools** - Project scaffolding and code generation

## Installation

Clone the repository and build the CLI tool:

```bash
git clone https://github.com/jimmitjoo/gemquick
cd gemquick
make build
```

This will create the `gq` executable in `dist/gq`. You can move this file to your PATH for global access.

## Quick Start

### Create a New Project

```bash
gq new my_project
cd my_project
make start
```

### Project Structure

```
my_project/
├── .env                 # Environment configuration
├── Makefile            # Build and development commands
├── handlers/           # HTTP handlers
├── migrations/         # Database migrations
├── views/              # Template files
├── email/              # Email templates
├── data/               # Models and database logic
├── public/             # Static assets
├── middleware/         # Custom middleware
└── logs/               # Application logs
```

## Development

### Available Make Commands

```bash
make key           # Generate new encryption key
make auth          # Create authentication system with user model
make mail          # Create new email template
make model         # Create new model in data directory
make migration     # Create new database migration
make handler       # Create new HTTP handler
make session       # Create session tables in database
make api-controller    # Create new API controller
make controller    # Create new resource controller
make middleware    # Create new middleware
make docker        # Generate Docker configuration
make deploy        # Generate deployment configuration
```

### Testing

Gemquick includes a beautiful test runner with colored output:

```bash
# Run all tests with colors
make test

# Run tests for specific package
./run-tests -p ./cache/...

# Generate coverage report
make cover

# Run tests without Docker dependencies
./run-tests -s
```

### Database Migrations

```bash
# Run migrations up
gq migrate up

# Roll back migrations
gq migrate down

# Create new migration
make migration name=create_users_table
```

## Core Components

### Web Server Configuration

```go
app := gemquick.Gemquick{}
app.New(rootPath)
app.AppName = "MyApp"
app.Debug = true
```

### Database Connection

Supports multiple databases through environment configuration:

- PostgreSQL
- MySQL/MariaDB  
- SQLite

### Caching

Choose between Redis or Badger cache:

```go
// Redis cache
app.Cache = app.CreateRedisCache()

// Badger cache
app.Cache = app.CreateBadgerCache()
```

### Background Jobs

```go
// Register a job processor
app.JobManager.RegisterProcessor("send-email", emailProcessor)

// Queue a job
app.JobManager.QueueJob("send-email", data)
```

### WebSocket Support

```go
// Initialize WebSocket hub
hub := websocket.NewHub()
go hub.Run()
```

### Security Features

- CSRF protection middleware
- Rate limiting and throttling
- Input validation and sanitization
- SQL injection prevention
- XSS protection
- Session fixation protection
- Secure password hashing

## Configuration

Configuration is managed through `.env` file:

```env
# Application
APP_NAME=MyApp
DEBUG=true
PORT=4000

# Database
DATABASE_TYPE=postgres
DATABASE_DSN=your_connection_string

# Cache
CACHE=redis
REDIS_HOST=localhost:6379

# Session
SESSION_TYPE=redis
SESSION_LIFETIME=24

# Mail
MAIL_PROVIDER=smtp
SMTP_HOST=localhost
SMTP_PORT=1025
```

## API Development

Gemquick includes API utilities for building RESTful services:

- Version management
- Standardized JSON responses
- API middleware
- Error handling

```go
// API versioning
api.Version("v1", func(r chi.Router) {
    r.Get("/users", handlers.GetUsers)
})

// JSON responses
api.JSON(w, http.StatusOK, data)
```

## Testing Philosophy

- Comprehensive test coverage (aim for >80% on critical paths)
- Table-driven tests for better coverage
- Security-focused testing
- Docker-optional test execution
- Colorful test output for better readability

## Contributing

Bug reports and pull requests are welcome on GitHub at the [Gemquick repository](https://github.com/jimmitjoo/gemquick/). This project is intended to be a safe, welcoming space for collaboration. Contributors are expected to adhere to the [Contributor Covenant](https://www.contributor-covenant.org/).

## License

The Gemquick framework is available as open source under the terms of the [MIT License](https://opensource.org/licenses/MIT).

## Documentation

For detailed documentation and examples, see:

- [TESTING.md](TESTING.md) - Complete testing guide
- [CLAUDE.md](CLAUDE.md) - AI assistant integration guide
- [Examples](examples/) - Sample implementations

## Support

For issues, questions, or suggestions, please open an issue on the [GitHub repository](https://github.com/jimmitjoo/gemquick/issues).