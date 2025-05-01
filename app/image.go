package app

import (
	"bytes"
	"fmt"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"urigallery/cache"
	"urigallery/util"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
)

type Image struct {
	Name      string `msgpack:"name"`
	Path      string `msgpack:"path"`
	Size      int    `msgpack:"size"`
	CreatedAt int64  `msgpack:"createdAt"`
}

func cacheKey(path string, size *int) string {
	if size == nil {
		return fmt.Sprintf("image/%s", path)
	}

	return fmt.Sprintf("image/%s_%d", path, *size)
}

var imageSuffixes = []string{".jpg", ".png"}

func isImageName(name string) bool {
	ext := filepath.Ext(name)
	return slices.Contains(imageSuffixes, ext)
}

func findImages(path string, prefix string) ([]Image, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	images := []Image{}

	for _, file := range files {
		if file.IsDir() {
			subImages, err := findImages(filepath.Join(path, file.Name()), filepath.Join(prefix, file.Name()))
			if err != nil {
				return nil, err
			}

			images = append(images, subImages...)
		} else {
			name := file.Name()
			if !isImageName(name) {
				continue
			}

			info, err := file.Info()
			if err != nil {
				return nil, err
			}

			images = append(images, Image{
				Name:      name,
				Path:      filepath.Join(prefix, name),
				Size:      int(info.Size()),
				CreatedAt: info.ModTime().UnixMilli(),
			})
		}
	}

	return images, nil
}

func resizeImage(picturesDir string, path string, size int) (io.Reader, error) {
	cacheKey := cacheKey(path, &size)
	cacheEntry, err := cache.Get(cacheKey)
	if err == nil {
		return util.CloseOnEOFReader(cacheEntry), nil
	}

	img, err := imgio.Open(filepath.Join(picturesDir, path))
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer(nil)

	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	if width < height {
		ratio := float64(height) / float64(width)
		height = size
		width = int(float64(height) / ratio)
		img = transform.Resize(img, width, height, transform.CatmullRom)
	} else {
		ratio := float64(width) / float64(height)
		width = size
		height = int(float64(width) / ratio)
		img = transform.Resize(img, width, height, transform.CatmullRom)
	}

	if err := png.Encode(buffer, img); err != nil {
		return nil, err
	}
	if err := cache.Set(cacheKey, buffer.Bytes()); err != nil {
		return nil, err
	}
	return buffer, nil
}

type ListImagesResponse struct {
	Images []Image `msgpack:"images"`
}

func ListImages(request *Request, picturesDir string) (*Response, error) {
	images, err := findImages(picturesDir, "")
	if err != nil {
		return nil, err
	}

	return newResponse().Success().Object(&ListImagesResponse{
		Images: images,
	}), nil
}

type DownloadImageRequest struct {
	Path   string `msgpack:"path"`
	Resize *int   `msgpack:"resize"`
}

func DownloadImage(request *Request, picturesDir string) (*Response, error) {
	data, err := Object[DownloadImageRequest](request)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if data.Resize != nil {
		reader, err = resizeImage(picturesDir, data.Path, *data.Resize)
		if err != nil {
			return nil, err
		}
	} else {
		file, err := os.Open(filepath.Join(picturesDir, data.Path))
		if err != nil {
			return nil, err
		}

		reader = util.CloseOnEOFReader(file)
	}
	return newResponse().Success().Reader(reader), nil
}
