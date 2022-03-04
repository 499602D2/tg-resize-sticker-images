package resize

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"tg-resize-sticker-images/queue"

	"github.com/h2non/bimg"
	"github.com/yusukebe/go-pngquant"
)

func ResizeImage(imgBuffer *bytes.Buffer) (*queue.Message, error) {
	/*
		Resizes an image in a byte buffer using libvips through bimg.
	*/
	// build image from buffer
	image := bimg.NewImage(imgBuffer.Bytes())

	// Read image dimensions for resize (int)
	size, err := image.Size()
	if err != nil {
		log.Println("Error reading image size:", err)

		return &queue.Message{
			Recipient: nil,
			Bytes:     nil,
			Caption:   "⚠️ Error reading image! Please send JPG/PNG/WebP images.",
		}, err
	}

	// Scaling factor and options for processing
	options := bimg.Options{
		Type:          bimg.ImageType(3), // ImageType(3) == PNG
		StripMetadata: true,              // Strip metadata
		Gravity:       bimg.GravitySmart, // SmartCrop
		Force:         true,              // Force resize to go through
	}

	// Get values for new height and width
	if size.Width >= size.Height {
		// Width >= height: set width to 512, scale height appropriately.
		scalingFactor := 512.0 / float64(size.Width)

		// If image needs to be enlarged, set
		options.Enlarge = scalingFactor > 1.0

		// Set options for width and height
		options.Width = 512
		options.Height = int(math.Round(float64(size.Height) * scalingFactor))
	} else {
		// Height >= width: set height to 512, scale width appropriately
		scalingFactor := 512.0 / float64(size.Height)

		// If image needs to be enlarged, set
		options.Enlarge = scalingFactor > 1.0

		// Set options for width and height
		options.Width = int(math.Round(float64(size.Width) * scalingFactor))
		options.Height = 512
	}

	// Process image in one shot (resize, PNG conversion)
	imageBytes, err := image.Process(options)

	if err != nil {
		// If conversion process fails, notify user
		log.Println("Error processing image!", err.Error())

		return &queue.Message{
			Recipient: nil,
			Bytes:     nil,
			Caption:   "⚠️ Error processing image!",
		}, err
	}

	// Compress image if size is too large
	if len(imageBytes)/1024 >= 512 {
		imageBytes, err = pngquant.CompressBytes(imageBytes, "6")
		if err != nil {
			// If compression process fails, notify user
			log.Println("Error compressing image:", err.Error())

			return &queue.Message{
				Recipient: nil,
				Bytes:     nil,
				Caption:   "⚠️ Error during image compression!",
			}, err
		}
	}

	// Construct the caption
	imgCaption := fmt.Sprintf(
		"🖼 Here's your sticker-ready image (%dx%d)! Forward this to @Stickers.",
		options.Width, options.Height,
	)

	// Notify user if the image was not compressed enough
	if len(imageBytes)/1024 >= 512 {
		log.Println("⚠️ Image compression failed! Buffer length (KB):", len(imageBytes)/1024)
		imgCaption += "\n\n⚠️ Image compression failed (≥512 KB): you must manually compress the image!"
	}

	// Notify user if image was upscaled
	if options.Enlarge {
		imgCaption += "\n\n⚠️ Image upscaled! Quality may have been lost: consider using a larger image."
	}

	return &queue.Message{Recipient: nil, Bytes: &imageBytes, Caption: imgCaption}, nil
}
