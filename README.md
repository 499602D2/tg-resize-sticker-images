# tg-resize-sticker-images
A fast and tiny, privacy-preserving bot that converts and compresses images so that they can be turned into Telegram stickers. Reachable on Telegram as [@resizeimgforstickerbot](https://t.me/resizeimgforstickerbot).

**Bot features**
- purely in-memory processing: nothing saved to disk
- fast image conversion, compression, and sending
	- conversion through libvips
	- compression through pngquant
- statistics periodically dumped from memory to a json-file

The bot handles images exclusively in memory, and does not store or cache received files. In order to collect statistics on how many people use the bot, the user ID of every user is stored in a json-file. This is the only information collected.

The current version the bot runs can be seen by running the `/stats` command.

## Compiling
Compiling the program from source requires [vips](https://www.libvips.org/). Vips can be found in most package managers as `libvips`, including apt and homebrew. With vips installed, run `git clone https://github.com/499602D2/tg-resize-sticker-images`, cd into `/tg-resize-sticker-images` and run `./build.sh`. Now you can run the program with `./tg-resize-sticker-images`. The program stores log-files under `/logs`.

### Possible compilation errors (macOS)
    go build github.com/h2non/bimg: invalid flag in pkg-config --cflags: -Xpreprocessor

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
    "StatUniqueChats": 2,
    "StatStarted": 1629725920,
    "UniqueUsers": [
       12345,
       23456
    ]
}
```

## Python implementation
Version 1.3.3 ("1.3.7") is the last Python version of the bot, and can be browsed at commit height [5c9effd](https://github.com/499602D2/tg-resize-sticker-images/tree/5c9effd4883e1f91a5abe9fca7e0f2650c986a76). This version was last updated in April of 2021, and used Pillow for image conversion and python-resize-image for image resizing.

## Changelog
<details>
  <summary>View historical changelog</summary>

	1.1 (2020.12.04): first tracked Python implementation

	1.2 (2020.12.05): optimized image quality
	
	1.3 (2020.12.06): help and source-code commands, warnings, uncompressed images
	
	1.3.1 (2020.12.13): limit compression level to 9, optimize images
	
	1.3.2 (2020.12.13): handle exceptions when bot receives a random file 
	
	1.3.3 (2021.01.14): handle various network exceptions, fix webp support
	
	1.3.4 (2021.02.07): use path relative to script location for data directory
	
	1.3.5 (2021.03.01): handle network timeouts
	
	1.3.6 (2021.03.02): properly stop updater on quit
	
	1.3.7 (2021.04.13): thousand separators for statistic message

	2.0.b (2021.03.29): Go implementation started

	2.0.0 (2021.05.15): first Go implementation

	2.1.0 (2021.05.16): keeping track of unique chats, binsearch

	2.2.0 (2021.05.17): callback buttons for /stats

	2.3.0 (2021.05.17): image compression with pngquant

	2.3.1 (2021.05.19): bug fixes, error handling

	2.4.0 (2021.08.22): error handling, local API support, handle interrupts

	2.4.1 (2021.08.25): logging changes to reduce disk writes

	2.5.0 (2021.08.30): added anti-spam measures, split the program into modules

	2.5.1 (2021.09.01): fix concurrent map writes

	2.5.2 (2021.09.09): improvements to spam management

	2.5.3 (2021.09.10): address occasional runtime errors

	2.5.4 (2021.09.13): tweaks to file names

	2.5.5 (2021.09.15): tweaks to error messages, memory

	2.5.6 (2021.09.27): logging improvements, add anti-spam insights

	2.5.7 (2021.09.30): callbacks for /spam, logging

	2.5.8 (2021.11.11): improvements to /spam command, bump telebot + core

	2.6.0 (2021.11.13): implement a message send queue, locks for config

	2.6.1 (2021.11.13): send error messages with queue

	2.6.2 (2021.11.14): add session struct, simplify media handling, add webp support

	2.6.3 (2021.11.15): log dl/resize failures, improve /start

	2.6.4 (2021.11.15): don't store chat ID on /start

	2.7.0 (2021.12.08): upgrade to telebot v3 and migrate code

	2.7.1 (2021.12.21): code refactor, bump deps

	2.8.0 (2022.02.02): rewrite resize function, optimize download flow, remove local API code, refactor code, small fixes

	2.8.1 (2022.02.02): go mod tidy, fix nil pointer dereference

	2.8.2 (2022.02.08): bump deps, added build script, optimize request flow

	2.8.3 (2022.03.03): attempt to fix resize errors, reduce queue clear interval, bump gocron 

	2.9.0 (2022.03.04): improve resize flow and fix errors when processing small images

	2.10.0 (2022.06.04): simplified and improved spam management, logging and message changes, code cleanup

	2.10.1 (2022.12.17): bump dependencies, use UUID for images, minor code cleanup

	2.10.2 (2022.12.18): correct major version, remove use of depreacted io/ioutil

	2.11.0 (2023.06.10): 100x100 px conversion mode, trailing-day statistics, bump dependencies
</details>
