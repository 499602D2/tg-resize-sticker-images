import os
import sys
import time
import io
import signal
import logging

from inspect import cleandoc
from datetime import datetime

import coloredlogs
import cursor
import redis

from PIL import Image
from resizeimage import resizeimage
from telegram import InputFile
from telegram.ext import Updater, CommandHandler, MessageHandler, Filters

from utils import load_config, time_delta_to_legible_eta

# keep track of time spent running
STARTUP_TIME = time.time()

# connect to redis for stats
rd = redis.Redis(host='localhost', port=6379, db=3, decode_responses=True)

def start(update, context):
	'''
	Responds to /start and /help commands.
	'''
	# construct message
	reply_msg = f'''🖼 *Resize Image for Stickers v{VERSION}*

	To resize an image to a sticker-ready format, just send it to this chat!

	[This bot is open-source.](https://github.com/499602D2/tg-resize-sticker-images)
	'''

	# pull chat id, send message
	chat_id = update.message.chat.id
	context.bot.send_message(chat_id, cleandoc(reply_msg), parse_mode='Markdown')

	logging.info(f'🌟 Bot added to a new chat! chat_id={chat_id}.')


def help(update, context):
	'''
	Responds to /help command
	'''
	# construct message
	reply_msg = '''🖼 To use the bot, simply send your image to this chat (jpg/png/webp).

	Hint: you can also send multiple images at once!
	'''

	# pull chat id, send message
	chat_id = update.message.chat.id
	context.bot.send_message(chat_id, cleandoc(reply_msg), parse_mode='Markdown')

	logging.info(f'🤖 Chat {chat_id} requested help.')


def source(update, context):
	'''
	Responds to /source command
	'''
	# construct message
	reply_msg = '''🐙 [Source on Github.](https://github.com/499602D2/tg-resize-sticker-images)
	'''

	# pull chat id, send message
	chat_id = update.message.chat.id
	context.bot.send_message(chat_id, cleandoc(reply_msg), parse_mode='Markdown')

	logging.info(f'🐙 Chat {chat_id} requested Github link!')


def statistics(update, context):
	'''
	Responds to /stats command
	'''
	if rd.exists('converted-imgs'):
		imgs = int(rd.get('converted-imgs'))
	else:
		imgs = 0

	if rd.exists('chats'):
		chats = rd.get('chats')
		chats = len(chats.split(','))
	else:
		chats = 0

	sec_running = int(time.time()) - STARTUP_TIME
	runtime = time_delta_to_legible_eta(time_delta=sec_running, full_accuracy=False)

	msg = f'''📊 *Bot statistics*
	Images converted: {imgs}
	Unique chats seen: {chats}
	Bot started {runtime} ago
	'''

	context.bot.send_message(
		update.message.chat.id, cleandoc(msg), parse_mode='Markdown')


def convert_img(update, context):
	# log start
	logging.info(f'Starting image conversion for {update.message.chat.id}...')

	# load img
	photo = update.message.photo[-1]
	photo_file = photo.get_file()

	# write to byte array, open
	img_bytes = photo_file.download_as_bytearray()
	img = Image.open(io.BytesIO(img_bytes))

	if img.format in ('JPEG', 'WEBP'):
		img = img.convert('RGB')
	elif img.format == 'PNG':
		pass
	else:
		logging.info(f'Image conversion failed for {update.message.chat.id}: not a jpg/png/webp!')
		context.bot.send_message(text='⚠️ Error: file is not a jpg/png/webp')
		return

	# read image dimensions
	w, h = img.size

	# resize larger side to 512
	if w >= h:
		img = resizeimage.resize_width(img, 512)
	else:
		img = resizeimage.resize_height(img, 512)

	# read width, height of new image
	w, h = img.size

	# save image to buffer
	byte_arr = io.BytesIO()
	img.save(byte_arr, format='PNG', compress_level=0)

	# compress if size > 512 KB (kibi, not kilo)
	if byte_arr.tell() / 1024 > 512:
		fsize = byte_arr.tell() / 1024
		compression_level, optimize = 1, False

		logging.warning(f'Image is too large ({fsize:.2f} KB): compressing...')
		while fsize > 512:
			if compression_level > 9:
				optimize = True

			temp = io.BytesIO()
			img.save(
				temp, format='PNG', optimize=optimize,
				compression_level=compression_level)

			fsize = temp.tell() / 1024
			byte_arr = temp

			logging.warning(f'\t{fsize:.2f} KB | clevel={compression_level}, optimize={optimize}')
			compression_level += 1
			if optimize:
				break

	# create telegram.InputFile object by reading raw bytes
	byte_arr.seek(0)
	img_file = InputFile(byte_arr)

	context.bot.send_document(
		chat_id=update.message.chat.id, document=img_file,
		caption=f"🖼 Here's your sticker-ready image ({w}x{h})! Forward this to @Stickers.",
		filename=f'resized-image-{int(time.time())}.png')

	# add +1 to stats
	if rd.exists('converted-imgs'):
		rd.set('converted-imgs', int(rd.get('converted-imgs')) + 1)
	else:
		rd.set('converted-imgs', 1)

	if rd.exists('chats'):
		chat_list = rd.get('chats').split(',')
		if str(update.message.chat.id) not in chat_list:
			chat_list.append(str(update.message.chat.id))
			rd.set('chats', ','.join(chat_list))
	else:
		rd.set('chats', str(update.message.chat.id))

	logging.info(f'{update.message.chat.id} successfully converted an image!')


def sigterm_handler(signal, frame):
	'''
	Logs program run time when we get sigterm.
	'''
	logging.info(f'✅ Got SIGTERM. Runtime: {datetime.now() - STARTUP_TIME}.')
	logging.info(f'Signal: {signal}, frame: {frame}.')
	sys.exit(0)


if __name__ == '__main__':
	VERSION = '1.2'
	DATA_DIR = 'data'
	DEBUG = True

	# load config, load bot
	config = load_config(data_dir=DATA_DIR)
	updater = Updater(config['bot_token'], use_context=True)

	# init log (disk)
	log = os.path.join(DATA_DIR, 'log-file.log')
	logging.basicConfig(
		filename=log, level=logging.DEBUG, format='%(asctime)s %(message)s', datefmt='%d/%m/%Y %H:%M:%S')

	# disable logging for urllib and requests because jesus fuck they make a lot of spam
	logging.getLogger('urllib3').setLevel(logging.CRITICAL)
	logging.getLogger('chardet.charsetprober').setLevel(logging.CRITICAL)
	logging.getLogger('telegram').setLevel(logging.ERROR)
	logging.getLogger('telegram.bot').setLevel(logging.ERROR)
	logging.getLogger('telegram.ext.updater').setLevel(logging.ERROR)
	logging.getLogger('telegram.vendor').setLevel(logging.ERROR)
	logging.getLogger('telegram.error.TelegramError').setLevel(logging.ERROR)
	coloredlogs.install(level='DEBUG')

	# get the dispatcher to register handlers
	dispatcher = updater.dispatcher

	dispatcher.add_handler(MessageHandler(Filters.photo, callback=convert_img))
	dispatcher.add_handler(CommandHandler(command=('start'), callback=start))
	dispatcher.add_handler(CommandHandler(command=('source'), callback=source))
	dispatcher.add_handler(CommandHandler(command=('stats'), callback=statistics))

	# all up to date, start polling
	updater.start_polling()

	# handle sigterm
	signal.signal(signal.SIGTERM, sigterm_handler)

	# hide cursor for pretty print
	if not DEBUG:
		cursor.hide()
		try:
			while True:
				for char in ('⠷', '⠯', '⠟', '⠻', '⠽', '⠾'):
					sys.stdout.write('%s\r' % '  Connected to Telegram! To quit: ctrl + c.')
					sys.stdout.write('\033[92m%s\r\033[0m' % char)
					sys.stdout.flush()
					time.sleep(0.1)

		except KeyboardInterrupt:
			# on exit, show cursor as otherwise it'll stay hidden
			cursor.show()
			logging.info('Ending...')
	else:
		while True:
			time.sleep(10)
