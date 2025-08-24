package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var mutex sync.Mutex

type emote struct {
	id              string
	name            string
	urlsByImageType map[string]map[string]string
}

type emoteSet struct {
	id       string
	name     string
	capacity float64
	emotes   []*emote
}

type emoteDownloader struct {
	user                     map[string]any
	emoteSets                []*emoteSet
	currentDownloadIndicator string
	downloadAmount           int
	percentage               float64
	finished                 bool
	errorLog                 []string
	showErrors               bool
}

func (e *emoteSet) String() string {
	return fmt.Sprintf("%s [%d Emotes]", e.name, len(e.emotes))
}

func fetchUser(userId string) (map[string]any, error) {

	userUrl := fmt.Sprintf("https://7tv.io/v3/users/%s", userId)

	userResp, uErr := http.Get(userUrl)
	if uErr != nil {
		return nil, fmt.Errorf("error connecting to api: %v", uErr)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != 200 {
		return nil, fmt.Errorf("user not found. Error Code: %s", userResp.Status)
	}

	body, rErr := io.ReadAll(userResp.Body)
	if rErr != nil {
		return nil, fmt.Errorf("error while reading response: %v", rErr)
	}

	var user map[string]any

	jErr := json.Unmarshal(body, &user)
	if jErr != nil {
		return nil, fmt.Errorf("can't unmarshal response body: %v", jErr)
	}

	return user, nil
}

func downloadAndWriteEmoteSets(model *model, emoteSets []*emoteSet, selectedTypes []string, selectedSizes []string, username string) {
	hasErrors := false
	for _, set := range emoteSets {
		emotePath := fmt.Sprintf("emotes/%s/%s", username, set.name)
		if err := os.MkdirAll(emotePath, os.ModePerm); err != nil {
			*model.stateHeader = fmt.Sprintf("Failed to create directory for emote set: %s\n Error: %s", set.name, err)
		}

		for _, selectedType := range selectedTypes {
			selectedType = strings.ToUpper(selectedType)
			for _, selectedSize := range selectedSizes {
				model.emoteDownloader.percentage = 0

				emoteAmount := getEmoteAmountOfTypeAndSize(set, selectedType, selectedSize)
				percentageIncrease := 1 / float64(emoteAmount)

				*model.stateHeader = fmt.Sprintf("%s: %s %s (%d emotes)", set.name, selectedType, selectedSize, emoteAmount)

				// download emotes
				group := new(errgroup.Group)
				group.SetLimit(runtime.NumCPU())
				for i := range set.emotes {
					group.Go(func() error {
						return downloadEmote(model, emotePath, set.emotes[i], selectedType, selectedSize, percentageIncrease)
					})
				}

				err := group.Wait()
				if err != nil {
					hasErrors = true
				}

				model.emoteDownloader.currentDownloadIndicator = fmt.Sprintf("Successfully downloaded %d emotes from %s.", model.emoteDownloader.downloadAmount, username)
			}
		}
	}

	model.emoteDownloader.finished = true

	if hasErrors {
		model.emoteDownloader.currentDownloadIndicator += "\n\nSome emotes couldn't be downloaded. Press f to see errors."
	}

}

func getEmoteAmountOfTypeAndSize(set *emoteSet, selectedType string, selectedSize string) int {
	amount := 0

	for _, emote := range set.emotes {
		if _, exists := emote.urlsByImageType[selectedType][selectedSize]; exists {
			amount++
		}
	}

	return amount
}

func downloadEmote(model *model, emotePath string, emote *emote, selectedType string, selectedSize string, percentageIncrease float64) error {
	if _, exists := emote.urlsByImageType[selectedType][selectedSize]; exists {
		startTime := time.Now()

		url := emote.urlsByImageType[selectedType][selectedSize]
		// emotes/username/emoteset/GIF4x/
		directoryPath := emotePath + "/" + selectedType + selectedSize

		fileName := emote.name + "." + strings.ToLower(selectedType)
		// emotes/username/emoteset/GIF4x/fileName.gif
		filePath := directoryPath + "/" + fileName

		if _, err := os.Stat(filePath); err == nil {
			mutex.Lock()
			model.emoteDownloader.currentDownloadIndicator = fmt.Sprintf("%s already exists - skipping\n", filePath)
			model.emoteDownloader.percentage += percentageIncrease
			model.downloadChannel <- struct{}{}
			mutex.Unlock()
			return nil
		}
		resp, err := http.Get(url)

		if err != nil {
			e := fmt.Errorf("failed to get emote %s", emote.name)
			model.emoteDownloader.errorLog = append(model.emoteDownloader.errorLog, e.Error())
			return e

		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			e := fmt.Errorf("failed to get emote %s. Status Code: %s", emote.name, resp.Status)
			model.emoteDownloader.errorLog = append(model.emoteDownloader.errorLog, e.Error())
			return e
		}

		// write to file
		if err := os.MkdirAll(directoryPath, os.ModePerm); err != nil {
			e := fmt.Errorf("failed to create directory %s for emote: %s", directoryPath, emote.name)
			model.emoteDownloader.errorLog = append(model.emoteDownloader.errorLog, e.Error())
			return e
		}

		if file, err := os.Create(filePath); err == nil {
			defer file.Close()
			_, err := io.Copy(file, resp.Body)
			if err != nil {
				e := fmt.Errorf("failed to write emote to file %s", fileName)
				model.emoteDownloader.errorLog = append(model.emoteDownloader.errorLog, e.Error())
				return e
			}

			endTime := time.Now()
			took := endTime.Sub(startTime)

			mutex.Lock()
			model.emoteDownloader.percentage += percentageIncrease
			model.emoteDownloader.currentDownloadIndicator = fmt.Sprintf("Downloaded %s - took %.2f seconds", emote.name, float64(took.Milliseconds())/1000)
			model.emoteDownloader.downloadAmount += 1
			model.downloadChannel <- struct{}{}
			mutex.Unlock()
		} else {
			e := fmt.Errorf("failed to create file for %s", fileName)
			model.emoteDownloader.errorLog = append(model.emoteDownloader.errorLog, e.Error())
			return e
		}
	}
	return nil
}

func fetchEmoteSetsOfUser(emoteSets []any) []*emoteSet {
	emoteSetIds := make([]string, len(emoteSets))

	for i, set := range emoteSets {
		set := set.(map[string]any)
		emoteSetIds[i] = set["id"].(string)
	}

	// fetch emote sets in parallel
	setChannels := make([]<-chan *emoteSet, 0)
	for _, id := range emoteSetIds {
		setChannel := make(chan *emoteSet)
		go fetchEmoteSet(id, setChannel)
		setChannels = append(setChannels, setChannel)
	}

	sets := make([]*emoteSet, 0)
	for _, channel := range setChannels {
		if set := <-channel; set != nil {
			set := *set

			sets = append(sets, &set)
		}
	}

	return sets
}

func fetchEmoteSet(id string, setChannel chan *emoteSet) {
	defer close(setChannel)
	apiUrl := fmt.Sprintf("https://7tv.io/v3/emote-sets/%s", id)

	setResp, err := http.Get(apiUrl)
	if err != nil {
		fmt.Println("error while fetching emoteset:", err)
		return
	}
	defer setResp.Body.Close()

	if setResp.StatusCode != 200 {
		fmt.Println("Emote set not found")
		return
	}

	body, err := io.ReadAll(setResp.Body)
	if err != nil {
		fmt.Println("error while reading emoteset response:", err)
		return
	}

	var set map[string]any
	if err := json.Unmarshal(body, &set); err != nil {
		fmt.Println("error while unmarshalling body:", err)
		return
	}

	respEmotes := set["emotes"]
	if respEmotes == nil {
		return
	}

	// fetch emotes in parallel
	emoteChannels := make([]<-chan *emote, 0)
	for _, respEmote := range respEmotes.([]any) {
		respEmote := respEmote.(map[string]any)
		data := respEmote["data"].(map[string]any)

		emoteChannel := make(chan *emote)
		go createEmoteFromData(data, emoteChannel)
		emoteChannels = append(emoteChannels, emoteChannel)
	}

	emotes := make([]*emote, 0)
	for _, channel := range emoteChannels {
		if emote := <-channel; emote != nil {
			emotes = append(emotes, emote)
		}
	}

	newEmoteSet := emoteSet{
		id:       set["id"].(string),
		name:     strings.Trim(set["name"].(string), " "),
		capacity: set["capacity"].(float64),
		emotes:   emotes,
	}

	setChannel <- &newEmoteSet
}

func createEmoteFromData(emoteData map[string]any, emoteChannel chan<- *emote) {
	defer close(emoteChannel)
	emote := emote{
		id:   emoteData["id"].(string),
		name: emoteData["name"].(string),
	}
	host := emoteData["host"].(map[string]any)
	baseUrl := "https:" + host["url"].(string)

	urlsByImageType := make(map[string]map[string]string)
	files := host["files"].([]any)

	for _, file := range files {
		file := file.(map[string]any)

		format := file["format"].(string)
		if _, exists := urlsByImageType[format]; !exists {
			urlsByImageType[format] = make(map[string]string, 0)
		}

		name := file["name"].(string)
		fileUrl := baseUrl + "/" + name
		size := strings.Split(name, ".")[0]
		urlsByImageType[format][size] = fileUrl
	}

	emote.urlsByImageType = urlsByImageType

	emoteChannel <- &emote
}
