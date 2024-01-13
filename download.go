package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/mc2soft/mpd"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func getTwitterVideoFiles(tweet *twitterscraper.Tweet, guild *discordgo.Guild) []*discordgo.File {
	files := make([]*discordgo.File, 0)

	for _, video := range tweet.Videos {
		// Create a temp file starting with twitter and ending with .mp4
		f, err := os.CreateTemp("", "twitter*.mp4")
		if err != nil {
			continue
		}

		err = downloadFile(f.Name(), video.URL)
		if err != nil {
			continue
		}

		file, err := os.Open(f.Name())
		if err != nil {
			continue
		}

		// compress video if it is too big
		compressVideo(file, guild)

		file, err = os.Open(f.Name())
		if err != nil {
			return nil
		}

		files = append(files, &discordgo.File{
			Name:   "video.mp4",
			Reader: file,
		})

		defer os.Remove(f.Name())
	}

	for _, gif := range tweet.GIFs {
		// Create a temp file starting with twitter and ending with .mp4
		f, err := os.CreateTemp("", "twitter*.mp4")
		if err != nil {
			continue
		}

		err = downloadFile(f.Name(), gif.URL)
		if err != nil {
			continue
		}

		// convert mp4 to gif
		cmd := exec.Command("ffmpeg",
			"-i", f.Name(),
			"-f", "gif",
			f.Name()+".gif")
		err = cmd.Run()
		if err != nil {
			log.Println(err)
		}

		file, err := os.Open(f.Name() + ".gif")
		if err != nil {
			continue
		}

		files = append(files, &discordgo.File{
			Name:   "video.gif",
			Reader: file,
		})

		defer os.Remove(f.Name())
	}

	return files
}

func getRedditVideoFile(url string, guild *discordgo.Guild, auth redditAuth) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	mpdf, err := os.CreateTemp("", "reddit*.mpd")
	if err != nil {
		return nil
	}
	defer os.Remove(mpdf.Name())
	vf, err := os.CreateTemp("", "reddit*.mp4")
	if err != nil {
		return nil
	}
	defer os.Remove(vf.Name())
	af, err := os.CreateTemp("", "reddit*.mp4")
	if err != nil {
		return nil
	}
	defer os.Remove(af.Name())
	cf, err := os.CreateTemp("", "reddit*.mp4")
	if err != nil {
		return nil
	}
	defer os.Remove(cf.Name())

	filenames := []string{vf.Name(), af.Name(), cf.Name()}

	err = downloadFile(mpdf.Name(), url)
	if err != nil {
		return nil
	}
	mpdfile, err := os.ReadFile(mpdf.Name())
	if err != nil {
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
			log.Println(err)
		}
	} else {
		cmd := exec.Command("ffmpeg", "-i", filenames[0], "-c", "copy", "-y", filenames[2])
		err = cmd.Run()
		if err != nil {
			log.Println(err)
		}
	}

	file, err := os.Open(cf.Name())
	if err != nil {
		return nil
	}

	compressVideo(file, guild)

	file, err = os.Open(cf.Name())
	if err != nil {
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

	cmd := exec.Command("./TwitchDownloaderCLI", "clipdownload", "--id", vodid, "-o", f.Name())
	err = cmd.Run()
	if err != nil {
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		return nil
	}

	// compress video if it is too big
	compressVideo(file, guild)

	file, err = os.Open(f.Name())
	if err != nil {
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
				return
			}

			err = downloadFile(f.Name(), image)
			if err != nil {
				return
			}

			file, err := os.Open(f.Name())
			if err != nil {
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
				return
			}

			err = downloadFile(f.Name(), video)
			if err != nil {
				return
			}

			file, err := os.Open(f.Name())
			if err != nil {
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

func getTikTokFile(url string) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "tiktok*.mp4")
	if err != nil {
		return nil
	}

	err = downloadFile(f.Name(), url)
	if err != nil {
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
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

func getVimeoFile(u string, guild *discordgo.Guild) []*discordgo.File {
	// Create a temp file starting with twitter and ending with .mp4
	f, err := os.CreateTemp("", "vimeo*.mp4")
	if err != nil {
		return nil
	}

	out, err := exec.Command("./yt-dlp", "-o", f.Name(), "--force-overwrites", u).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		return nil
	}

	file, err := os.Open(f.Name())
	if err != nil {
		return nil
	}

	// compress video if it is too big
	compressVideo(file, guild)

	file, err = os.Open(f.Name())
	if err != nil {
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
