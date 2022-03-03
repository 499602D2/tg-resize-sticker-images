package resize

import (
	"bytes"
	"fmt"
	"log"
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

	// Resize image
	var scalingFactor float64
	if size.Width >= size.Height {
		// Width >= height: set width to 512, scale height appropriately.
		scalingFactor = 512.0 / float64(size.Width)
		image.Resize(512, int(float64(size.Height)*scalingFactor))
	} else {
		// Height >= width: set height to 512, scale width appropriately
		scalingFactor = 512.0 / float64(size.Height)
		image.Resize(int(float64(size.Width)*scalingFactor), 512)
	}

	// Save new size, check that image is still readable
	newSize, err := image.Size()
	if err != nil {
		log.Println("Error reading image size after resizing:", err)

		return &queue.Message{
			Recipient: nil,
			Bytes:     nil,
			Caption:   "⚠️ Error resizing image! Please send JPG/PNG/WebP images.",
		}, err
	}

	// Convert image to a PNG
	imageBytes, err := image.Convert(bimg.ImageType(3))
	if err != nil {
		// If conversion process fails, notify user
		log.Println("Error converting image to PNG!", err.Error())

		return &queue.Message{
			Recipient: nil,
			Bytes:     nil,
			Caption:   "⚠️ Error converting image to PNG!",
		}, err
	}

	// Compress image if size is too large
	if len(imageBytes)/1024 >= 512 {
		imageBytes, err = pngquant.CompressBytes(imageBytes, "6")
		if err != nil {
			// If compression process fails, notify user
			log.Println("Error compressing image:", err)

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
		newSize.Width, newSize.Height,
	)

	// Notify user if the image was not compressed enough
	if len(imageBytes)/1024 >= 512 {
		log.Println("⚠️ Image compression failed! Buffer length (KB):", len(imageBytes)/1024)
		imgCaption += "\n\n⚠️ Image compression failed (≥512 KB): you must manually compress the image!"
	}

	// Notify user if image was upscaled
	if scalingFactor > 1.0 {
		imgCaption += "\n\n⚠️ Image upscaled! Quality may have been lost: consider using a larger image."
	}

	return &queue.Message{Recipient: nil, Bytes: &imageBytes, Caption: imgCaption}, nil
}
