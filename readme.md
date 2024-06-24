# Gemquick

![alt gemquick](https://raw.githubusercontent.com/jimmitjoo/gemquick-bare/main/public/images/gemquick-logo.png)

## Installation

Clone the repository and run `make build`. This will build the gem executable in `dist/gq`. You can move this file to wherever you like on your system and run it from there. From now on we will refer to this file as `gq`.

## Usage

Run `gq help` to see the help menu. This will show you all the available commands and options.

## Development

To start developing a new project with Gemquick, run `gq new my_project`. This will create a new directory called `my_project` with the necessary files to start developing a new gem.

```
gq new my_project
cd my_project
make start
```

### Functionality

Gemquick is a framework for building web applications. It provides a set of tools to help you build your application in Golang with some stuff out of the box. For example, it comes with a built-in web server, a router, an authentication system, a mail engine, config for SMS providers, a few filesystems to choose from, a template engine, and a database connection just to name a few.

```
make auth # Create an authentication system with a user model
make mail # Create a new email in the email directory
make model # Create a new model in the data directory
make migration # Create a new migration in the migrations directory
make handler # Create a new handler in the handlers directory
make session # Create a new table in the database for sessions

```

## Contributing

Bug reports and pull requests are welcome on GitHub at the [Gemquick repository](https://github.com/jimmitjoo/gemquick/). This project is intended to be a safe, welcoming space for collaboration. Contributors are expected to adhere to the [Contributor Covenant](https://www.contributor-covenant.org/).

## License

The Gemquick framework is available as open source under the terms of the [MIT License](https://opensource.org/licenses/MIT).