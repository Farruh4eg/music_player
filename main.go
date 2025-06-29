package main

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/ebitengine/oto/v3"
)

var (
	selectedSong string
)

func main() {
	log.Println("Application starting...")
	a := app.New()
	w := a.NewWindow("Music Player")
	w.Resize(fyne.NewSize(1280, 720))
	playbackValueBinding := binding.NewFloat()

	musicFilesRegex, err := regexp.Compile(`\.(?i)(mp3|wav|flac|aac|ogg|wma|m4a|alac|aiff|pcm)$`)
	if err != nil {
		fmt.Println("Could not compile music files regex", err)
		return
	}

	musicFilesPaths := make([]string, 0, 128)
	e := filepath.WalkDir("E:/Download/Music", func(path string, d fs.DirEntry, err error) error {
		if err == nil && musicFilesRegex.MatchString(path) {
			musicFilesPaths = append(musicFilesPaths, path)
		}
		return nil
	})

	if e != nil {
		fmt.Println("Could not walk through music directory", e)
		return
	}

	log.Printf("Found %d music files.", len(musicFilesPaths))

	musicList := widget.NewList(
		func() int { return len(musicFilesPaths) },
		func() fyne.CanvasObject { return widget.NewLabel("Song title here") },
		func(id widget.ListItemID, object fyne.CanvasObject) {
			_, file := filepath.Split(musicFilesPaths[id])
			object.(*widget.Label).SetText(file)
		},
	)

	musicList.OnSelected = func(id widget.ListItemID) {
		selectedSong = musicFilesPaths[id]
		log.Printf("Song selected in UI: ID=%d, Path=%s", id, selectedSong)
	}

	playbackSlider := widget.NewSliderWithData(0.0, 1.0, playbackValueBinding)

	otoCtx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		fmt.Println("Could not create context", err)
		return
	}
	<-ready
	log.Println("Oto context is ready.")

	durationLabel := widget.NewLabel("00:00")

	currentDurationFloat := binding.NewFloat()    // this is used to pass the current duration value from our playback gorouting
	currentDurationBinding := binding.NewString() // and this is used to actually convert that float value to MM:SS string for use in our label

	currentDurationFloat.AddListener(binding.NewDataListener(func() {
		newValue, err := currentDurationFloat.Get()
		if err != nil {
			log.Println("Could not get current duration", err)
			return
		}

		minutes := int(newValue / 60)
		seconds := int(newValue) % 60

		currentDurationBinding.Set(fmt.Sprintf("%02d:%02d", minutes, seconds))
	}))

	currentDurationLabel := widget.NewLabelWithData(currentDurationBinding)

	playButton := widget.NewButton("Play", func() {
		log.Println("--- Play button clicked ---")

		if selectedSong == "" {
			log.Println("ERROR: No song selected.")
			return
		}

		playbackValueBinding.Set(0.0)
		stopPlayback()
		time.Sleep(100 * time.Millisecond)

		go startPlaybackGoroutine(selectedSong, otoCtx, playbackValueBinding, playbackSlider, durationLabel, currentDurationFloat)
	})

	// Контейнер для кнопки и списка песен.
	// Кнопка помещается сверху (top), а список в центр (center),
	// чтобы он занял все оставшееся место.
	sliderWithDurationContent := container.NewBorder(nil, nil, currentDurationLabel, durationLabel, playbackSlider)
	bottomContent := container.NewBorder(playButton, nil, nil, nil, musicList)

	// Финальная компоновка всего окна.
	// Контейнер со слайдером идет наверх (top).
	// Контейнер с кнопкой и списком идет в центр (center).
	content := container.NewBorder(sliderWithDurationContent, nil, nil, nil, bottomContent)

	w.SetContent(content)
	w.ShowAndRun()
}
