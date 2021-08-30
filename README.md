# tg-resize-sticker-images
A tiny, privacy-preserving bot that converts and compresses images so that they can be turned into Telegram stickers. Reachable on Telegram as [@resizeimgforstickerbot](https://t.me/resizeimgforstickerbot).

This repository contains both the initial Python implementation, as well as a newer Go implementation. The bot currently runs the version written in Go: the version can be checked with the `/stats` command.

There are some slight differences between the two:

Python
- image conversion and compression through pillow
- Redis for statistics

Go
- image conversion through vips
- image compression through pngquant
- statistics periodically dumped from memory to a json-file

Both handle images exclusively in memory, and do not store or cache received files.

## Compiling
Compiling the program from source requires [vips](https://libvips.github.io/libvips/), which can be found from most package managers, including apt and homebrew. With vips installed, run `git clone https://github.com/499602D2/tg-resize-sticker-images`, cd into `tg-resize-sticker-images/Go/src` and run `go build -o resize-bot`. Now you can run the program with `./resize-bot`. The program creates two folders, `config` and `logs`.

## Configuration of the Go version: Cloud API server (default)
Configuration is stored in `botConfig.json`, under the `Go/src/config` folder. If running the program without a local bot API server, the setup is trivial: you're asked to enter your bot's API key, and then you're ready to go.

Additionally, you can set the `Owner` property in the configuration file to your own Telegram user's ID, in order to disable logging of requests made from said account. Finding out your user ID should be trivial from the logs: simply convert an image or run a command.

## Configuration of the Go version: local bot API server
If you're using a local bot API server, the configuration is a little more nuanced. To convert from the default cloud API servers to a local server, first start the bot normally. Then, open `botConfig.json` and under `API` set `LocalAPIEnabled` to `true` and `URL` to your API server's endpoint (default `http://localhost:8081`).

Now start the program once: verify that you see the text `✅ Logged out from cloud API server: please restart the program.` in the log files. Then, open the configuration file again and verify that `CloudAPILoggedOut` is set to `True`, assuming the log-out was successful.

Now, add in your bot API server's working directory into `LocalWorkingDir`: this should be _relative_ to the path of the program. As an example, assuming you have compiled the program and have the compiled as e.g. `resize-bot` in the folder `~/tg-resize-sticker-images/Go/src` and the API server running under `~/telegram-bot-api`, you would set `LocalWorkingDir` to `../../../telegram-bot-api`.

Sample configuration could look as follows (when the executable resides in `~/tg-resize-sticker-images/Go/src`):

````
{
    "Token": "12345:abcdefgh",
    "API": {
        "LocalAPIEnabled": true,
	"CloudAPILoggedOut": true,
	"LocalWorkingDir": "../../../telegram-bot-api",
	"URL": "http://localhost:8081"
    },
    "Owner": 12345,
    "StatConverted": 10,
    "StatUniqueChats": 2,
    "StatStarted": 1629725920,
    "UniqueUsers": [
       12345,
       23456
    ]
}
```














