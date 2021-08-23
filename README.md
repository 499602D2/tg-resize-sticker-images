# tg-resize-sticker-images
A tiny bot that converts images so they can be turned into tg stickers. Reachable on Telegram as [@resizeimgforstickerbot](https://t.me/resizeimgforstickerbot).

This repository contains both the initial Python implementation, as well as a newer Go implementation. The bot currently runs the version written in Go.

There are some slight differences between the two:

Python
- image conversion and compression through pillow
- Redis for statistics

Go
- image conversion through vips
- image compression through pngquant
- statistics periodically dumped from memory to a json-file

Both handle images exclusively in memory, and do not store or cache received files.