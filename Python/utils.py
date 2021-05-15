import os
import time
import datetime

import ujson as json

def store_config(config_json: dict, data_dir: str):
	'''
	Stores the configuration specified in config_json onto disk.

	Keyword arguments:
		config_json (dict): new config dictionary

	Returns:
		None
	'''
	with open(os.path.join(data_dir, 'bot-config.json'), 'w') as config_file:
		json.dump(config_json, config_file, indent=4)

	print('Updated config dumped!')


def create_config(data_dir: str):
	'''
	Runs the config file setup if file doesn't exist or is corrupted/missing data.

	Keyword arguments:
		data_dir (str): location where config file is created

	Returns:
		None
	'''
	if not os.path.isdir(data_dir):
		os.makedirs(data_dir)

	with open(os.path.join(data_dir, 'bot-config.json'), 'w') as config_file:
		print('\nTo function, stickerResizeBot needs a bot API key;')
		print('to get one, send a message to @botfather on Telegram.')

		bot_token = input('Enter bot token: ')
		print()

		config = {
			'bot_token': bot_token,
			'owner': 0,
			'redis': {
				'host': 'localhost',
				'port': 6379,
				'db_num': 0
			},
			'local_api_server': {
				'enabled': False,
				'address': None,
    			'api_id': None,
    			'api_hash': None
			}
		}

		json.dump(config, config_file, indent=4)


def load_config(data_dir: str) -> dict:
	'''
	Load variables from configuration file.

	Keyword arguments:
		data_dir (str): location of config file

	Returns:
		config (dict): configuration in json/dict format
	'''
	# if config file doesn't exist, create it
	if not os.path.isfile(os.path.join(data_dir, 'bot-config.json')):
		print('Config file not found: performing setup.\n')
		create_config(data_dir)

	with open(os.path.join(data_dir, 'bot-config.json'), 'r') as config_file:
		try:
			return json.load(config_file)
		except:
			print('JSONDecodeError: error loading configuration file. Running config setup...')

			create_config(data_dir)
			return load_config(data_dir)

	with open(os.path.join(data_dir, 'bot-config.json'), 'r') as config_file:
		return json.load(config_file)


def time_delta_to_legible_eta(time_delta: int, full_accuracy: bool) -> str:
	'''
	This is a tiny helper function, used to convert integer time deltas
	(i.e. second deltas) to a legible ETA, where the largest unit of time
	is measured in days.

	Keyword arguments:
		time_delta (int): time delta in seconds to convert
		full_accuracy (bool): whether to use triple precision or not
			(in this context, e.g. dd:mm:ss vs. dd:mm)

	Returns:
		pretty_eta (str): the prettily formatted, readable ETA string
	'''
	# convert time delta to a semi-redable format: {days, hh:mm:ss}
	eta_str = "{}".format(str(datetime.timedelta(seconds=time_delta)))

	# parse into a "pretty" string. If ',' in string, it's more than 24 hours.
	if ',' in eta_str:
		day_str = eta_str.split(',')[0]
		hours = int(eta_str.split(',')[1].split(':')[0])
		mins = int(eta_str.split(',')[1].split(':')[1])

		if hours > 0 or full_accuracy:
			pretty_eta = f'{day_str}{f", {hours} hour"}'

			if hours != 1:
				pretty_eta += 's'

			if full_accuracy:
				pretty_eta += f', {mins} minute{"s" if mins != 1 else ""}'

		else:
			if mins != 0 or full_accuracy:
				pretty_eta = f'{day_str}{f", {mins} minute"}'

				if mins != 1:
					pretty_eta += 's'
			else:
				pretty_eta = f'{day_str}'
	else:
		# split eta_string into hours, minutes, and seconds -> convert to integers
		hhmmss_split = eta_str.split(':')
		hours, mins, secs = (
			int(hhmmss_split[0]),
			int(hhmmss_split[1]),
			int(float(hhmmss_split[2]))
		)

		if hours > 0:
			pretty_eta = f'{hours} hour{"s" if hours > 1 else ""}'
			pretty_eta += f', {mins} minute{"s" if mins != 1 else ""}'

			if full_accuracy:
				pretty_eta += f', {secs} second{"s" if secs != 1 else ""}'

		else:
			if mins > 0:
				pretty_eta = f'{mins} minute{"s" if mins != 1 else ""}'
				pretty_eta += f', {secs} second{"s" if secs != 1 else ""}'
			else:
				if secs > 0:
					pretty_eta = f'{secs} second{"s" if secs != 1 else ""}'
				else:
					pretty_eta = 'just now'

	return pretty_eta
