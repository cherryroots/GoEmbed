package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/mc2soft/mpd"
)

func downloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		log.Println(err)
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func getRedditVideoFile(url string, guild *discordgo.Guild) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	mpdf, err := os.CreateTemp("", "reddit*.mpd")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer os.Remove(mpdf.Name())
	vf, err := os.CreateTemp("", "reddit*.mp4")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer os.Remove(vf.Name())
	af, err := os.CreateTemp("", "reddit*.mp4")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer os.Remove(af.Name())
	cf, err := os.CreateTemp("", "reddit*.mp4")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer os.Remove(cf.Name())

	filenames := []string{vf.Name(), af.Name(), cf.Name()}

	err = downloadFile(mpdf.Name(), url)
	if err != nil {
		log.Println(err)
		return nil
	}
	mpdfile, err := os.ReadFile(mpdf.Name())
	if err != nil {
		log.Println(err)
		return nil
	}

	mp := new(mpd.MPD)
	mp.Decode(mpdfile)
	period := mp.Period[0]
	// get the last representation in each AdaptationSet
	for _, as := range period.AdaptationSets {
		as.Representations = as.Representations[len(as.Representations)-1:]
	}

	// remove the part after the last / in the url
	re := regexp.MustCompile(`(.*/)[^/]*$`)
	url = re.ReplaceAllString(url, "$1")

	sets := len(period.AdaptationSets)
	if sets > 2 {
		sets = 2
	}

	var wg sync.WaitGroup
	wg.Add(sets)
	for set := 0; set < sets; set++ {
		go func(set int) {
			defer wg.Done()
			err = downloadFile(filenames[set], url+*period.AdaptationSets[set].Representations[0].BaseURL)
			if err != nil {
				log.Println(err)
			}
		}(set)
	}
	wg.Wait()

	if sets == 2 {
		cmd := exec.Command("ffmpeg", "-i", filenames[0], "-i", filenames[1], "-c", "copy", "-y", filenames[2])
		err = cmd.Run()
		if err != nil {
			log.Printf("Error merging video and audio: %v", err)
		}
	} else {
		cmd := exec.Command("ffmpeg", "-i", filenames[0], "-c", "copy", "-y", filenames[2])
		err = cmd.Run()
		if err != nil {
			log.Printf("Error merging video and audio: %v", err)
		}
	}

	file, err := os.Open(cf.Name())
	if err != nil {
		log.Println(err)
		return nil
	}

	compressVideo(file, guild, false)

	file, err = os.Open(cf.Name())
	if err != nil {
		log.Println(err)
		return nil
	}

	videoFile := []*discordgo.File{
		{
			Name:   "video.mp4",
			Reader: file,
		},
	}

	return videoFile
}

func getTwitchClipFile(vodid string, guild *discordgo.Guild) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "twitch*.mp4")
	if err != nil {
		return nil
	}

	err = os.Setenv("DOTNET_BUNDLE_EXTRACT_BASE_DIR", filepath.Join("./TwitchDownloaderCLI", ".net"))
	if err != nil {
		log.Println(err)
	}

	downloadType := "clipdownload"

	out, err := exec.Command("./TwitchDownloaderCLI/TwitchDownloaderCLI", downloadType, "--collision", "overwrite", "--id", vodid, "-o", f.Name()).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		log.Printf("Failed to download Twitch Clip id: %s: %s", vodid, err)
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		log.Println(err)
		return nil
	}

	// compress video if it is too big
	compressVideo(file, guild, false)

	file, err = os.Open(f.Name())
	if err != nil {
		log.Println(err)
		return nil
	}

	videoFile := []*discordgo.File{
		{
			Name:   "video.mp4",
			Reader: file,
		},
	}

	defer os.Remove(f.Name())

	return videoFile
}

func getInstagramFiles(images []string, videos []string) ([]*discordgo.File, []*discordgo.File) {
	imageFiles := make([]*discordgo.File, 0)
	videoFiles := make([]*discordgo.File, 0)

	var wg sync.WaitGroup
	wg.Add(len(images) + len(videos))

	for _, image := range images {
		go func(image string) {
			defer wg.Done()
			// Create a temp file starting with twitter and ending with .mp4
			f, err := os.CreateTemp("", "instagram*.jpg")
			if err != nil {
				log.Println(err)
				return
			}

			err = downloadFile(f.Name(), image)
			if err != nil {
				log.Println(err)
				return
			}

			file, err := os.Open(f.Name())
			if err != nil {
				log.Println(err)
				return
			}

			imageFiles = append(imageFiles, &discordgo.File{
				Name:   "image.jpg",
				Reader: file,
			})

			defer os.Remove(f.Name())
		}(image)
	}

	for _, video := range videos {
		go func(video string) {
			defer wg.Done()
			// Create a temp file starting with twitter and ending with .mp4
			f, err := os.CreateTemp("", "instagram*.mp4")
			if err != nil {
				log.Println(err)
				return
			}

			err = downloadFile(f.Name(), video)
			if err != nil {
				log.Println(err)
				return
			}

			file, err := os.Open(f.Name())
			if err != nil {
				log.Println(err)
				return
			}

			videoFiles = append(videoFiles, &discordgo.File{
				Name:   "video.mp4",
				Reader: file,
			})

			defer os.Remove(f.Name())
		}(video)
	}

	wg.Wait()
	return imageFiles, videoFiles
}

func getVimeoFile(u string, guild *discordgo.Guild) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "vimeo*.mp4")
	if err != nil {
		log.Println(err)
		return nil
	}

	out, err := exec.Command("./yt-dlp", "-o", f.Name(), "--force-overwrites", u).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		log.Printf("Failed to download video: %s", err)
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		log.Println(err)
		return nil
	}

	// compress video if it is too big
	compressVideo(file, guild, false)

	file, err = os.Open(f.Name())
	if err != nil {
		log.Println(err)
		return nil
	}

	videoFile := []*discordgo.File{
		{
			Name:   "video.mp4",
			Reader: file,
		},
	}

	defer os.Remove(f.Name())

	return videoFile
}
