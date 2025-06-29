package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"

	"github.com/ebitengine/oto/v3"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type ffprobeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

type playerControl struct {
	player *oto.Player
	writer *io.PipeWriter
	cancel context.CancelFunc
}

var (
	playerMutex  sync.Mutex
	activePlayer *playerControl
)

func stopPlayback() {
	playerMutex.Lock()
	defer playerMutex.Unlock()

	if activePlayer != nil {
		log.Println("Stopping previous playback...")
		if activePlayer.writer != nil {
			activePlayer.writer.Close()
		}
		if activePlayer.player != nil {
			activePlayer.player.Close()
		}
		if activePlayer.cancel != nil {
			activePlayer.cancel()
		}
		activePlayer = nil
		log.Println("Previous playback stopped.")
	}
}

func getTrackDuration(filename string) (time.Duration, error) {
	data, err := ffmpeg.Probe(filename)
	if err != nil {
		fmt.Println("Could not fetch track duration", err)
		return 0, err
	}

	var probeOutput ffprobeOutput
	err = json.Unmarshal([]byte(data), &probeOutput)
	if err != nil {
		fmt.Println("Could not unmarshal track duration", err)
		return 0, err
	}

	durationFloat, err := strconv.ParseFloat(probeOutput.Format.Duration, 64)
	if err != nil {
		fmt.Println("Could not parse duration", err)
		return 0, err
	}

	duration := time.Duration(durationFloat) * time.Second
	return duration, nil
}

func startPlaybackGoroutine(
	songToPlay string,
	otoCtx *oto.Context,
	playbackValueBinding binding.Float,
	playbackSlider *widget.Slider,
	durationLabel *widget.Label,
	currentDurationBinding binding.Float) {

	log.Printf("Playback goroutine started for song: '%s'", songToPlay)

	log.Println("Fetching duration of the song")
	duration, err := getTrackDuration(songToPlay)
	if err != nil {
		log.Println("ERROR: Could not get track duration", err)
	}
	log.Println("Duration is", duration.Seconds())

	reader, writer := io.Pipe()
	player := otoCtx.NewPlayer(reader)

	_, cancel := context.WithCancel(context.Background())

	playerMutex.Lock()
	activePlayer = &playerControl{
		player: player,
		writer: writer,
		cancel: cancel,
	}
	playerMutex.Unlock()

	defer func() {
		log.Printf("Cleaning up for: %s", songToPlay)
		player.Close()
		writer.Close()
		reader.Close()
	}()

	go func() {
		stderr := &bytes.Buffer{}
		err := ffmpeg.Input(songToPlay).
			Output("pipe:1", ffmpeg.KwArgs{
				"acodec": "pcm_s16le",
				"f":      "s16le",
				"ac":     "2",
				"ar":     "44100",
			}).
			WithOutput(writer).
			WithErrorOutput(stderr).
			OverWriteOutput().
			Run()

		if err != nil {
			log.Printf("FFmpeg process error: %v\nFFmpeg stderr: %s", err, stderr.String())
		}
		writer.Close()
	}()

	log.Println("Calling player.Play()...")
	playbackSlider.Max = duration.Seconds()
	secondsModulo := int(duration.Seconds()) % 60
	fyne.Do(func() {
		durationLabel.SetText(fmt.Sprintf("%02d:%02d", int(duration.Minutes()), secondsModulo))
	})
	player.Play()

	go func(p *oto.Player, slider *widget.Slider, totalDuration time.Duration) {
		startTime := time.Now()

		for p.IsPlaying() {
			elapsed := time.Since(startTime)

			if elapsed > totalDuration {
				elapsed = totalDuration
			}
			playbackValueBinding.Set(elapsed.Seconds())
			currentDurationBinding.Set(elapsed.Seconds())
			fyne.Do(func() {
				playbackSlider.Refresh()
			})
			time.Sleep(100 * time.Millisecond)
		}

		log.Printf("UI updater finished for: %s", songToPlay)

	}(player, playbackSlider, duration)

	for player.IsPlaying() {
		time.Sleep(time.Millisecond)
	}

	log.Printf("Playback finished for: %s", songToPlay)
}
