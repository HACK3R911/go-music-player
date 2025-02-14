package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

var (
	mu       sync.Mutex
	streamer beep.StreamSeekCloser
	ctrl     *beep.Ctrl
	done     = make(chan bool)
)

func main() {
	filePath := flag.String("file", "", "Path to the audio file")
	flag.Parse()

	if *filePath == "" {
		fmt.Println("Please specify an audio file with --file")
		return
	}

	f, err := os.Open(*filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer.Close()

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		log.Fatal(err)
	}

	ctrl = &beep.Ctrl{Streamer: beep.Loop(-1, streamer)}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	speaker.Play(ctrl)

	fmt.Println("Playing... (press: p - pause, s - stop, r - resume)")

	go handleInput()

	select {
	case <-sig:
		fmt.Println("\nInterrupted")
	case <-done:
		fmt.Println("\nPlayback finished")
	}
}

func handleInput() {
	for {
		var cmd string
		fmt.Scanln(&cmd)

		mu.Lock()
		switch cmd {
		case "p":
			speaker.Lock()
			ctrl.Paused = true
			speaker.Unlock()
			fmt.Println("Paused")
		case "r":
			speaker.Lock()
			ctrl.Paused = false
			speaker.Unlock()
			fmt.Println("Resuming...")
		case "s":
			speaker.Lock()
			ctrl.Streamer = nil
			speaker.Unlock()
			fmt.Println("Stopped")
			done <- true
			return
		}
		mu.Unlock()
	}
}
