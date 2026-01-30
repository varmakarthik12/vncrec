package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"net"
	"path/filepath"
	"strings"

	vnc "github.com/amitbet/vnc2video"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
	"time"
)

const (
	initialRetryDelay = 5 * time.Second
	maxRetryDelay     = 2 * time.Minute
)

// generateRandomSuffix creates a random hex string for unique filenames
func generateRandomSuffix() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp if random fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// generateOutputFilename creates a filename with a random suffix
// e.g., "output.mp4" -> "output-a3f8b2c1.mp4"
func generateOutputFilename(basePattern string) string {
	ext := filepath.Ext(basePattern)
	base := strings.TrimSuffix(basePattern, ext)
	suffix := generateRandomSuffix()
	return fmt.Sprintf("%s-%s%s", base, suffix, ext)
}

// commonFlags returns the flags shared between record and daemon commands
func commonFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "ffmpeg",
			Value:   "ffmpeg",
			Usage:   "Which ffmpeg executable to use",
			EnvVars: []string{"VR_FFMPEG_BIN"},
		},
		&cli.StringFlag{
			Name:    "host",
			Value:   "localhost",
			Usage:   "VNC host",
			EnvVars: []string{"VR_VNC_HOST"},
		},
		&cli.IntFlag{
			Name:    "port",
			Value:   5900,
			Usage:   "VNC port",
			EnvVars: []string{"VR_VNC_PORT"},
		},
		&cli.StringFlag{
			Name:    "password",
			Value:   "secret",
			Usage:   "Password to connect to the VNC host",
			EnvVars: []string{"VR_VNC_PASSWORD"},
		},
		&cli.IntFlag{
			Name:    "framerate",
			Value:   30,
			Usage:   "Framerate to record",
			EnvVars: []string{"VR_FRAMERATE"},
		},
		&cli.IntFlag{
			Name:    "crf",
			Value:   35,
			Usage:   "Constant Rate Factor (CRF) to record with",
			EnvVars: []string{"VR_CRF"},
		},
		&cli.StringFlag{
			Name:    "output-path",
			Value:   "",
			Usage:   "Output directory path for recordings. Default: current directory with 'recordings' subfolder",
			EnvVars: []string{"VR_OUTPUT_PATH"},
		},
		&cli.StringFlag{
			Name:    "format",
			Value:   "mp4",
			Usage:   "Output format: 'mp4' (default) or 'hls'",
			EnvVars: []string{"VR_FORMAT"},
		},
		&cli.IntFlag{
			Name:    "mp4-max-duration",
			Value:   1800,
			Usage:   "Maximum duration per MP4 file in seconds (default: 1800 = 30 min)",
			EnvVars: []string{"VR_MP4_MAX_DURATION"},
		},
		&cli.IntFlag{
			Name:    "hls-segment-duration",
			Value:   30,
			Usage:   "Duration of each HLS segment in seconds (max 30)",
			EnvVars: []string{"VR_HLS_SEGMENT_DURATION"},
		},
		&cli.IntFlag{
			Name:    "hls-max-duration",
			Value:   172800,
			Usage:   "Maximum HLS recording duration to keep in seconds (default: 2 days = 172800)",
			EnvVars: []string{"VR_HLS_MAX_DURATION"},
		},
	}
}

func main() {
	app := &cli.App{
		Name:    path.Base(os.Args[0]),
		Usage:   "Connect to a vnc server and record the screen to a video.",
		Version: "0.3.0",
		Action:  recorder,
		Flags:   commonFlags(),
		Commands: []*cli.Command{
			{
				Name:    "daemon",
				Aliases: []string{"d", "watch"},
				Usage:   "Run continuously in background, retry on connection failure with exponential backoff",
				Flags:   commonFlags(),
				Action:  daemonRecorder,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.WithError(err).Fatal("recording failed.")
	}
}

// getOutputPath returns the resolved output path, creating directories as needed
func getOutputPath(c *cli.Context) (string, error) {
	outputPath := c.String("output-path")

	// If no path provided, use current directory
	if outputPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		outputPath = cwd
	}

	// Always create a 'recordings' subdirectory
	recordingsPath := filepath.Join(outputPath, "recordings")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(recordingsPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create recordings directory: %w", err)
	}

	return recordingsPath, nil
}

// daemonRecorder runs the recorder in daemon mode with infinite retries
func daemonRecorder(c *cli.Context) error {
	retryDelay := initialRetryDelay

	logrus.Info("Starting VNC recorder in daemon mode...")

	for {
		outputPath, err := getOutputPath(c)
		if err != nil {
			logrus.WithError(err).Error("Failed to get output path")
			return err
		}
		logrus.WithField("outputPath", outputPath).Info("Starting new HLS recording session")

		// Try to record - this blocks until connection drops or error
		err = doRecord(c, outputPath)

		if err != nil {
			logrus.WithError(err).WithField("retryIn", retryDelay).Warn("Recording failed, will retry...")

			// Sleep before retry
			time.Sleep(retryDelay)

			// Exponential backoff: double the delay, up to max
			retryDelay = retryDelay * 2
			if retryDelay > maxRetryDelay {
				retryDelay = maxRetryDelay
			}
		} else {
			// Recording ended gracefully (e.g., user signal) - reset delay
			logrus.Info("Recording session ended, starting new session...")
			retryDelay = initialRetryDelay
		}
	}
}

func recorder(c *cli.Context) error {
	outputPath, err := getOutputPath(c)
	if err != nil {
		return err
	}
	return doRecord(c, outputPath)
}

// doRecord performs the actual VNC recording with the specified output path
func doRecord(c *cli.Context, outputPath string) error {
	address := fmt.Sprintf("%s:%d", c.String("host"), c.Int("port"))
	dialer, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		logrus.WithError(err).Error("connection to VNC host failed.")
		return err
	}
	defer dialer.Close()

	logrus.WithField("address", address).Info("connection established.")

	// Negotiate connection with the server.
	cchServer := make(chan vnc.ServerMessage)
	cchClient := make(chan vnc.ClientMessage)
	errorCh := make(chan error)

	var secHandlers []vnc.SecurityHandler
	if c.String("password") == "" {
		secHandlers = []vnc.SecurityHandler{
			&vnc.ClientAuthNone{},
		}
	} else {
		secHandlers = []vnc.SecurityHandler{
			&vnc.ClientAuthVNC{Password: []byte(c.String("password"))},
		}
	}

	ccflags := &vnc.ClientConfig{
		SecurityHandlers: secHandlers,
		DrawCursor:       true,
		PixelFormat:      vnc.PixelFormat32bit,
		ClientMessageCh:  cchClient,
		ServerMessageCh:  cchServer,
		Messages:         vnc.DefaultServerMessages,
		Encodings: []vnc.Encoding{
			&vnc.RawEncoding{},
			&vnc.TightEncoding{},
			&vnc.HextileEncoding{},
			&vnc.ZRLEEncoding{},
			&vnc.CopyRectEncoding{},
			&vnc.CursorPseudoEncoding{},
			&vnc.CursorPosPseudoEncoding{},
			&vnc.ZLibEncoding{},
			&vnc.RREEncoding{},
		},
		ErrorCh: errorCh,
	}

	vncConnection, err := vnc.Connect(context.Background(), dialer, ccflags)
	if err != nil {
		logrus.WithError(err).Error("connection negotiation to VNC host failed.")
		return err
	}
	defer vncConnection.Close()
	screenImage := vncConnection.Canvas

	// Find ffmpeg: first check if user provided a custom path, then fallback to global PATH
	ffmpegArg := c.String("ffmpeg")
	var ffmpegPath string

	// Check if it's an absolute path that exists
	if filepath.IsAbs(ffmpegArg) {
		if _, err := os.Stat(ffmpegArg); err == nil {
			ffmpegPath = ffmpegArg
			logrus.WithField("ffmpeg", ffmpegPath).Info("using ffmpeg from configured path")
		}
	}

	// If no valid absolute path, try to find in PATH
	if ffmpegPath == "" {
		var err error
		ffmpegPath, err = exec.LookPath(ffmpegArg)
		if err != nil {
			logrus.WithError(err).WithField("ffmpeg", ffmpegArg).Error("ffmpeg binary not found in PATH or configured location")
			return err
		}
		logrus.WithField("ffmpeg", ffmpegPath).Info("ffmpeg binary found in PATH")
	}

	// Create encoder based on format flag
	format := c.String("format")
	framerate := c.Int("framerate")

	type VideoEncoder interface {
		Encode(img image.Image)
		Close()
	}

	var vcodec VideoEncoder
	var encoderDone chan struct{}

	if format == "hls" {
		// Validate segment duration for HLS
		segmentDuration := c.Int("hls-segment-duration")
		if segmentDuration > 30 {
			logrus.Warn("Segment duration exceeds 30 seconds, capping to 30")
			segmentDuration = 30
		}
		if segmentDuration < 1 {
			segmentDuration = 30
		}

		hlsEncoder := &HLSEncoder{
			FFMpegBinPath:      ffmpegPath,
			Framerate:          framerate,
			ConstantRateFactor: c.Int("crf"),
			SegmentDuration:    segmentDuration,
			MaxDuration:        c.Int("hls-max-duration"),
			OutputPath:         outputPath,
		}
		vcodec = hlsEncoder
		logrus.Info("Using HLS format")

		//goland:noinspection GoUnhandledErrorResult
		go hlsEncoder.Run()
	} else {
		// MP4 format with duration-based rotation
		maxDuration := c.Int("mp4-max-duration")
		logrus.WithField("maxDuration", maxDuration).Info("Using MP4 format with rotation")

		mp4Encoder := &MP4Encoder{
			FFMpegBinPath:      ffmpegPath,
			Framerate:          framerate,
			ConstantRateFactor: c.Int("crf"),
			OutputPath:         outputPath,
		}
		vcodec = mp4Encoder
		encoderDone = make(chan struct{})

		// Run MP4 encoder in a loop with duration-based rotation
		go func() {
			for {
				// Generate new output filename
				timestamp := fmt.Sprintf("%d", time.Now().Unix())
				outputFile := filepath.Join(outputPath, fmt.Sprintf("output-%s.mp4", timestamp))

				logrus.WithFields(logrus.Fields{
					"outputFile":  outputFile,
					"maxDuration": maxDuration,
				}).Info("Starting new MP4 recording")

				// Run the encoder (blocks until closed or error)
				err := mp4Encoder.Run(outputFile)
				if err != nil {
					logrus.WithError(err).Error("MP4 encoder error")
				}

				select {
				case <-encoderDone:
					logrus.Info("MP4 encoder stopped")
					return
				default:
					// Continue to next file
					logrus.Info("Rotating to new MP4 file...")
					// Reset encoder state for next file
					mp4Encoder.closed = false
				}
			}
		}()

		// Duration timer to trigger rotation
		go func() {
			for {
				select {
				case <-encoderDone:
					return
				case <-time.After(time.Duration(maxDuration) * time.Second):
					logrus.WithField("maxDuration", maxDuration).Info("Max duration reached, closing current file...")
					mp4Encoder.Close()
				}
			}
		}()
	}

	for _, enc := range ccflags.Encodings {
		myRenderer, ok := enc.(vnc.Renderer)

		if ok {
			myRenderer.SetTargetImage(screenImage)
		}
	}

	vncConnection.SetEncodings([]vnc.EncodingType{
		vnc.EncCursorPseudo,
		vnc.EncPointerPosPseudo,
		vnc.EncCopyRect,
		vnc.EncTight,
		vnc.EncZRLE,
		vnc.EncHextile,
		vnc.EncZlib,
		vnc.EncRRE,
	})

	// Create a done channel to signal when we should stop
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				timeStart := time.Now()

				vcodec.Encode(screenImage.Image)

				timeTarget := timeStart.Add((1000 / time.Duration(framerate)) * time.Millisecond)
				timeLeft := timeTarget.Sub(time.Now())
				if timeLeft > 0 {
					time.Sleep(timeLeft)
				}
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	frameBufferReq := 0
	timeStart := time.Now()

	for {
		select {
		case err := <-errorCh:
			close(done)
			vcodec.Close()
			time.Sleep(time.Second * 1)
			logrus.WithError(err).Error("VNC connection error")
			return err
		case msg := <-cchClient:
			logrus.WithFields(logrus.Fields{
				"messageType": msg.Type(),
				"message":     msg,
			}).Debug("client message received.")

		case msg := <-cchServer:
			if msg.Type() == vnc.FramebufferUpdateMsgType {
				secsPassed := time.Now().Sub(timeStart).Seconds()
				frameBufferReq++
				reqPerSec := float64(frameBufferReq) / secsPassed
				logrus.WithFields(logrus.Fields{
					"reqs":           frameBufferReq,
					"seconds":        secsPassed,
					"Req Per second": reqPerSec,
				}).Debug("framebuffer update")

				reqMsg := vnc.FramebufferUpdateRequest{Inc: 1, X: 0, Y: 0, Width: vncConnection.Width(), Height: vncConnection.Height()}
				reqMsg.Write(vncConnection)
			}
		case sig := <-sigCh:
			if sig != nil {
				logrus.WithField("signal", sig).Info("signal received.")
				close(done)
				vcodec.Close()
				// give some time to write the file
				time.Sleep(time.Second * 1)
				os.Exit(0)
			}
		}
	}
}
