package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var opts struct {
	osuSongPath string
	outDir      string
}

func init() {
	flag.StringVar(&opts.osuSongPath, "p", "", "path to osu songs directory")
	flag.StringVar(&opts.outDir, "d", "Songs", "the songs output directory")
	flag.Parse()

	if opts.osuSongPath == "" {
		println("missing required -d argument\n")
		os.Exit(2)
	}
}

func main() {
	songs, err := os.ReadDir(opts.osuSongPath)
	if err != nil {
		panic(err)
	}

	isTTY := isTTY()
	end := len(songs)
	skipped := 0

	// ensure outdir
	if _, err := os.Stat(opts.outDir); os.IsNotExist(err) {
		err = os.Mkdir(opts.outDir, 0755)
		if err != nil {
			panic(err)
		}
	}

	progressStrLen := 0
	pad := 0

	for i, song := range songs {
		if song.IsDir() {
			func() {
				defer func() {
					if r := recover(); r != nil {
						skipped++
					}
				}()

				filePath, err := findBeatmapFile(song.Name())
				if err != nil {
					panic(err)
				}

				file, err := os.Open(filePath)
				if err != nil {
					panic(err)
				}
				defer file.Close()

				var songMetadata struct {
					AudioFileName string
					Title         string
					Artist        string
				}

				valueExp := regexp.MustCompile("^[a-zA-Z]+:(.*)")

				fullfilled := func() bool {
					return (songMetadata.AudioFileName != "" &&
						songMetadata.Artist != "" &&
						songMetadata.Title != "")
				}

				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					if fullfilled() {
						break
					}

					line := scanner.Text()
					if strings.HasPrefix(line, "AudioFilename:") {
						songMetadata.AudioFileName = strings.TrimSpace(valueExp.FindStringSubmatch(line)[1])
					} else if strings.HasPrefix(line, "Title:") {
						songMetadata.Title = strings.TrimSpace(valueExp.FindStringSubmatch(line)[1])
					} else if strings.HasPrefix(line, "Artist:") {
						songMetadata.Artist = strings.TrimSpace(valueExp.FindStringSubmatch(line)[1])
					}
				}
				if err := scanner.Err(); err != nil {
					panic(err)
				}

				audioFilePath := filepath.Join(filepath.Dir(filePath), songMetadata.AudioFileName)
				buf, err := os.ReadFile(audioFilePath)
				if err != nil {
					panic(err)
				}

				songFileName := fmt.Sprintf(
					"%s - %s%s",
					songMetadata.Artist,
					songMetadata.Title,
					filepath.Ext(audioFilePath),
				)
				invalidChars := regexp.MustCompile(`[/\\?%*:|"<>]`)
				songFileName = invalidChars.ReplaceAllString(songFileName, "_")
				err = os.WriteFile(filepath.Join(opts.outDir, songFileName), buf, 0755)
				if err != nil {
					panic(err)
				}

				if isTTY {
					songTitle := TruncateString(songMetadata.Title, 30)
					if len(songTitle) == 30 {
						songTitle += "..."
					}

					if len(songTitle) > pad {
						pad = len(songTitle)
					}

					str := fmt.Sprintf("%d/%d %-*s\r", i+1, end, pad, songTitle)
					progressStrLen = len(str)
					fmt.Print(str)
				}
			}()
		}
	}

	fmt.Printf("%-*s\r", progressStrLen, "") // clear cr
	fmt.Printf("\rDone!")
	fmt.Printf("\nExported: %d\nSkipped: %d\n", end-skipped, skipped)
}

func findBeatmapFile(dir string) (filePath string, err error) {
	found := errors.New("found") // stop walking dir

	dir = filepath.Join(opts.osuSongPath, dir)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && filepath.Ext(path) == ".osu" {
			filePath = path
			return found
		}

		return nil
	})

	if err == found {
		err = nil
	}

	return
}

func TruncateString(str string, length int) string {
	if length <= 0 {
		return ""
	}

	if utf8.RuneCountInString(str) < length {
		return str
	}

	return string([]rune(str)[:length])
}

func isTTY() bool {
	i, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return i.Mode()&os.ModeCharDevice != 0
}
