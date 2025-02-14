package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

var (
	mu           sync.Mutex
	streamer     beep.StreamSeekCloser
	ctrl         *beep.Ctrl
	done         = make(chan bool)
	playlist     []string
	currentTrack int
	format       beep.Format
)

func main() {
	folderPath := flag.String("folder", "", "Path to the folder containing audio files")
	flag.Parse()

	if *folderPath == "" {
		fmt.Println("Please specify a folder with --folder")
		return
	}

	// Сканируем папку и находим все MP3-файлы
	err := filepath.Walk(*folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".mp3" {
			playlist = append(playlist, path)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	if len(playlist) == 0 {
		fmt.Println("No MP3 files found in the specified folder")
		return
	}

	currentTrack = 0

	if err := playCurrentTrack(); err != nil {
		log.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Controls: p - pause/resume, s - stop, n - next, b - previous, q - quit")
	fmt.Printf("Now playing %d/%d: %s\n", currentTrack+1, len(playlist), playlist[currentTrack])

	go handleInput()

	select {
	case <-sig:
		fmt.Println("\nInterrupted")
	case <-done:
		fmt.Println("\nExiting...")
	}

	speaker.Close()
	if streamer != nil {
		streamer.Close()
	}
}

func playCurrentTrack() error {
	mu.Lock()
	defer mu.Unlock()

	speaker.Clear()
	if streamer != nil {
		streamer.Close()
	}

	f, err := os.Open(playlist[currentTrack])
	if err != nil {
		return err
	}

	fmt.Printf("Loading track: %s\n", playlist[currentTrack])

	streamer, format, err = mp3.Decode(f)
	if err != nil {
		f.Close()
		return err
	}

	// Инициализация speaker
	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/30)); err != nil {
		streamer.Close()
		return err
	}

	ctrl = &beep.Ctrl{
		Streamer: beep.Seq(streamer, beep.Callback(func() {
			nextTrack()
		})),
		Paused: false,
	}

	fmt.Println("Starting playback...")
	speaker.Play(ctrl)
	return nil
}

func nextTrack() {
	mu.Lock()
	if currentTrack < len(playlist)-1 {
		currentTrack++
	} else {
		currentTrack = 0
	}
	mu.Unlock()

	if err := playCurrentTrack(); err != nil {
		log.Printf("Error loading next track: %v", err)
		return
	}
	fmt.Printf("Now playing %d/%d: %s\n", currentTrack+1, len(playlist), playlist[currentTrack])
}

func previousTrack() {
	mu.Lock()
	if currentTrack > 0 {
		currentTrack--
	} else {
		currentTrack = len(playlist) - 1
	}
	mu.Unlock()

	if err := playCurrentTrack(); err != nil {
		log.Printf("Error loading previous track: %v", err)
		return
	}
	fmt.Printf("Now playing %d/%d: %s\n", currentTrack+1, len(playlist), playlist[currentTrack])
}

func handleInput() {
	for {
		var cmd string
		fmt.Scanln(&cmd)

		switch cmd {
		case "p":
			mu.Lock()
			speaker.Lock()
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()
			mu.Unlock()
			if ctrl.Paused {
				fmt.Println("Paused")
			} else {
				fmt.Println("Resuming...")
			}

		case "s":
			mu.Lock()
			speaker.Lock()
			speaker.Clear()
			if streamer != nil {
				streamer.Seek(0)
			}
			ctrl.Paused = false
			speaker.Unlock()
			mu.Unlock()
			fmt.Println("Stopped")

		case "n":
			nextTrack()

		case "b":
			previousTrack()

		case "q":
			done <- true
			return

		default:
			fmt.Println("Unknown command")
		}
	}
}
