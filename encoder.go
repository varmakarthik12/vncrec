package main

/**
* XXX: Ugly workaround for https://github.com/amitbet/vnc2video/issues/10. I've copied the file and build a
* X264ImageCustomEncoder. Once this is merged, we can drop the encoder.go file again.
 */

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	vnc "github.com/amitbet/vnc2video"
	"github.com/amitbet/vnc2video/encoders"
	"github.com/sirupsen/logrus"
)

func encodePPMforRGBA(w io.Writer, img *image.RGBA) error {
	maxvalue := 255
	size := img.Bounds()
	// write ppm header
	_, err := fmt.Fprintf(w, "P6\n%d %d\n%d\n", size.Dx(), size.Dy(), maxvalue)
	if err != nil {
		return err
	}

	if convImage == nil {
		convImage = make([]uint8, size.Dy()*size.Dx()*3)
	}

	rowCount := 0
	for i := 0; i < len(img.Pix); i++ {
		if (i % 4) != 3 {
			convImage[rowCount] = img.Pix[i]
			rowCount++
		}
	}

	if _, err := w.Write(convImage); err != nil {
		return err
	}

	return nil
}

func encodePPMGeneric(w io.Writer, img image.Image) error {
	maxvalue := 255
	size := img.Bounds()
	// write ppm header
	_, err := fmt.Fprintf(w, "P6\n%d %d\n%d\n", size.Dx(), size.Dy(), maxvalue)
	if err != nil {
		return err
	}

	// write the bitmap
	colModel := color.RGBAModel
	row := make([]uint8, size.Dx()*3)
	for y := size.Min.Y; y < size.Max.Y; y++ {
		i := 0
		for x := size.Min.X; x < size.Max.X; x++ {
			color := colModel.Convert(img.At(x, y)).(color.RGBA)
			row[i] = color.R
			row[i+1] = color.G
			row[i+2] = color.B
			i += 3
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

var convImage []uint8

func encodePPM(w io.Writer, img image.Image) error {
	if img == nil {
		return errors.New("nil image")
	}
	img1, isRGBImage := img.(*vnc.RGBImage)
	img2, isRGBA := img.(*image.RGBA)
	if isRGBImage {
		return encodePPMforRGBImage(w, img1)
	} else if isRGBA {
		return encodePPMforRGBA(w, img2)
	}
	return encodePPMGeneric(w, img)
}
func encodePPMforRGBImage(w io.Writer, img *vnc.RGBImage) error {
	maxvalue := 255
	size := img.Bounds()
	// write ppm header
	_, err := fmt.Fprintf(w, "P6\n%d %d\n%d\n", size.Dx(), size.Dy(), maxvalue)
	if err != nil {
		return err
	}

	if _, err := w.Write(img.Pix); err != nil {
		return err
	}
	return nil
}

type HLSEncoder struct {
	encoders.X264ImageEncoder
	FFMpegBinPath      string
	cmd                *exec.Cmd
	input              io.WriteCloser
	closed             bool
	Framerate          int
	ConstantRateFactor int
	SegmentDuration    int // Duration of each HLS segment in seconds
	MaxDuration        int // Maximum recording duration to keep in seconds
	OutputPath         string
}

func (enc *HLSEncoder) Init() {
	if enc.Framerate == 0 {
		enc.Framerate = 12
	}
	if enc.SegmentDuration == 0 {
		enc.SegmentDuration = 10
	}
	if enc.MaxDuration == 0 {
		enc.MaxDuration = 172800 // 2 days in seconds
	}

	// Calculate hls_list_size based on max duration and segment duration
	hlsListSize := enc.MaxDuration / enc.SegmentDuration

	// Use strftime pattern for segment filenames to ensure uniqueness across restarts
	// Format: segment_YYYYMMDD_HHMMSS_%%05d.ts (timestamp + sequence number)
	segmentPattern := filepath.Join(enc.OutputPath, "segment_%Y%m%d_%H%M%S_%%05d.ts")
	playlistPath := filepath.Join(enc.OutputPath, "stream.m3u8")

	cmd := exec.Command(enc.FFMpegBinPath,
		"-f", "image2pipe",
		"-vcodec", "ppm",
		"-r", strconv.Itoa(enc.Framerate),
		"-an", // no audio
		"-y",
		"-i", "-",
		"-vcodec", "libx264",
		"-preset", "veryfast",
		"-g", "250",
		"-crf", strconv.Itoa(enc.ConstantRateFactor),
		"-pix_fmt", "yuv420p",
		"-f", "hls",
		"-hls_time", strconv.Itoa(enc.SegmentDuration),
		"-hls_list_size", strconv.Itoa(hlsListSize),
		"-hls_flags", "delete_segments+append_list+omit_endlist",
		"-strftime", "1",
		"-hls_segment_filename", segmentPattern,
		playlistPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	encInput, err := cmd.StdinPipe()
	enc.input = encInput
	if err != nil {
		logrus.WithError(err).Error("can't get ffmpeg input pipe.")
	}
	enc.cmd = cmd
}

func (enc *HLSEncoder) Run() error {
	if _, err := os.Stat(enc.FFMpegBinPath); os.IsNotExist(err) {
		return err
	}

	enc.Init()
	logrus.WithFields(logrus.Fields{
		"outputPath":      enc.OutputPath,
		"segmentDuration": enc.SegmentDuration,
		"maxDuration":     enc.MaxDuration,
		"hlsListSize":     enc.MaxDuration / enc.SegmentDuration,
	}).Info("Starting HLS recording")
	logrus.Infof("launching binary: %v", enc.cmd)
	err := enc.cmd.Run()
	if err != nil {
		logrus.WithError(err).Errorf("error while launching ffmpeg: %v", enc.cmd.Args)
		return err
	}
	return nil
}

func (enc *HLSEncoder) Encode(img image.Image) {
	if enc.input == nil || enc.closed {
		return
	}

	err := encodePPM(enc.input, img)
	if err != nil {
		logrus.WithError(err).Error("error while encoding image.")
	}
}

func (enc *HLSEncoder) Close() {
	if enc.closed {
		return
	}
	enc.closed = true
	err := enc.input.Close()
	if err != nil {
		logrus.WithError(err).Error("could not close input.")
	}
}

// MP4Encoder records to a single MP4 file
type MP4Encoder struct {
	encoders.X264ImageEncoder
	FFMpegBinPath      string
	cmd                *exec.Cmd
	input              io.WriteCloser
	closed             bool
	Framerate          int
	ConstantRateFactor int
	OutputPath         string
}

func (enc *MP4Encoder) Init(outputFile string) {
	if enc.Framerate == 0 {
		enc.Framerate = 12
	}

	cmd := exec.Command(enc.FFMpegBinPath,
		"-f", "image2pipe",
		"-vcodec", "ppm",
		"-r", strconv.Itoa(enc.Framerate),
		"-an",
		"-y",
		"-i", "-",
		"-vcodec", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-g", "250",
		"-crf", strconv.Itoa(enc.ConstantRateFactor),
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		outputFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	encInput, err := cmd.StdinPipe()
	enc.input = encInput
	if err != nil {
		logrus.WithError(err).Error("can't get ffmpeg input pipe.")
	}
	enc.cmd = cmd
}

func (enc *MP4Encoder) Run(outputFile string) error {
	if _, err := os.Stat(enc.FFMpegBinPath); os.IsNotExist(err) {
		return err
	}

	enc.Init(outputFile)
	logrus.WithField("outputFile", outputFile).Info("Starting MP4 recording")
	logrus.Infof("launching binary: %v", enc.cmd)
	err := enc.cmd.Run()
	if err != nil {
		logrus.WithError(err).Errorf("error while launching ffmpeg: %v", enc.cmd.Args)
		return err
	}
	return nil
}

func (enc *MP4Encoder) Encode(img image.Image) {
	if enc.input == nil || enc.closed {
		return
	}

	err := encodePPM(enc.input, img)
	if err != nil {
		logrus.WithError(err).Error("error while encoding image.")
	}
}

func (enc *MP4Encoder) Close() {
	if enc.closed {
		return
	}
	enc.closed = true
	if enc.input != nil {
		enc.input.Close()
	}
}
