package resize

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/h2non/bimg"
)

func TestResizeFunction(t *testing.T) {
	// Set vips operating parameters, defer shutdown
	bimg.VipsCacheSetMax(250)
	defer bimg.Shutdown()

	// Set default mode to "sticker"
	mode := false

	// Folder containing large test images
	folders := []string{
		"test-images-large/",
	}

	for _, imageFolder := range folders {
		files, err := os.ReadDir(imageFolder)
		if err != nil {
			t.Log("Error opening test image dir:", err)
			t.Fail()
		}

		for _, file := range files {
			// Skip non-image files
			if file.Name()[0] == '.' {
				continue
			}

			// Create path, load image from disk
			filePath := filepath.Join(imageFolder, file.Name())
			file, err := os.Open(filePath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}

			// Defer closing
			defer file.Close()

			// Create buffer
			var imgBuf bytes.Buffer
			_, err = imgBuf.ReadFrom(file)

			if err != nil {
				t.Logf("Error reading image from file! %s", err.Error())
				t.Fail()
			}

			// Resize
			_, err = ResizeImage(&imgBuf, mode)

			if err != nil {
				t.Logf("Error resizing image (%s): %s", file.Name(), err)
				t.Fail()
			}

			fmt.Printf("Successfully resized image %s\n", file.Name())

			// Memory stats
			vipsMem := bimg.VipsMemory()
			fmt.Printf("Allocs: %d | Mem: %d | MemHighW: %d\n\n", vipsMem.Allocations, vipsMem.Memory, vipsMem.MemoryHighwater)
		}
	}
}
