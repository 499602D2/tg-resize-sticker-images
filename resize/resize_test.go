package resize

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/h2non/bimg"
)

func TestResizeFunction(t *testing.T) {
	// Start vips
	vips.LoggingSettings(nil, vips.LogLevel(3))
	vips.Startup(&vips.Config{MaxCacheSize: 16})

	// Folder containing test images
	imageFolder := "test-images/"

	files, err := ioutil.ReadDir(imageFolder)
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
		imgBuf, err := ioutil.ReadFile(filePath)

		if err != nil {
			t.Log("Error loading image:", err)
			t.Fail()
		}

		// Resize
		_, err = ResizeImage(imgBuf)

		if err != nil {
			t.Logf("Error resizing image (%s): %s", file.Name(), err)
			t.Fail()
		}

		fmt.Printf("Successfully resized image %s\n", file.Name())
		break
	}

	vips.Shutdown()
}

func TestNewResizeFunction(t *testing.T) {
	// Set vips operating parameters, defer shutdown
	bimg.VipsCacheSetMax(16)
	defer bimg.Shutdown()

	// Folder containing large test images
	imageFolder := "test-images-large/"

	files, err := ioutil.ReadDir(imageFolder)
	if err != nil {
		t.Log("Error opening test image dir:", err)
		t.Fail()
	}

	var i int
	for i < 3 {
		for _, file := range files {
			// Skip non-image files
			if file.Name()[0] == '.' {
				continue
			}

			// Create path, load image from disk
			filePath := filepath.Join(imageFolder, file.Name())
			imgBuf, err := ioutil.ReadFile(filePath)

			if err != nil {
				t.Log("Error loading image:", err)
				t.Fail()
			}

			// Resize
			_, err = NewResizeImage(imgBuf)

			if err != nil {
				t.Logf("Error resizing image (%s): %s", file.Name(), err)
				t.Fail()
			}

			fmt.Printf("[newResizeFunc] Successfully resized image %s\n", file.Name())

			// Memory stats
			vipsMem := bimg.VipsMemory()
			fmt.Printf("Allocs: %d | Mem: %d | MemHighW: %d\n", vipsMem.Allocations, vipsMem.Memory, vipsMem.MemoryHighwater)

		}
		i++
	}
}
