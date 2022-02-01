package resize

import (
	"bytes"
	"fmt"
	"image/png"
	"log"
	"tg-resize-sticker-images/queue"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/h2non/bimg"
	"github.com/yusukebe/go-pngquant"
)

func NewResizeImage(imgBytes []byte) (*queue.Message, error) {
	/*
		Resizes an image in a byte buffer using libvips through bimg.
	*/
	// build image from buffer
	image := bimg.NewImage(imgBytes)

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
	imageBytes, err := image.Convert(3)
	if err != nil {
		// If conversion process fails, notify user
		log.Println("Error converting image to PNG!", err)

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

func ResizeImage(imgBytes []byte) (*queue.Message, error) {
	/*
		Resizes an image in a byte buffer using libvips through govips.

		Inputs:
			imgBytes: the image to resize

		Outputs:
			Message: a message object containing the image and caption
			error: errors encountered during resize
	*/

	defer vips.ShutdownThread()

	// Build image from buffer
	img, err := vips.NewImageFromBuffer(imgBytes)
	if err != nil {
		errorMsg := fmt.Sprintf("⚠️ Error decoding image (%s).", err.Error())
		if err.Error() == "unsupported image format" {
			errorMsg += " Please send JPG/PNG/WebP images."
		}

		return &queue.Message{Recipient: nil, Bytes: nil, Caption: errorMsg}, err
	}

	// defer closing for later
	defer img.Close()

	// Dimensions for resize (int)
	w, h := img.Width(), img.Height()

	// Determine the factor by how much to scale the image with (vips wants f64)
	var resScale float64
	if w >= h {
		resScale = 512.0 / float64(w)
	} else {
		resScale = 512.0 / float64(h)
	}

	// Resize, upscale status
	err = img.Resize(resScale, vips.KernelAuto)
	imgUpscaled := resScale > 1.0

	if err != nil {
		errorMsg := fmt.Sprintf("⚠️ Error resizing image (%s)", err.Error())

		return &queue.Message{Recipient: nil, Bytes: nil, Caption: errorMsg}, err
	}

	// Increment compression ratio if size is too large
	pngParams := vips.PngExportParams{
		StripMetadata: true,
		Compression:   6,
		Interlace:     false,
	}

	// Encode as png into a new buffer
	pngBuff, _, err := img.ExportPng(&pngParams)
	if err != nil {
		var errorMsg string
		if err.Error() == "unsupported image format" {
			errorMsg = "⚠️ Unsupported image format!"
		} else {
			errorMsg = fmt.Sprintf("⚠️ Error encoding image (%s)", err.Error())
		}

		return &queue.Message{Recipient: nil, Bytes: nil, Caption: errorMsg}, err
	}

	// Did we reach the target file size?
	compressionFailed := len(pngBuff)/1024 >= 512

	// If compression fails, run the image through pngquant
	if compressionFailed {
		expParams := vips.ExportParams{
			Format:        vips.ImageTypePNG,
			StripMetadata: true,
			Compression:   6,
		}

		imgImg, err := img.ToImage(&expParams)
		if err != nil {
			log.Println("⚠️ Error exporting image as image.Image:", err)
			return &queue.Message{Recipient: nil, Bytes: nil, Caption: err.Error()}, err
		}

		cImg, err := pngquant.Compress(imgImg, "6")
		if err != nil {
			log.Println("⚠️ Error compressing image with pngquant:", err)
			return &queue.Message{Recipient: nil, Bytes: nil, Caption: err.Error()}, err
		}

		// Write to buffer
		cBuff := new(bytes.Buffer)
		err = png.Encode(cBuff, cImg)
		if err != nil {
			log.Println("⚠️ Error encoding cImg as png:", err)
			return &queue.Message{Recipient: nil, Bytes: nil, Caption: err.Error()}, err
		}

		pngBuff = cBuff.Bytes()
		compressionFailed = len(pngBuff)/1024 >= 512

		if compressionFailed {
			log.Println("\t⚠️ Image compression failed! Buffer length (KB):", len(cBuff.Bytes())/1024)
		}
	}

	// Construct the caption
	imgCaption := fmt.Sprintf(
		"🖼 Here's your sticker-ready image (%dx%d)! Forward this to @Stickers.", img.Width(), img.Height(),
	)

	// Add notice to user if image was upscaled or compressed
	if imgUpscaled {
		imgCaption += "\n\n⚠️ Image upscaled! Quality may have been lost: consider using a larger image."
	} else if compressionFailed {
		imgCaption += "\n\n⚠️ Image compression failed (≥512 KB): you must manually compress the image!"
	}

	return &queue.Message{Recipient: nil, Bytes: &pngBuff, Caption: imgCaption}, nil
}
