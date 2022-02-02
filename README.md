# tg-resize-sticker-images
A fast and tiny, privacy-preserving bot that converts and compresses images so that they can be turned into Telegram stickers. Reachable on Telegram as [@resizeimgforstickerbot](https://t.me/resizeimgforstickerbot).

**Bot features**
- purely in-memory processing: nothing saved to disk
- fast image conversion, compression, and sending
- image conversion through libvips
- image compression through pngquant
- statistics periodically dumped from memory to a json-file

The bot handles images exclusively in memory, and _does not_ store or cache received files. In order to collect statistics on how many people use the bot, the user ID of every user is stored in a json-file. This is the only information collected.

The current version the bot runs can be seen by running the `/stats` command.

## Compiling
Compiling the program from source requires [vips](https://www.libvips.org/.) Vips can be found in most package managers as `libvips`, including apt and homebrew. With vips installed, run `git clone https://github.com/499602D2/tg-resize-sticker-images`, cd into `/tg-resize-sticker-images` and run `go build -o resize-bot`. Now you can run the program with `./resize-bot`. The program stores log-files under `/logs`.

### Possible compilation errors (macOS)
    go build github.com/davidbyttow/govips/v2/vips: invalid flag in pkg-config --cflags: -Xpreprocessor

This can be fixed by running `export CGO_CFLAGS_ALLOW=-Xpreprocessor` in your shell.


## Configuring the bot
Configuration is stored in `botConfig.json`, under the `/config` folder. The setup is trivial: you're asked to enter your bot's API key, and then you're ready to go.

Additionally, you can set the `Owner` property in the configuration file to your own Telegram user ID, in order to disable logging of requests made from said account. Finding out your user ID should be trivial from the logs: simply convert an image or run a command.

A sample configuration file looks as follows:

```
{
    "Token": "12345:abcdefgh",
    "Owner": 12345,
    "StatConverted": 10,
    "StatUniqueChats" (2,
    "StatStarted": 1629725920,
    "UniqueUsers": [
       12345,
       23456
    ]
}
```

## Changelog
<details>
  <summary>View historical changelog</summary>

0.0.0 (2021.03.29): started

1.0.0 (2021.05.15): first go implementation

1.1.0 (2021.05.16): keeping track of unique chats, binsearch

1.2.0 (2021.05.17): callback buttons for /stats

1.3.0 (2021.05.17): image compression with pngquant

1.3.1 (2021.05.19): bug fixes, error handling

1.4.0 (2021.08.22): error handling, local API support, handle interrupts

1.4.1 (2021.08.25): logging changes to reduce disk writes

1.5.0 (2021.08.30): added anti-spam measures, split the program into modules

1.5.1 (2021.09.01): fix concurrent map writes

1.5.2 (2021.09.09): improvements to spam management

1.5.3 (2021.09.10): address occasional runtime errors

1.5.4 (2021.09.13): tweaks to file names

1.5.5 (2021.09.15): tweaks to error messages, memory

1.5.6 (2021.09.27): logging improvements, add anti-spam insights

1.5.7 (2021.09.30): callbacks for /spam, logging

1.5.8 (2021.11.11): improvements to /spam command, bump telebot + core

1.6.0 (2021.11.13): implement a message send queue, locks for config

1.6.1 (2021.11.13): send error messages with queue

1.6.2 (2021.11.14): add session struct, simplify media handling, add webp support

1.6.3 (2021.11.15): log dl/resize failures, improve /start

1.6.4 (2021.11.15): don't store chat ID on /start

1.7.0 (2021.12.08): upgrade to telebot v3 and migrate code

1.7.1 (2021.12.21): code refactor, bump deps

1.8.0 (2022.02.02): rewrite resize function, optimize download flow, remove local API code, refactor code, small fixes
</details>
