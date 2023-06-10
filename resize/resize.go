package resize

import (
	"bytes"
	"fmt"
	"math"
	"tg-resize-sticker-images/queue"

	"github.com/h2non/bimg"
	"github.com/rs/zerolog/log"
	"github.com/yusukebe/go-pngquant"
	tb "gopkg.in/telebot.v3"
)

func emojiResizeOptions(options bimg.Options, size bimg.ImageSize) bimg.Options {
	// Force to 100x100 px
	options.Width = 100
	options.Height = 100

	// Set enlarge based on original dimensions
	if (size.Width < 100) || (size.Height < 100) {
		options.Enlarge = true
	}

	return options
}

func stickerResizeOptions(options bimg.Options, size bimg.ImageSize) bimg.Options {
	// Get values for new height and width
	if size.Width >= size.Height {
		// If scaling factor is greater than 1.0, the image needs to be enlarged
		options.Enlarge = (512.0 / float64(size.Width)) > 1.0

		// Set options for width and height
		options.Width = 512
		options.Height = int(math.Round(float64(size.Height) * (512.0 / float64(size.Width))))
	} else {
		// If scaling factor is greater than 1.0, the image needs to be enlarged
		options.Enlarge = (512.0 / float64(size.Height)) > 1.0

		// Set options for width and height
		options.Width = int(math.Round(float64(size.Width) * (512.0 / float64(size.Height))))
		options.Height = 512
	}

	return options
}

// Resizes an image in a byte buffer using libvips through bimg.
func ResizeImage(imgBuffer *bytes.Buffer, inEmojiMode bool) (*queue.Message, error) {
	// Build image from buffer
	image := bimg.NewImage(imgBuffer.Bytes())

	// Read image dimensions for resize (int)
	size, err := image.Size()
	if err != nil {
		log.Error().Err(err).Msg("Error reading image size")

		return &queue.Message{
			Recipient: nil,
			Bytes:     nil,
			Caption:   "⚠️ Error reading image! Please send JPG/PNG/WebP images.",
		}, err
	}

	// Scaling factor and options for processing
	options := bimg.Options{
		Type:          bimg.PNG,          // ImageType(3) == PNG
		StripMetadata: true,              // Strip metadata
		Gravity:       bimg.GravitySmart, // SmartCrop
		Force:         true,              // Force resize to go through
	}

	// Set mode based on inEmojiMode
	mode := "sticker"
	if inEmojiMode {
		mode = "emoji"
	}

	switch mode {
	case "sticker":
		// Resize options for sticker mode
		options = stickerResizeOptions(options, size)

	case "emoji":
		// Resize options for emoji mode
		options = emojiResizeOptions(options, size)

	default:
		// If mode is not 'sticker' or 'emoji', return error
		return &queue.Message{
			Recipient: nil,
			Bytes:     nil,
			Caption:   fmt.Sprintf("⚠️ Invalid mode '%s'!", mode),
		}, fmt.Errorf("Invalid mode '%s'!", mode)
	}

	// Process image in one shot (resize, PNG conversion)
	imageBytes, err := image.Process(options)

	if err != nil {
		// If conversion process fails, notify user
		log.Error().Err(err).Msg("Error processing image")

		return &queue.Message{
			Recipient: nil,
			Bytes:     nil,
			Caption:   "⚠️ Error processing image!",
		}, err
	}

	if len(imageBytes)/1024 >= 512 {
		// Compress image if size is over 512 kibibytes
		imageBytes, err = pngquant.CompressBytes(imageBytes, "6")

		if err != nil {
			// If compression process fails, notify user
			log.Error().Err(err).Msg("Error compressing image")

			return &queue.Message{
				Recipient: nil,
				Bytes:     nil,
				Caption:   "⚠️ Error during image compression!",
			}, err
		}
	}

	// Construct the caption
	imgCaption := fmt.Sprintf(
		"🖼 Here's your %s-ready image (%dx%d)! Forward this to @Stickers.",
		mode, options.Width, options.Height,
	)

	// Notify user if the image was not compressed enough
	// TODO add a "recompress" method
	if len(imageBytes)/1024 >= 512 {
		log.Warn().Msgf("⚠️ Image compression failed, buffer length %d KB", len(imageBytes)/1024)
		imgCaption += "\n\n⚠️ Image compression failed (≥512 KB): you must manually compress the image!"
	}

	inlineBtnText := ""
	switch mode {
	case "sticker":
		// Warn user if image was upscaled
		if options.Enlarge {
			imgCaption += "\n\n⚠️ Image upscaled! Quality may have been lost: consider using a larger image."
		}

		// User is in sticker mode: add text to inline button
		inlineBtnText = "Switch to emoji-mode"
	case "emoji":
		// Warn user if image was upscaled or distorted
		if (options.Enlarge) && (size.Width != size.Height) {
			imgCaption += "\n\n⚠️ Image distorted and upscaled! Consider using a larger, square image."
		} else if options.Enlarge {
			imgCaption += "\n\n⚠️ Image upscaled! Quality may have been lost: consider using a larger image."
		} else if size.Width != size.Height {
			imgCaption += "\n\n⚠️ Image distorted! Consider using a square image."
		}

		// User is in emoji mode: add text to inline button
		inlineBtnText = "Switch to sticker-mode"
	}

	// Add send-options to change mode
	sopts := tb.SendOptions{
		ParseMode: "Markdown",
		ReplyMarkup: &tb.ReplyMarkup{
			InlineKeyboard: [][]tb.InlineButton{{tb.InlineButton{Text: inlineBtnText, Data: "mode/switch"}}},
		},
	}

	return &queue.Message{Recipient: nil, Bytes: &imageBytes, Caption: imgCaption, Sopts: sopts}, nil
}
